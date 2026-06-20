# pgtest — PostgreSQL Stored Procedure & SQL Testing Tool

<p align="center">
  <strong>English</strong> | <a href="README.md">中文</a>
</p>

<p align="center">
  <img src="https://img.shields.io/badge/language-Go-00ADD8?style=flat&logo=go&logoColor=white" alt="Go">
  <img src="https://img.shields.io/badge/database-PostgreSQL-336791?style=flat&logo=postgresql&logoColor=white" alt="PostgreSQL">
  <img src="https://img.shields.io/badge/license-MIT-blue.svg?style=flat" alt="License">
  <img src="https://img.shields.io/badge/version-26.06.08-brightgreen?style=flat" alt="Version">
</p>

---

pgtest is a **declarative, YAML-driven** PostgreSQL database testing framework written in Go. Define test cases in YAML configuration files and automate testing of stored procedures, SQL functions, and queries — no Go code required.

## Table of Contents

- [pgtest — PostgreSQL Stored Procedure \& SQL Testing Tool](#pgtest--postgresql-stored-procedure--sql-testing-tool)
  - [Table of Contents](#table-of-contents)
  - [Features](#features)
  - [Installation](#installation)
    - [Prerequisites](#prerequisites)
    - [Build from Source](#build-from-source)
    - [Dependencies](#dependencies)
  - [Quick Start](#quick-start)
    - [1. Prepare SQL File](#1-prepare-sql-file)
    - [2. Write Test Configuration](#2-write-test-configuration)
    - [3. Run Tests](#3-run-tests)
  - [Configuration Reference](#configuration-reference)
    - [Top-Level Config](#top-level-config)
    - [Test Case (CaseConfig)](#test-case-caseconfig)
    - [Test Step (StepConfig)](#test-step-stepconfig)
  - [Assertions Reference](#assertions-reference)
    - [Simple Assertion Format](#simple-assertion-format)
    - [Structured Assertion (AssertConfig)](#structured-assertion-assertconfig)
    - [All Assertion Types](#all-assertion-types)
  - [Variable System](#variable-system)
    - [Reference Syntax](#reference-syntax)
      - [Variable Resolution Rules](#variable-resolution-rules)
    - [Environment Variable Expansion](#environment-variable-expansion)
    - [Inter-Step Data Passing](#inter-step-data-passing)
  - [Lifecycle Hooks](#lifecycle-hooks)
  - [Full Examples](#full-examples)
    - [Stored Procedure Testing](#stored-procedure-testing)
    - [Stored Procedure JSON Return Testing](#stored-procedure-json-return-testing)
    - [Async Testing with Retries](#async-testing-with-retries)
    - [Nested Test Cases](#nested-test-cases)
  - [Output Formats](#output-formats)
    - [Console (Default)](#console-default)
    - [JSON](#json)
    - [JUnit XML](#junit-xml)
  - [CI/CD Integration](#cicd-integration)
    - [GitLab CI](#gitlab-ci)
    - [Docker Compose (Local)](#docker-compose-local)
  - [CLI Options](#cli-options)
    - [Filtering \& Selection](#filtering--selection)
      - [Filter by Case Name (`-name`)](#filter-by-case-name--name)
      - [Filter by Query Name (`-query`)](#filter-by-query-name--query)
      - [Combined Usage](#combined-usage)
    - [Verbose Debug Mode](#verbose-debug-mode)
    - [Notice Debug Mode](#notice-debug-mode)
  - [Project Structure](#project-structure)
  - [Best Practices](#best-practices)
  - [License](#license)

## Features

- **Declarative Configuration** — Define test cases in YAML, zero code
- **Rich Assertion Engine** — 13 assertion types: equality, range, regex, NULL checks, and more
- **Multi-Dimensional Variables** — Global, case-level, environment variables, with `$ref` and `{{mustache}}` syntaxes
- **Named Queries** — Extract SQL into standalone `.sql` files, reference by name
- **Lifecycle Hooks** — Multi-level setup/teardown, before_all/after_all for data preparation and cleanup
- **Nested Cases** — Support sub-case hierarchies, auto-flattened to `parent.child` naming
- **Retry Mechanism** — Step-level retries with timeout support, ideal for async/eventual-consistency scenarios
- **Multi-Format Reports** — Console (human-friendly), JSON, JUnit XML
- **CI-Friendly** — Non-zero exit codes + JUnit reports, integrates with Jenkins/GitLab CI/GitHub Actions
- **Automatic JSON Parsing** — JSON strings returned by stored procedures are auto-expanded into structured columns for direct assertions
- **Safe Verbose Output** — DSN passwords automatically masked, no sensitive information leakage

## Installation

### Prerequisites

- Go 1.13+
- A PostgreSQL instance

### Build from Source

```bash
git clone https://github.com/your-org/pgtest.git
cd pgtest
go build -o pgtest .
```

This produces a `pgtest` executable in the current directory.

### Dependencies

Core dependencies:
- `github.com/lib/pq` — PostgreSQL driver
- `gopkg.in/yaml.v2` — YAML parsing

The `dotsql/` directory contains a built-in SQL named-query loader.

## Quick Start

### 1. Prepare SQL File

Create `queries.sql` and define all your SQL statements as named queries:

```sql
-- name: create-schema
CREATE TABLE IF NOT EXISTS users (
    id SERIAL PRIMARY KEY,
    name TEXT NOT NULL,
    email TEXT UNIQUE NOT NULL,
    created_at TIMESTAMP DEFAULT NOW()
);

-- name: drop-schema
DROP TABLE IF EXISTS users;

-- name: create-test-data
INSERT INTO users (name, email) VALUES
    ('Alice', 'alice@example.com'),
    ('Bob', 'bob@example.com'),
    ('Charlie', 'charlie@example.com');

-- name: cleanup-test-data
DELETE FROM users WHERE email IN ('alice@example.com', 'bob@example.com', 'charlie@example.com');

-- name: find-all-users
SELECT id, name, email FROM users ORDER BY id;

-- name: find-user-by-email
SELECT id, name, email FROM users WHERE email = $1;

-- name: user-count
SELECT count(*) AS cnt FROM users;
```

> **Syntax**: Each named query starts with `-- name: <name>` and ends with `;`. PostgreSQL `$1`, `$2` parameter placeholders are supported.
>
> **Skipping Queries**: Converting all SQL content of a query into `--` comment lines marks that query as **disabled**. It will be automatically skipped during execution (marked as SKIPPED) without error. Useful for temporarily disabling a test:
>
> ```sql
> -- name: client_new
> --CALL proc_web_client_new('{"sys_user_id":1,"method":"POST"}');
> ```

### 2. Write Test Configuration

Create `test.yaml`:

```yaml
driver: postgres
dsn: "$DATABASE_URL"

setup:
  - create-schema

teardown:
  - drop-schema

cases:
  - name: "User Query Tests"
    desc: "Verify basic user CRUD operations"
    setup:
      - create-test-data
    teardown:
      - cleanup-test-data
    steps:
      - name: "find-all-users"
        assert: "count == 3"

      - name: "find-user-by-email"
        args:
          - "alice@example.com"
        assertions:
          - type: equals
            column: "name"
            value: "Alice"
            row: 0
```

### 3. Run Tests

```bash
# Set the database connection
export DATABASE_URL="host=localhost port=5432 user=postgres dbname=testdb sslmode=disable"

# Run
pgtest -config test.yaml -sql queries.sql

# Or omit -sql (auto-derived from config filename)
pgtest -config test.yaml
```

Sample output:

```
  ✓ User Query Tests/find-all-users [12ms]
  ✓ User Query Tests/find-user-by-email [8ms]

━━━━━━━━━━━━━━━━━━━━━━━━━━
Results: 2 total, 2 passed, 0 failed, 0 skipped, 0 errors
Duration: 23ms
Status: PASSED
```

## Configuration Reference

### Top-Level Config

| Field | Type | Description |
|-------|------|-------------|
| `driver` | `string` | Database driver, default `postgres`; also supports `pgx`, `pgxpool` |
| `dsn` | `string` | Database connection string, supports `$ENV` environment variable substitution |
| `globals` | `map` | Global variables available to all test cases |
| `setup` | `[]string` | SQL executed before all tests (named queries or raw SQL) |
| `teardown` | `[]string` | SQL executed after all tests |
| `cases` | `[]CaseConfig` | List of test cases |

### Test Case (CaseConfig)

| Field | Type | Description |
|-------|------|-------------|
| `name` | `string` | Case name (required) |
| `desc` | `string` | Case description |
| `skip` | `bool` | Set to `true` to skip this case |
| `retry` | `int` | Default retry count for all steps (overridable per step) |
| `timeout` | `string` | Case-level timeout, e.g., `"30s"`, `"1m"` |
| `vars` | `map` | Case-level variables, higher priority than globals |
| `setup` | `[]string` | SQL executed before the case runs (once) |
| `teardown` | `[]string` | SQL executed after the case runs (once) |
| `before_all` | `[]string` | Runs **before each step** in this case |
| `after_all` | `[]string` | Runs **after each step** in this case |
| `steps` | `[]StepConfig` | List of test steps |
| `cases` | `[]CaseConfig` | Nested sub-cases |

### Test Step (StepConfig)

| Field | Type | Description |
|-------|------|-------------|
| `name` | `string` | Step name |
| `desc` | `string` | Step description |
| `skip` | `bool` | Skip this step |
| `query` | `string` | SQL statement or named query name (optional; defaults to dotsql query matching `name`) |
| `args` | `[]any` | Query parameters, supports `$var`, `{{var}}`, and `$step.col` variable references |
| `assert` | `string` or `[]string` | Simple assertions, e.g., `"count == 5"`; also supports list form |
| `assertions` | `[]AssertConfig` | Structured assertion list |
| `expect` | `any` | Expected value of the first row, first column |
| `expect_rows` | `int` | Expected number of returned rows |
| `expect_cols` | `[]string` | Expected column names |
| `retry` | `int` | Retry count for this step (200ms interval) |
| `timeout` | `string` | Step-level timeout, e.g., `"10s"` |

## Assertions Reference

### Simple Assertion Format

Use `column operator value` syntax in the `assert` field:

```yaml
assert: "count == 5"
assert: "name != Bob"
assert: "count > 0"
assert: "score >= 60"
assert: "email =~ .*@example\\.com"
```

Supported operators: `==`, `!=`, `>`, `<`, `>=`, `<=`, `=~` (regex match).

`assert` also supports a list form for multiple simple assertions:

```yaml
assert:
  - "count == 5"
  - "name != Bob"
```

### Structured Assertion (AssertConfig)

Use full assertion configuration in the `assertions` list:

| Field | Type | Description |
|-------|------|-------------|
| `type` | `string` | Assertion type |
| `column` | `string` | Target column name (or `count` for row count) |
| `value` | `any` | Expected value |
| `row` | `int` | Row index (0-based), `-1` means all rows |

### All Assertion Types

| Type | Aliases | Description | Example |
|------|---------|-------------|---------|
| `equals` | `eq`, `==` | Value equality (auto numeric coercion) | `{ type: equals, column: name, value: "Alice" }` |
| `not_equals` | `ne`, `!=` | Value inequality | `{ type: not_equals, column: status, value: "deleted" }` |
| `contains` | | String containment | `{ type: contains, column: email, value: "@example" }` |
| `gt` | `>` | Greater than | `{ type: gt, column: score, value: 60 }` |
| `lt` | `<` | Less than | `{ type: lt, column: age, value: 100 }` |
| `gte` | `>=` | Greater than or equal | `{ type: gte, column: count, value: 1 }` |
| `lte` | `<=` | Less than or equal | `{ type: lte, column: price, value: 999 }` |
| `matches` | `regex` | Regex match | `{ type: matches, column: email, value: "^[a-z]+@.*" }` |
| `is_null` | `null` | Value is NULL | `{ type: is_null, column: deleted_at }` |
| `not_null` | | Value is NOT NULL | `{ type: not_null, column: id }` |
| `in` | | Value in list | `{ type: in, column: status, value: ["active","pending"] }` |
| `count` | | Row count check | `{ type: count, value: 5 }` |
| `exists` | | At least one row returned | `{ type: exists }` |

> **Note**: The `count` assertion can also be written concisely as `assert: "count == 5"` with the same effect.

## Variable System

pgtest supports multiple variable layers for SQL queries and parameters:

1. **Global Variables** — Defined in top-level `globals`
2. **Case Variables** — Defined in `vars`
3. **Step Outputs** — First-row data from a previous step's result
4. **Environment Variables** — Referenced via `$VARIABLE` or `{{VARIABLE}}`

### Reference Syntax

```yaml
globals:
  schema: "public"
  limit: 100

cases:
  - name: "Variable Example"
    vars:
      email: "test@example.com"
    steps:
      - name: "Use Variables"
        query: "SELECT * FROM {{schema}}.users WHERE email = $1 LIMIT $2"
        args:
          - "$email"      # Reference case variable
          - "{{limit}}"   # Reference global variable
```

#### Variable Resolution Rules

- **In SQL (`{{mustache}}` syntax)**: case vars > global vars > step outputs
- **In args (`$var` syntax)**: step outputs > case vars > global vars > environment vars
- **In args (`{{var}}` syntax)**: step outputs > case vars > global vars

### Environment Variable Expansion

Use `$ENV` syntax directly in configuration files:

```yaml
dsn: "$DATABASE_URL"
```

If the `DATABASE_URL` environment variable is not set, the built-in default is used:

```
host=localhost port=5432 user=postgres dbname=postgres sslmode=disable
```

### Inter-Step Data Passing

pgtest supports passing the first row of a previous step's result as variables for use in subsequent steps within the same test case.

**Reference Syntax:** Three equivalent notations are supported:

| Syntax | Where to Use | Example |
|--------|-------------|---------|
| `$stepname.col` | args list, in SQL (as `$N` parameter) | `args: ["$insert.id"]` |
| `{{stepname.col}}` | args list, in SQL (inline substitution) | `query: "UPDATE ... WHERE id = {{insert.id}}"` |
| `stepname.col` (bare) | args list | `args: ["insert.id"]` |

> **Recommendation**: Use `$stepname.col` in args (uniform with variables), and `{{stepname.col}}` for inline substitution in SQL.

**How It Works:**
1. After each step succeeds, its first-row data is automatically stored in a step-output table keyed by the step's `name`
2. Subsequent steps can reference these values in args and query SQL via `$stepname.colname`, `{{stepname.colname}}`, or bare `stepname.colname`

**Example 1: Referencing Step Output in args**

```yaml
cases:
  - name: "Insert then Query"
    steps:
      - name: "Insert New User"
        query: "INSERT INTO users (name, email) VALUES ('Alice', 'alice@example.com') RETURNING id, name"
        # After success, id and name are automatically saved as step outputs

      - name: "Query by Returned ID"
        query: "SELECT * FROM users WHERE id = $1"
        args:
          - "$insert_new_user.id"     # $stepname.col — passed via parameter binding

      - name: "Equivalent References"
        query: "SELECT * FROM users WHERE id = $1 AND name = $2"
        args:
          - "insert_new_user.id"      # Bare — auto-detected as step output
          - "{{insert_new_user.name}}" # Wrapped — also supported
```

**Example 2: Inline Substitution in SQL**

Useful for substituting values directly into SQL strings (e.g., inside JSON strings) without `$N` placeholders:

```yaml
cases:
  - name: "Inline Reference Example"
    steps:
      - name: "Create Order"
        query: "INSERT INTO orders (user_id) VALUES (1) RETURNING id"
        # Step output: id

      - name: "Call Procedure with Inline Value"
        query: "master_accept_order"
        # SQL reference:
        # -- name: master_accept_order
        # CALL proc_web_order_hall('{
        #   "sys_user_id":3,
        #   "method":"PUT",
        #   "json":{"id":{{create_order.id}}}
        # }');
        # {{create_order.id}} is replaced at runtime with the actual value (e.g., 347),
        # resulting in: CALL proc_web_order_hall('{"id":347}')
```

> **Auto Parameter Trimming**: When `{{step.col}}` is substituted inline in SQL, the corresponding `$N` placeholder is removed. pgtest automatically counts the remaining `$N` placeholders and trims excess args to prevent `"got N parameters but the statement requires M"` errors. This means the following also works (the arg value is silently ignored):
>
> ```yaml
> steps:
>   - name: "Mixed Refs"
>     query: "master_accept_order"    # Already has {{step.id}} inline in SQL
>     args:
>       - "create_order.id"           # Auto-ignored (SQL has no $1)
> ```

**Example 3: Stored Procedure JSON Return**

```yaml
cases:
  - name: "Create Order then Query"
    steps:
      - name: "Call Create Order Procedure"
        query: "SELECT proc_create_order(1, 100)"
        # Returns JSON string, auto-parsed into columns: resid, resmsg, order_id

      - name: "Query Details by Order ID"
        query: "SELECT * FROM orders WHERE id = $1"
        args:
          - "$call_create_order_proc.order_id"
```

> **Variable Priority**: In SQL `{{mustache}}` substitution: case vars > global vars > step outputs. In args `$ref` resolution: step outputs > case vars > global vars > environment vars. Step outputs use `stepname.col` dot-separated syntax and won't conflict with regular variables.

## Lifecycle Hooks

pgtest provides multi-level lifecycle control:

```
Global setup
  ├─ Case 1 before_all
  │   ├─ Step 1
  │   │   └─ Case 1 after_all
  │   ├─ Step 2
  │   │   └─ Case 1 after_all
  │   └─ Case 1 teardown
  ├─ Case 2 setup
  │   └─ ...
Global teardown
```

| Hook | Scope | Execution Timing |
|------|-------|------------------|
| Global `setup` | All tests | Executed once before the first test |
| Global `teardown` | All tests | Executed once after the last test |
| Case `before_all` | Single case | Before each step in that case |
| Case `after_all` | Single case | After each step in that case |
| Case `setup` | Single case | Before all steps in the case (once) |
| Case `teardown` | Single case | After all steps in the case (once) |

```yaml
cases:
  - name: "Transaction Isolation Test"
    setup:
      - create-order-table
    teardown:
      - drop-order-table
    before_all:
      - begin-transaction
    after_all:
      - rollback-transaction
    steps:
      - name: "Insert Order"
        query: "INSERT INTO orders (amount) VALUES (100)"
      - name: "Verify Order"
        query: "SELECT count(*) AS cnt FROM orders"
        assert: "cnt == 1"
```

## Full Examples

### Stored Procedure Testing

Suppose you have a stored procedure `calculate_bonus(employee_id INT, sales_amount DECIMAL)`:

```sql
-- name: create-bonus-proc
CREATE OR REPLACE FUNCTION calculate_bonus(
    employee_id INT,
    sales_amount DECIMAL
) RETURNS DECIMAL AS $$
BEGIN
    IF sales_amount > 100000 THEN
        RETURN sales_amount * 0.10;
    ELSIF sales_amount > 50000 THEN
        RETURN sales_amount * 0.05;
    ELSE
        RETURN sales_amount * 0.02;
    END IF;
END;
$$ LANGUAGE plpgsql;
```

Corresponding YAML test configuration:

```yaml
driver: postgres
dsn: "$DATABASE_URL"

globals:
  high_threshold: 100000
  mid_threshold: 50000

setup:
  - create-bonus-proc

cases:
  - name: "Bonus Calculation — High Performance"
    desc: "Sales >100k should earn 10% bonus"
    vars:
      sales: 150000
    steps:
      - name: "Call Procedure"
        query: "SELECT calculate_bonus(1, {{sales}}) AS bonus"
        assertions:
          - type: equals
            column: "bonus"
            value: 15000

  - name: "Bonus Calculation — Mid Performance"
    desc: "Sales 50k-100k should earn 5% bonus"
    steps:
      - name: "Call Procedure"
        query: "SELECT calculate_bonus(1, 75000) AS bonus"
        assertions:
          - type: equals
            column: "bonus"
            value: 3750

  - name: "Bonus Calculation — Low Performance"
    desc: "Sales <50k should earn 2% bonus"
    steps:
      - name: "Call Procedure"
        query: "SELECT calculate_bonus(1, 30000) AS bonus"
        assertions:
          - type: equals
            column: "bonus"
            value: 600
```

### Stored Procedure JSON Return Testing

When a stored procedure returns a JSON string via `RETURN`, or a JSON error message via `RAISE EXCEPTION`, pgtest automatically detects and parses the JSON into key-value pairs. Each JSON field becomes a result column, enabling direct field assertions.

**Successful JSON Return (RETURN):**

```sql
-- Stored procedure: returns JSON string
CREATE OR REPLACE FUNCTION proc_create_order(
    p_user_id INT,
    p_product_id INT
) RETURNS TEXT AS $$
DECLARE
    v_result TEXT;
BEGIN
    -- Business logic...
    v_result := '{"resid":0,"resmsg":"Order created","order_id":12345}';
    RETURN v_result;
END;
$$ LANGUAGE plpgsql;
```

```yaml
# test.yaml — Assertions on a stored procedure's successful JSON return
cases:
  - name: "Create Order Success"
    steps:
      - name: "Call Procedure"
        query: "SELECT proc_create_order(1, 100)"
        assertions:
          - type: equals
            column: resid
            value: 0
          - type: equals
            column: resmsg
            value: "Order created"
          - type: gt
            column: order_id
            value: 0
```

> **Note**: If the query result is exactly 1 row × 1 column and the value is a valid JSON object string, pgtest automatically expands it into multiple columns.

**Error JSON Return (RAISE EXCEPTION):**

```sql
-- Stored procedure: returns JSON error via RAISE EXCEPTION
CREATE OR REPLACE PROCEDURE proc_web_client_new(
    p_data JSON
) AS $$
DECLARE
    v_address_id INT;
BEGIN
    v_address_id := (p_data->>'address_id')::INT;
    IF v_address_id IS NULL OR v_address_id <= 0 THEN
        RAISE EXCEPTION '{"resid":-40,"resmsg":"Please select an address"}';
    END IF;
    -- Business logic...
END;
$$ LANGUAGE plpgsql;
```

```sql
-- queries.sql
-- name: client_new
CALL proc_web_client_new('{"sys_user_id":1,"method":"POST"}');
```

```yaml
# test.yaml — Assertions on RAISE EXCEPTION JSON error
cases:
  - name: "Web Client — Missing Address"
    steps:
      - name: "Call Procedure"
        query: "client_new"
        assertions:
          - type: equals
            column: resid
            value: -40
          - type: equals
            column: resmsg
            value: "Please select an address"

      - name: "Call Procedure with Params"
        query: "CALL proc_web_client_new($1)"
        args:
          - '{"sys_user_id":1,"method":"POST","address_id":5}'
        assertions:
          - type: equals
            column: resid
            value: 0
          - type: equals
            column: resmsg
            value: "Order created"
```

> **How it works**: When PostgreSQL throws an exception, the pq driver captures the error message as `pq: {"resid":-40,"resmsg":"..."}`. pgtest strips the `pq: ` prefix, checks if the remaining string is valid JSON, and if so parses it into structured column/row data, then runs the normal assertion flow. Regular database errors in non-JSON format (e.g., syntax errors, connection failures) are unaffected and reported as usual.

### Async Testing with Retries

Ideal for testing async tasks, message-queue consumption results, and similar scenarios:

```yaml
cases:
  - name: "Async Task Test"
    desc: "Verify data is correctly updated after message-queue processing"
    steps:
      - name: "Trigger Async Job"
        query: "SELECT trigger_async_job('process_orders')"

      - name: "Wait and Verify Result"
        retry: 10        # Retry up to 10 times
        timeout: "30s"   # Timeout after 30 seconds
        query: "SELECT status FROM orders WHERE id = 1"
        assertions:
          - type: equals
            column: "status"
            value: "completed"
```

### Nested Test Cases

Organize tests by functional module:

```yaml
cases:
  - name: "User Module"
    cases:
      - name: "Registration"
        steps:
          - name: "Create User"
            query: "INSERT INTO users (name, email) VALUES ('Test', 'test@test.com') RETURNING id"
          - name: "Verify Creation"
            query: "SELECT count(*) AS cnt FROM users WHERE email = 'test@test.com'"
            assert: "cnt == 1"

      - name: "Login"
        steps:
          - name: "Verify Password"
            query: "SELECT verify_password('test@test.com', 'password123') AS valid"
            assertions:
              - type: equals
                column: "valid"
                value: true
```

> Nested cases are auto-expanded to `User Module.Registration` and `User Module.Login`.

## Output Formats

### Console (Default)

```
  ✓ CaseName/StepName [15ms]
  ✗ Failing Step [8ms]
    Error: equals: expected "Bob", got "Alice"
  ○ Skipped Case []
  ⚠ Errored Step [3ms]
    Error: query error: connection refused

━━━━━━━━━━━━━━━━━━━━━━━━━━
Results: 4 total, 1 passed, 1 failed, 1 skipped, 1 errors
Duration: 30ms
Status: FAILED
```

### JSON

```bash
pgtest -format json
```

Outputs machine-readable JSON with detailed test information:

```json
{
  "tests": [
    {
      "status": "PASSED",
      "name": "CaseName/StepName",
      "duration": 15000000,
      "error": "",
      "retries": 0
    }
  ],
  "summary": {
    "total": 4,
    "passed": 3,
    "failed": 1,
    "skipped": 0,
    "errors": 0,
    "duration": 30000000
  }
}
```

### JUnit XML

```bash
pgtest -format junit > report.xml
```

Generates standard JUnit XML format, directly consumable by Jenkins, GitLab CI, GitHub Actions, and other CI systems.

## CI/CD Integration

### GitLab CI

```yaml
# .gitlab-ci.yml
pgtest:
  image: golang:1.21
  services:
    - postgres:15
  variables:
    DATABASE_URL: "host=postgres user=postgres dbname=postgres sslmode=disable"
  script:
    - cd pgtest
    - go build -o pgtest .
    - ./pgtest -config test.yaml -sql queries.sql -format junit -verbose > report.xml
  artifacts:
    reports:
      junit: pgtest/report.xml
    when: always
```

### Docker Compose (Local)

```yaml
# docker-compose.yml
version: '3'
services:
  postgres:
    image: postgres:15
    environment:
      POSTGRES_PASSWORD: testpass
      POSTGRES_DB: testdb
    ports:
      - "5432:5432"

  pgtest:
    build: .
    depends_on:
      - postgres
    environment:
      DATABASE_URL: "host=postgres port=5432 user=postgres password=testpass dbname=testdb sslmode=disable"
    command: -config test.yaml -sql queries.sql
```

## CLI Options

```
Usage: pgtest [options]

Options:
  -config string    Path to test configuration file (default "test.yaml")
  -sql string       Path to SQL named-query file (default "queries.sql").
                    When -config is explicitly set and -sql is not,
                    the SQL filename is auto-derived from the config filename
                    (extensions replaced with .sql)
  -format string    Output format: console, json, junit (default "console")
  -verbose          Enable verbose output: prints DSN, connection status, case names
                    Note: passwords in DSN are automatically masked (default false)
  -name string      Run only the test case matching the given name;
                    supports Chinese names and nested case dot-separated format
  -query string     Run only steps whose query field matches the given value
  -notice           Print RAISE NOTICE messages from PostgreSQL procedures (default false)
  -stop-on-error    Stop executing subsequent test cases on error (default true)
  -version          Print version information and exit
  -help             Show help information and exit
```

### Filtering & Selection

During development, you often only need to run a single test case or a specific query. pgtest provides `-name` and `-query` for this purpose:

#### Filter by Case Name (`-name`)

When `-name` is specified, only the matching test case is run. Supports Chinese names and nested cases (dot-separated format):

```bash
# Run the case named "User Query Tests"
pgtest -name "User Query Tests"

# Run nested case "User Module.Registration"
pgtest -name "User Module.Registration"

# Combine with verbose
pgtest -name "User Query Tests" -verbose
```

> If no matching case is found, pgtest exits with an error: `Error: no test case found with name "xxx"`

#### Filter by Query Name (`-query`)

When `-query` is specified, only steps whose `query` field matches are run. If a step does not have a `query` field, its `name` is used for matching. Non-matching steps and cases are automatically filtered out:

```bash
# Run only steps with query "find-all-users" (across all cases)
pgtest -query "find-all-users"

# Run steps with a specific raw query
pgtest -query "CALL proc_web_order_hall(...)"

# If the step name matches the dotsql query name (query omitted), it still matches
pgtest -query "client_new"

# Combine with verbose
pgtest -query "find-all-users" -verbose
```

> If no matching steps are found, pgtest exits with an error: `Error: no steps found matching query "xxx"`

#### Combined Usage

`-name` and `-query` can be used together — first filter by case name, then by query:

```bash
# Narrow down to a specific case, then filter its queries
pgtest -name "User Query Tests" -query "find-all-users"
```

### Verbose Debug Mode

When your test configuration doesn't work or you can't connect to the database, use `-verbose` to quickly diagnose the issue:

```bash
pgtest -config test.yaml -verbose
```

Sample output:

```
[verbose] config file: test.yaml
[verbose] sql file: queries.sql
[verbose] database driver: postgres
[verbose] database dsn: host=localhost port=5432 user=postgres dbname=postgres sslmode=disable
[verbose] connecting to database...
[verbose] database connection established
[verbose] running case: User Query Tests
  ✓ User Query Tests/Find All Users [12ms]
[verbose] running case: Edge Cases
  ✓ Edge Cases/Empty Result Check [3ms]
[verbose] skipping case: Skipped Case

━━━━━━━━━━━━━━━━━━━━━━━━━━
Results: 3 total, 2 passed, 0 failed, 1 skipped, 0 errors
Duration: 15ms
Status: PASSED
```

> **Security Note**: Password fields in the DSN are automatically masked as `***` and will not leak in verbose output.

### Notice Debug Mode

When executing PostgreSQL stored procedures or functions that contain `RAISE NOTICE` statements, use `-notice` to print these messages to stderr for debugging internal logic:

```bash
pgtest -config test.yaml -notice
```

Sample output:

```
[notice] Starting bonus calculation, Employee ID: 1, Sales: 150000
[notice] High performance tier applied, bonus rate: 10%
  ✓ Bonus — High Performance/Call Procedure [15ms]
```

```sql
-- Stored procedure example
CREATE OR REPLACE FUNCTION calculate_bonus(emp_id INT, sales DECIMAL)
RETURNS DECIMAL AS $$
BEGIN
    RAISE NOTICE 'Starting bonus calculation, Employee ID: %, Sales: %', emp_id, sales;
    IF sales > 100000 THEN
        RAISE NOTICE 'High performance tier applied, bonus rate: 10%';
        RETURN sales * 0.10;
    END IF;
    ...
END;
$$ LANGUAGE plpgsql;
```

> **Note**: Notice messages are output to stderr and do not affect the test report output on stdout (console/json/junit).

If the database connection fails, verbose output clearly indicates where it failed:

```
[verbose] database driver: postgres
[verbose] database dsn: host=localhost port=5432 user=postgres dbname=postgres sslmode=disable
[verbose] connecting to database...
[verbose] database ping failed: dial tcp 127.0.0.1:5432: connect: connection refused
Error creating runner: runner.New: ping db: dial tcp 127.0.0.1:5432: connect: connection refused
```

## Project Structure

```
pgtest/
├── main.go              # CLI entry point
├── config/
│   └── config.go        # YAML parsing, DSN management, case flattening
├── assert/
│   └── assert.go        # Assertion engine, type coercion, result collection
├── runner/
│   ├── runner.go        # Test executor, variable substitution, retry logic, JSON parsing
│   └── reporter.go      # Multi-format report output (console/json/junit)
├── dotsql/
│   ├── dotsql.go        # Named SQL query loader
│   └── scanner.go       # SQL file scanning and parsing
├── go.mod
├── go.sum
└── test.yaml            # Sample configuration
```

## Best Practices

1. **Separate SQL from Config** — Place complex SQL in `queries.sql` and reference by name to keep YAML clean
2. **Match Config & SQL Filenames** — Keep `.yaml` config files and their corresponding `.sql` files with the same base name (e.g., `user_test.yaml` and `user_test.sql`). This way, specifying `-config user_test.yaml` auto-derives the SQL file path, eliminating the need for a separate `-sql` parameter
3. **Isolate Data per Case** — Use `setup`/`teardown` to prepare and clean up data per case, preventing cross-case interference
4. **Use Retries Judiciously** — Only enable retries for async or eventual-consistency scenarios; avoid masking real bugs
5. **Cover Edge Cases** — Use `is_null`, `not_null`, `matches`, and other assertions to cover NULL handling, format validation, and boundary conditions
6. **Integrate JUnit** — Use `-format junit` in CI to get visual test-trend charts
7. **Security First** — DSN passwords are automatically masked in verbose output; never hardcode passwords in YAML, always use environment variables

## License

MIT License. See LICENSE files in sub-packages for details.