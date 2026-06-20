package runner

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"pgtest/assert"
	"pgtest/config"
	"pgtest/dotsql"

	"github.com/lib/pq"
)

// Runner executes test cases against a database.
type Runner struct {
	db          *sql.DB
	dotsql      *dotsql.DotSql
	cfg         *config.Config
	reporter    *Reporter
	globals     map[string]interface{}
	verbose     bool
	notice      bool
	stopOnError bool
	stepOutputs map[string]map[string]interface{} // step name -> column -> value
}

// New creates a new Runner.
func New(cfg *config.Config, dotsql *dotsql.DotSql, reporter *Reporter, verbose bool, notice bool, stopOnError bool) (*Runner, error) {
	driver, dsn := cfg.GetDSNWithDriver()

	// Print DSN info in verbose mode (mask password for security)
	if verbose {
		fmt.Printf("[verbose] database driver: %s\n", driver)
		fmt.Printf("[verbose] database dsn: %s\n", maskDSN(dsn))
		fmt.Printf("[verbose] connecting to database...\n")
	}

	var db *sql.DB
	var err error

	if notice {
		// Use pq connector directly so we can register a notice handler
		rawConn, err2 := pq.NewConnector(dsn)
		if err2 != nil {
			if verbose {
				fmt.Printf("[verbose] create connector failed: %v\n", err2)
			}
			return nil, fmt.Errorf("runner.New: create connector: %w", err2)
		}

		// Wrap with notice handler
		noticeConn := pq.ConnectorWithNoticeHandler(rawConn, func(notice *pq.Error) {
			fmt.Fprintf(os.Stderr, "[notice] %s\n", notice.Message)
		})

		db = sql.OpenDB(noticeConn)
	} else {
		db, err = sql.Open(driver, dsn)
		if err != nil {
			if verbose {
				fmt.Printf("[verbose] database connection failed: %v\n", err)
			}
			return nil, fmt.Errorf("runner.New: open db: %w", err)
		}
	}

	if err := db.Ping(); err != nil {
		db.Close()
		if verbose {
			fmt.Printf("[verbose] database ping failed: %v\n", err)
		}
		return nil, fmt.Errorf("runner.New: ping db: %w", err)
	}

	if verbose {
		fmt.Printf("[verbose] database connection established\n")
	}

	// Copy globals
	g := make(map[string]interface{})
	for k, v := range cfg.Globals {
		g[k] = v
	}

	return &Runner{
		db:          db,
		dotsql:      dotsql,
		cfg:         cfg,
		reporter:    reporter,
		globals:     g,
		verbose:     verbose,
		notice:      notice,
		stopOnError: stopOnError,
		stepOutputs: make(map[string]map[string]interface{}),
	}, nil
}

// Close closes the database connection.
func (r *Runner) Close() error {
	return r.db.Close()
}

// Run executes all test cases.
func (r *Runner) Run() {
	// Run root-level setup/teardown
	r.runSQLBatch(r.cfg.Setup, "global-setup")
	defer r.runSQLBatch(r.cfg.Teardown, "global-teardown")

	for _, tc := range r.cfg.Cases {
		shouldStop := r.runCase(tc)
		if r.stopOnError && shouldStop {
			break
		}
	}
	r.reporter.Flush()
}

// runCase runs a single test case (and its before_all/after_all hooks).
// Returns true if the case should trigger a stop (a step failed).
func (r *Runner) runCase(tc config.CaseConfig) bool {
	if tc.Skip {
		if r.verbose {
			fmt.Printf("[verbose] skipping case: %s\n", tc.Name)
		}
		r.reporter.Add(ReportLine{
			Status: StatusSkipped,
			Name:   tc.Name,
		})
		return false
	}

	if r.verbose {
		fmt.Printf("[verbose] running case: %s\n", tc.Name)
	}

	// Merge case-level vars
	vars := make(map[string]interface{})
	for k, v := range r.globals {
		vars[k] = v
	}
	for k, v := range tc.Vars {
		vars[k] = v
	}

	r.runSQLBatch(tc.BeforeAll, tc.Name+"/before_all")
	defer r.runSQLBatch(tc.AfterAll, tc.Name+"/after_all")

	for _, step := range tc.Steps {
		failed := r.runStep(tc.Name, step, vars)
		if r.stopOnError && failed {
			return true
		}
	}
	return false
}

// runStep executes a single test step.
// Returns true if the step failed.
func (r *Runner) runStep(caseName string, step config.StepConfig, vars map[string]interface{}) bool {
	stepName := caseName + "/" + step.Name
	if step.Name == "" {
		stepName = caseName
	}

	if step.Skip {
		r.reporter.Add(ReportLine{
			Status: StatusSkipped,
			Name:   stepName,
		})
		return false
	}

	// If the named query is disabled (all content commented out), skip silently.
	// Use step.Name as lookup key when step.Query is empty.
	dotsqlKey := step.Query
	if dotsqlKey == "" {
		dotsqlKey = step.Name
	}
	if r.dotsql.IsDisabled(dotsqlKey) {
		r.reporter.Add(ReportLine{
			Status: StatusSkipped,
			Name:   stepName,
		})
		return false
	}

	start := time.Now()

	// Determine retry count
	maxRetries := step.Retry
	if maxRetries <= 0 {
		maxRetries = 1 // at least one attempt
	}

	var lastErr error
	var lastResult *assert.Result
	for attempt := 0; attempt < maxRetries; attempt++ {
		if attempt > 0 {
			time.Sleep(200 * time.Millisecond)
		}

		lastResult, lastErr = r.executeStep(step, vars)
		if lastErr == nil {
			// Store step output so subsequent steps can reference it
			r.storeStepOutput(step, lastResult)
			r.reporter.Add(ReportLine{
				Status:   StatusPassed,
				Name:     stepName,
				Duration: time.Since(start),
				Retries:  attempt,
			})
			return false
		}
	}

	r.reporter.Add(ReportLine{
		Status:   StatusFailed,
		Name:     stepName,
		Duration: time.Since(start),
		Error:    lastErr.Error(),
		Retries:  maxRetries - 1,
	})
	return true
}

// executeStep runs a single step attempt, returning the raw result on success.
func (r *Runner) executeStep(step config.StepConfig, vars map[string]interface{}) (*assert.Result, error) {
	// Resolve the query string with variables
	// When step.Query is empty, fall back to step.Name as the dotsql lookup key.
	query := step.Query
	dotsqlKey := step.Query
	if dotsqlKey == "" {
		dotsqlKey = step.Name
		if dotsqlKey == "" {
			return nil, fmt.Errorf("step has no query and no name")
		}
	}

	// Lookup named queries from dotsql
	if namedQuery, err := r.dotsql.Raw(dotsqlKey); err == nil {
		query = namedQuery
	} else if step.Query == "" {
		// If query field was omitted, dotsql lookup MUST succeed (step.Name is not raw SQL).
		return nil, fmt.Errorf("named query %q not found (and no inline query provided)", dotsqlKey)
	}
	// otherwise, treat query as raw SQL (when step.Query is a literal SQL string)

	// Variable substitution in query: {{key}} and {{step.col}}
	query = r.substituteVars(query, vars)

	// Build args, with variable substitution
	args := make([]interface{}, len(step.Args))
	for i, arg := range step.Args {
		args[i] = r.resolveVar(arg, vars)
	}

	// Trim args to match the number of $N placeholders in the final query.
	// When {{step.col}} is inlined in SQL, the corresponding arg is no longer needed.
	args = r.trimArgsToPlaceholders(query, args)

	// Verbose: print the SQL and args being executed
	if r.verbose {
		r.printVerboseSQL(query, args)
	}

	// Run SQL
	rows, err := r.db.Query(query, args...)
	if err != nil {
		// Check if the error message is a JSON object from a custom RAISE EXCEPTION
		if result, rawJSON := tryParseJSONError(err.Error()); result != nil {
			if r.verbose {
				fmt.Fprintf(os.Stderr, "[verbose] RAISE EXCEPTION (raw JSON): %s\n", rawJSON)
				r.printVerboseResult(result)
			}
			return nil, runResultAssertions(result, step)
		}
		return nil, fmt.Errorf("query error: %w", err)
	}

	result, err := assert.CollectRows(rows)
	if err != nil {
		return nil, fmt.Errorf("collect rows: %w", err)
	}

	// Verbose: print raw query result (before JSON expansion)
	if r.verbose {
		r.printVerboseResult(result)
	}

	// Check if the result is a single JSON string value (stored procedure return)
	if parsed := tryParseJSONResult(result); parsed != nil {
		result = parsed
	}

	return result, runResultAssertions(result, step)
}

// storeStepOutput saves the first row of a step result keyed by step name.
// Subsequent steps can reference values via $stepname.col or {{stepname.col}}.
func (r *Runner) storeStepOutput(step config.StepConfig, result *assert.Result) {
	if result == nil || result.Count == 0 || step.Name == "" {
		return
	}

	cols := make(map[string]interface{})
	for i, col := range result.Columns {
		if i < len(result.Rows[0]) {
			cols[col] = result.Rows[0][i]
		}
	}
	r.stepOutputs[step.Name] = cols
}

// substituteVars replaces {{key}} and {{step.col}} patterns in a string.
// Variable resolution order: step vars > globals > step outputs.
// PostgreSQL $1, $2 parameter placeholders are not expanded.
func (r *Runner) substituteVars(s string, vars map[string]interface{}) string {
	result := s

	// Replace {{key}} from vars and globals
	for k, v := range vars {
		result = strings.ReplaceAll(result, "{{"+k+"}}", fmt.Sprintf("%v", v))
	}
	for k, v := range r.globals {
		if _, ok := vars[k]; !ok { // vars take precedence
			result = strings.ReplaceAll(result, "{{"+k+"}}", fmt.Sprintf("%v", v))
		}
	}

	// Replace {{step.col}} from step outputs
	for stepName, cols := range r.stepOutputs {
		for col, val := range cols {
			placeholder := "{{" + stepName + "." + col + "}}"
			result = strings.ReplaceAll(result, placeholder, fmt.Sprintf("%v", val))
		}
	}

	return result
}

// resolveVar resolves a value that may be a variable reference like "$var_name" or "$step.col".
func (r *Runner) resolveVar(val interface{}, vars map[string]interface{}) interface{} {
	s, ok := val.(string)
	if !ok {
		return val
	}

	// $stepname.col — step output reference (check before $key to avoid partial matches)
	if strings.HasPrefix(s, "$") {
		key := s[1:]
		// Try step output first (dot-separated)
		if v, found := r.resolveStepVar(key); found {
			return v
		}
		// Then vars
		if v, ok := vars[key]; ok {
			return v
		}
		// Then globals
		if v, ok := r.globals[key]; ok {
			return v
		}
		// fallback to env
		if envVal := os.Getenv(key); envVal != "" {
			return envVal
		}
		return s // keep raw if not found
	}

	// {{key}} or {{step.col}} in args
	if strings.HasPrefix(s, "{{") && strings.HasSuffix(s, "}}") {
		key := s[2 : len(s)-2]
		// Try step output first
		if v, found := r.resolveStepVar(key); found {
			return v
		}
		// Then vars
		if v, ok := vars[key]; ok {
			return v
		}
		// Then globals
		if v, ok := r.globals[key]; ok {
			return v
		}
	}

	// Bare string — may still be a step output reference like "stepname.col"
	if v, found := r.resolveStepVar(s); found {
		return v
	}
	return s
}

// resolveStepVar resolves a "stepname.column" reference from stored step outputs.
func (r *Runner) resolveStepVar(ref string) (interface{}, bool) {
	dot := strings.Index(ref, ".")
	if dot <= 0 || dot >= len(ref)-1 {
		return nil, false
	}
	stepName := ref[:dot]
	colName := ref[dot+1:]

	stepOut, ok := r.stepOutputs[stepName]
	if !ok {
		return nil, false
	}
	val, ok := stepOut[colName]
	return val, ok
}

// trimArgsToPlaceholders counts the number of $N parameter placeholders in the
// final query and truncates args to match. This prevents "got N parameters but
// the statement requires M" errors when {{step.col}} is inlined in the SQL via
// substituteVars and also referenced in args.
func (r *Runner) trimArgsToPlaceholders(query string, args []interface{}) []interface{} {
	maxN := countMaxPlaceholder(query)
	if len(args) > maxN {
		return args[:maxN]
	}
	return args
}

// countMaxPlaceholder returns the maximum $N placeholder number found in s,
// or 0 if no placeholders are present. E.g. "SELECT * FROM t WHERE a=$1 AND b=$2" → 2.
func countMaxPlaceholder(s string) int {
	maxN := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '$' && i+1 < len(s) && s[i+1] >= '1' && s[i+1] <= '9' {
			// Read the full number
			j := i + 1
			for j < len(s) && s[j] >= '0' && s[j] <= '9' {
				j++
			}
			numStr := s[i+1 : j]
			n := 0
			for _, ch := range numStr {
				n = n*10 + int(ch-'0')
			}
			if n > maxN {
				maxN = n
			}
			i = j - 1
		}
	}
	return maxN
}

// runResultAssertions runs all assertions, expect, expect_rows, expect_cols against a result.
func runResultAssertions(result *assert.Result, step config.StepConfig) error {
	// Run simple assertions — all must pass
	for i, a := range step.Assert {
		if err := assert.RunSimpleAssert(a, result); err != nil {
			if len(step.Assert) > 1 {
				return fmt.Errorf("assert[%d] (%q): %w", i, a, err)
			}
			return fmt.Errorf("assert: %w", err)
		}
	}

	// Run structured assertions
	if len(step.Assertions) > 0 {
		if err := assert.RunAssertions(step.Assertions, result); err != nil {
			return fmt.Errorf("assertions: %w", err)
		}
	}

	// Check expect value
	if step.Expect != nil {
		if result.Count == 0 {
			return fmt.Errorf("expect: no rows returned")
		}
		actual := result.Rows[0][0]
		if !deepEqual(actual, step.Expect) {
			return fmt.Errorf("expect: expected %v, got %v", step.Expect, actual)
		}
	}

	// Check expected row count
	if step.ExpectRows > 0 && result.Count != step.ExpectRows {
		return fmt.Errorf("expect_rows: expected %d, got %d", step.ExpectRows, result.Count)
	}

	// Check expected columns
	if len(step.ExpectCols) > 0 {
		if len(result.Columns) != len(step.ExpectCols) {
			return fmt.Errorf("expect_cols: expected %d columns, got %d (%v)", len(step.ExpectCols), len(result.Columns), result.Columns)
		}
		for i, expected := range step.ExpectCols {
			if !strings.EqualFold(result.Columns[i], expected) {
				return fmt.Errorf("expect_cols: column %d expected %q, got %q", i, expected, result.Columns[i])
			}
		}
	}

	return nil
}

// tryParseJSONError attempts to parse a pg error message as a JSON object.
// Returns the parsed Result and the raw JSON string (after stripping "pq: ").
// Returns nil, "" if the error is not valid JSON.
func tryParseJSONError(errStr string) (*assert.Result, string) {
	// Strip the "pq: " prefix that the pq driver adds
	cleanErr := errStr
	if strings.HasPrefix(cleanErr, "pq: ") {
		cleanErr = cleanErr[4:]
	}

	// Attempt to parse as a JSON object
	var obj map[string]interface{}
	if err := json.Unmarshal([]byte(cleanErr), &obj); err != nil {
		return nil, "" // not valid JSON, return nil to fall through to normal error handling
	}
	if len(obj) == 0 {
		return nil, "" // empty JSON object, treat as normal error
	}

	// Build columns and row values from the JSON object
	columns := make([]string, 0, len(obj))
	vals := make([]interface{}, len(obj))
	i := 0
	for k, v := range obj {
		columns = append(columns, k)
		vals[i] = v
		i++
	}

	return &assert.Result{
		Columns: columns,
		Rows:    [][]interface{}{vals},
		Count:   1,
	}, cleanErr
}

// tryParseJSONResult checks if a query result contains a single JSON string value
// (e.g. a stored procedure that returns a JSON string on success). If so, parses
// the JSON object and returns a new Result where each JSON key becomes a column.
// Returns nil if the result is not a single JSON string.
func tryParseJSONResult(result *assert.Result) *assert.Result {
	// Only applies to results with exactly 1 row and 1 column
	if result.Count != 1 || len(result.Columns) != 1 {
		return nil
	}

	// The single value must be a string
	strVal, ok := result.Rows[0][0].(string)
	if !ok || strVal == "" {
		return nil
	}

	// Trim whitespace
	strVal = strings.TrimSpace(strVal)

	// Attempt to parse as a JSON object
	var obj map[string]interface{}
	if err := json.Unmarshal([]byte(strVal), &obj); err != nil {
		return nil // not valid JSON
	}
	if len(obj) == 0 {
		return nil
	}

	// Build columns and row values from the JSON object
	columns := make([]string, 0, len(obj))
	vals := make([]interface{}, len(obj))
	i := 0
	for k, v := range obj {
		columns = append(columns, k)
		vals[i] = v
		i++
	}

	return &assert.Result{
		Columns: columns,
		Rows:    [][]interface{}{vals},
		Count:   1,
	}
}

// runSQLBatch executes a list of SQL statements (from dotsql or raw).
func (r *Runner) runSQLBatch(statements []string, label string) {
	for _, stmt := range statements {
		// Skip disabled queries (all comment lines)
		if r.dotsql.IsDisabled(stmt) {
			continue
		}
		sqlText := stmt
		if named, err := r.dotsql.Raw(stmt); err == nil {
			sqlText = named
		}
		if _, err := r.db.Exec(sqlText); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: %s: %v\n", label, err)
		}
	}
}

// deepEqual compares two values for equality with some type coercion.
func deepEqual(a, b interface{}) bool {
	as := fmt.Sprintf("%v", a)
	bs := fmt.Sprintf("%v", b)
	return as == bs
}

// maskDSN replaces password in DSN string for safe verbose output.
func maskDSN(dsn string) string {
	// Handle key=value format (lib/pq style)
	// Look for password=xxx pattern
	result := dsn
	// Try password= pattern
	if idx := strings.Index(strings.ToLower(result), "password="); idx >= 0 {
		start := idx + len("password=")
		end := start
		for end < len(result) && result[end] != ' ' {
			end++
		}
		if end > start {
			result = result[:start] + "***" + result[end:]
		}
	}
	return result
}

// printVerboseSQL prints the SQL query and its arguments to stderr.
func (r *Runner) printVerboseSQL(query string, args []interface{}) {
	fmt.Fprintf(os.Stderr, "[verbose] SQL:\n%s\n", query)
	if len(args) > 0 {
		argStrs := make([]string, len(args))
		for i, a := range args {
			switch v := a.(type) {
			case string:
				argStrs[i] = fmt.Sprintf("'%s'", v)
			case nil:
				argStrs[i] = "NULL"
			default:
				argStrs[i] = fmt.Sprintf("%v", v)
			}
		}
		fmt.Fprintf(os.Stderr, "[verbose] args: %s\n", strings.Join(argStrs, ", "))
	}
}

// printVerboseResult prints the query result (columns and rows) to stderr.
func (r *Runner) printVerboseResult(result *assert.Result) {
	if result == nil {
		fmt.Fprintf(os.Stderr, "[verbose] result: <nil>\n")
		return
	}
	fmt.Fprintf(os.Stderr, "[verbose] result: %d row(s), columns: %s\n",
		result.Count, strings.Join(result.Columns, ", "))
	for i, row := range result.Rows {
		vals := make([]string, len(row))
		for j, v := range row {
			switch val := v.(type) {
			case nil:
				vals[j] = "NULL"
			case string:
				vals[j] = fmt.Sprintf("'%s'", val)
			case []byte:
				vals[j] = fmt.Sprintf("'%s'", string(val))
			default:
				vals[j] = fmt.Sprintf("%v", val)
			}
		}
		fmt.Fprintf(os.Stderr, "[verbose]   row[%d]: %s\n", i, strings.Join(vals, ", "))
	}
}
