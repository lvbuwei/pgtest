package assert

import (
	"database/sql"
	"fmt"
	"reflect"
	"regexp"
	"strconv"
	"strings"

	"pgtest/config"
)

// Result holds query result data for assertions.
type Result struct {
	Columns []string
	Rows    [][]interface{}
	Count   int
}

// CollectRows reads all rows from a *sql.Rows into a Result.
func CollectRows(rows *sql.Rows) (*Result, error) {
	defer rows.Close()

	cols, err := rows.Columns()
	if err != nil {
		return nil, fmt.Errorf("assert.CollectRows: %w", err)
	}

	r := &Result{Columns: cols}

	for rows.Next() {
		scanArgs := make([]interface{}, len(cols))
		scanVals := make([]interface{}, len(cols))
		for i := range scanArgs {
			scanArgs[i] = &scanVals[i]
		}
		if err := rows.Scan(scanArgs...); err != nil {
			return nil, fmt.Errorf("assert.CollectRows scan: %w", err)
		}
		row := make([]interface{}, len(cols))
		for i, v := range scanVals {
			// Convert []byte to string for easier comparison
			if b, ok := v.([]byte); ok {
				row[i] = string(b)
			} else {
				row[i] = v
			}
		}
		r.Rows = append(r.Rows, row)
	}
	r.Count = len(r.Rows)
	return r, rows.Err()
}

// RunAssertion executes a single AssertConfig against a Result.
func RunAssertion(assertCfg config.AssertConfig, result *Result) error {
	switch strings.ToLower(assertCfg.Type) {
	case "equals", "eq", "==":
		return assertEquals(assertCfg, result)
	case "not_equals", "ne", "!=":
		return assertNotEquals(assertCfg, result)
	case "contains":
		return assertContains(assertCfg, result)
	case "gt", ">":
		return assertGt(assertCfg, result)
	case "lt", "<":
		return assertLt(assertCfg, result)
	case "gte", ">=":
		return assertGte(assertCfg, result)
	case "lte", "<=":
		return assertLte(assertCfg, result)
	case "matches", "regex":
		return assertMatches(assertCfg, result)
	case "is_null", "null":
		return assertNull(assertCfg, result)
	case "not_null":
		return assertNotNull(assertCfg, result)
	case "in":
		return assertIn(assertCfg, result)
	case "count":
		return assertCount(assertCfg, result)
	case "exists":
		return assertExists(assertCfg, result)
	default:
		return fmt.Errorf("unknown assertion type: %q", assertCfg.Type)
	}
}

// getValue retrieves a specific value from the Result based on column name and row index.
func getValue(column string, row int, result *Result) (interface{}, error) {
	if strings.ToLower(column) == "count" {
		return int64(result.Count), nil
	}

	colIdx := -1
	for i, c := range result.Columns {
		if strings.EqualFold(c, column) {
			colIdx = i
			break
		}
	}
	if colIdx < 0 {
		return nil, fmt.Errorf("column %q not found in result (columns: %v)", column, result.Columns)
	}

	if row == -1 {
		// collect all values for this column across all rows
		var vals []interface{}
		for _, r := range result.Rows {
			if colIdx < len(r) {
				vals = append(vals, r[colIdx])
			}
		}
		return vals, nil
	}

	if row >= len(result.Rows) {
		return nil, fmt.Errorf("row index %d out of range (total rows: %d)", row, len(result.Rows))
	}
	if colIdx >= len(result.Rows[row]) {
		return nil, fmt.Errorf("column index %d out of range in row %d", colIdx, row)
	}
	return result.Rows[row][colIdx], nil
}

// compare compares two values with type coercion for numeric types.
func compare(a, b interface{}) (int, error) {
	af, aIsNum := toFloat(a)
	bf, bIsNum := toFloat(b)

	if aIsNum && bIsNum {
		switch {
		case af < bf:
			return -1, nil
		case af > bf:
			return 1, nil
		default:
			return 0, nil
		}
	}

	aStr := fmt.Sprintf("%v", a)
	bStr := fmt.Sprintf("%v", b)
	return strings.Compare(aStr, bStr), nil
}

// equals compares with type coercion.
func equals(a, b interface{}) bool {
	af, aIsNum := toFloat(a)
	bf, bIsNum := toFloat(b)
	if aIsNum && bIsNum {
		return af == bf
	}
	return reflect.DeepEqual(a, b)
}

func toFloat(v interface{}) (float64, bool) {
	switch val := v.(type) {
	case float64:
		return val, true
	case float32:
		return float64(val), true
	case int:
		return float64(val), true
	case int64:
		return float64(val), true
	case int32:
		return float64(val), true
	case int16:
		return float64(val), true
	case int8:
		return float64(val), true
	case uint64:
		return float64(val), true
	case string:
		f, err := strconv.ParseFloat(val, 64)
		if err == nil {
			return f, true
		}
		return 0, false
	default:
		return 0, false
	}
}

// containsValue checks if a slice contains a value.
func containsValue(slice []interface{}, val interface{}) bool {
	for _, v := range slice {
		if equals(v, val) {
			return true
		}
	}
	return false
}

func assertCount(assertCfg config.AssertConfig, result *Result) error {
	expected, ok := toFloat(assertCfg.Value)
	if !ok {
		return fmt.Errorf("count assertion requires numeric value, got %T", assertCfg.Value)
	}
	if int64(result.Count) != int64(expected) {
		return fmt.Errorf("count: expected %d, got %d", int64(expected), result.Count)
	}
	return nil
}

func assertExists(assertCfg config.AssertConfig, result *Result) error {
	if result.Count == 0 {
		return fmt.Errorf("exists: expected at least 1 row, got 0")
	}
	return nil
}

func assertEquals(assertCfg config.AssertConfig, result *Result) error {
	val, err := getValue(assertCfg.Column, assertCfg.Row, result)
	if err != nil {
		return err
	}
	if !equals(val, assertCfg.Value) {
		return fmt.Errorf("equals: expected %v, got %v", assertCfg.Value, val)
	}
	return nil
}

func assertNotEquals(assertCfg config.AssertConfig, result *Result) error {
	val, err := getValue(assertCfg.Column, assertCfg.Row, result)
	if err != nil {
		return err
	}
	if equals(val, assertCfg.Value) {
		return fmt.Errorf("not_equals: value %v should not equal %v", val, assertCfg.Value)
	}
	return nil
}

func assertContains(assertCfg config.AssertConfig, result *Result) error {
	val, err := getValue(assertCfg.Column, assertCfg.Row, result)
	if err != nil {
		return err
	}
	valStr := fmt.Sprintf("%v", val)
	expectStr := fmt.Sprintf("%v", assertCfg.Value)
	if !strings.Contains(valStr, expectStr) {
		return fmt.Errorf("contains: %q does not contain %q", valStr, expectStr)
	}
	return nil
}

func assertGt(assertCfg config.AssertConfig, result *Result) error {
	val, err := getValue(assertCfg.Column, assertCfg.Row, result)
	if err != nil {
		return err
	}
	cmp, err := compare(val, assertCfg.Value)
	if err != nil {
		return err
	}
	if cmp <= 0 {
		return fmt.Errorf("gt: expected %v > %v", val, assertCfg.Value)
	}
	return nil
}

func assertLt(assertCfg config.AssertConfig, result *Result) error {
	val, err := getValue(assertCfg.Column, assertCfg.Row, result)
	if err != nil {
		return err
	}
	cmp, err := compare(val, assertCfg.Value)
	if err != nil {
		return err
	}
	if cmp >= 0 {
		return fmt.Errorf("lt: expected %v < %v", val, assertCfg.Value)
	}
	return nil
}

func assertGte(assertCfg config.AssertConfig, result *Result) error {
	val, err := getValue(assertCfg.Column, assertCfg.Row, result)
	if err != nil {
		return err
	}
	cmp, err := compare(val, assertCfg.Value)
	if err != nil {
		return err
	}
	if cmp < 0 {
		return fmt.Errorf("gte: expected %v >= %v", val, assertCfg.Value)
	}
	return nil
}

func assertLte(assertCfg config.AssertConfig, result *Result) error {
	val, err := getValue(assertCfg.Column, assertCfg.Row, result)
	if err != nil {
		return err
	}
	cmp, err := compare(val, assertCfg.Value)
	if err != nil {
		return err
	}
	if cmp > 0 {
		return fmt.Errorf("lte: expected %v <= %v", val, assertCfg.Value)
	}
	return nil
}

func assertMatches(assertCfg config.AssertConfig, result *Result) error {
	val, err := getValue(assertCfg.Column, assertCfg.Row, result)
	if err != nil {
		return err
	}
	pattern := fmt.Sprintf("%v", assertCfg.Value)
	re, err := regexp.Compile(pattern)
	if err != nil {
		return fmt.Errorf("matches: invalid regex %q: %w", pattern, err)
	}
	valStr := fmt.Sprintf("%v", val)
	if !re.MatchString(valStr) {
		return fmt.Errorf("matches: %q does not match pattern %q", valStr, pattern)
	}
	return nil
}

func assertNull(assertCfg config.AssertConfig, result *Result) error {
	val, err := getValue(assertCfg.Column, assertCfg.Row, result)
	if err != nil {
		return err
	}
	if val != nil {
		return fmt.Errorf("is_null: expected null, got %v", val)
	}
	return nil
}

func assertNotNull(assertCfg config.AssertConfig, result *Result) error {
	val, err := getValue(assertCfg.Column, assertCfg.Row, result)
	if err != nil {
		return err
	}
	if val == nil {
		return fmt.Errorf("not_null: expected non-null value")
	}
	return nil
}

func assertIn(assertCfg config.AssertConfig, result *Result) error {
	val, err := getValue(assertCfg.Column, assertCfg.Row, result)
	if err != nil {
		return err
	}
	// Value should be a slice
	rv := reflect.ValueOf(assertCfg.Value)
	if rv.Kind() != reflect.Slice {
		return fmt.Errorf("in: assertion value must be a list, got %T", assertCfg.Value)
	}
	for i := 0; i < rv.Len(); i++ {
		if equals(val, rv.Index(i).Interface()) {
			return nil
		}
	}
	return fmt.Errorf("in: %v not in %v", val, assertCfg.Value)
}

// RunAssertions executes all assertions against the result.
func RunAssertions(assertions []config.AssertConfig, result *Result) error {
	for i, a := range assertions {
		if a.Type == "" {
			return fmt.Errorf("assertion %d: type is required", i)
		}
		if err := RunAssertion(a, result); err != nil {
			return fmt.Errorf("assertion %d (%s): %w", i, a.Type, err)
		}
	}
	return nil
}

// RunSimpleAssert runs a simple assertion string like "count == 5" or "first_name == John".
func RunSimpleAssert(assertStr string, result *Result) error {
	assertStr = strings.TrimSpace(assertStr)
	if assertStr == "" {
		return nil
	}

	// Try to parse "column operator value" format
	parts := strings.SplitN(assertStr, " ", 3)
	if len(parts) < 2 {
		return fmt.Errorf("invalid assertion format: %q (expected: column operator value)", assertStr)
	}

	column := parts[0]
	operator := parts[1]
	var value interface{}
	if len(parts) >= 3 {
		value = parts[2]
	}

	// Map operator to assert type
	typeMap := map[string]string{
		"==": "equals",
		"!=": "not_equals",
		">":  "gt",
		"<":  "lt",
		">=": "gte",
		"<=": "lte",
		"=~": "matches",
	}

	assertType, ok := typeMap[operator]
	if !ok {
		assertType = operator // use raw string
	}

	return RunAssertion(config.AssertConfig{
		Type:   assertType,
		Column: column,
		Value:  value,
		Row:    0,
	}, result)
}
