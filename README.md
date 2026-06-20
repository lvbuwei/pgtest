# pgtest — PostgreSQL Stored Procedure & SQL Testing Tool

<p align="center">
  <a href="README_en.md">English</a> | <strong>中文</strong>
</p>

<p align="center">
  <img src="https://img.shields.io/badge/language-Go-00ADD8?style=flat&logo=go&logoColor=white" alt="Go">
  <img src="https://img.shields.io/badge/database-PostgreSQL-336791?style=flat&logo=postgresql&logoColor=white" alt="PostgreSQL">
  <img src="https://img.shields.io/badge/license-MIT-blue.svg?style=flat" alt="License">
  <img src="https://img.shields.io/badge/version-26.06.08-brightgreen?style=flat" alt="Version">
</p>

---

pgtest 是一个用 Go 编写的、**声明式 YAML 驱动**的 PostgreSQL 数据库测试框架。通过编写 YAML 配置文件来定义测试用例，无需编写 Go 代码即可对存储过程、SQL 函数和查询进行自动化测试。

## 目录

- [pgtest — PostgreSQL Stored Procedure \& SQL Testing Tool](#pgtest--postgresql-stored-procedure--sql-testing-tool)
  - [目录](#目录)
  - [特性](#特性)
  - [安装](#安装)
    - [前置条件](#前置条件)
    - [从源码构建](#从源码构建)
    - [依赖](#依赖)
  - [快速开始](#快速开始)
    - [1. 准备 SQL 文件](#1-准备-sql-文件)
    - [2. 编写测试配置](#2-编写测试配置)
    - [3. 运行测试](#3-运行测试)
  - [配置文件详解](#配置文件详解)
    - [顶层配置](#顶层配置)
    - [测试用例 (CaseConfig)](#测试用例-caseconfig)
    - [测试步骤 (StepConfig)](#测试步骤-stepconfig)
  - [断言参考](#断言参考)
    - [简洁断言格式](#简洁断言格式)
    - [结构化断言 (AssertConfig)](#结构化断言-assertconfig)
    - [所有断言类型](#所有断言类型)
  - [变量系统](#变量系统)
    - [引用语法](#引用语法)
      - [变量解析规则](#变量解析规则)
    - [环境变量展开](#环境变量展开)
    - [步骤间数据传递](#步骤间数据传递)
  - [生命周期钩子](#生命周期钩子)
  - [完整示例](#完整示例)
    - [存储过程测试](#存储过程测试)
    - [存储过程 JSON 返回值测试](#存储过程-json-返回值测试)
    - [带重试的异步测试](#带重试的异步测试)
    - [嵌套用例](#嵌套用例)
  - [输出格式](#输出格式)
    - [Console（默认）](#console默认)
    - [JSON](#json)
    - [JUnit XML](#junit-xml)
  - [CI/CD 集成](#cicd-集成)
    - [GitHub Actions](#github-actions)
    - [GitLab CI](#gitlab-ci)
    - [Docker Compose 本地运行](#docker-compose-本地运行)
  - [命令行参数](#命令行参数)
    - [过滤与筛选](#过滤与筛选)
      - [按用例名称筛选 (`-name`)](#按用例名称筛选--name)
      - [按查询名称筛选 (`-query`)](#按查询名称筛选--query)
      - [组合使用](#组合使用)
    - [Verbose 调试模式](#verbose-调试模式)
    - [Notice 调试模式](#notice-调试模式)
  - [项目结构](#项目结构)
  - [最佳实践](#最佳实践)
  - [License](#license)

## 特性

- **声明式配置** — YAML 定义测试用例，零代码编写
- **丰富的断言引擎** — 13 种断言类型，覆盖等值、范围、正则、NULL 检查等
- **多维变量系统** — 支持全局变量、用例级变量、环境变量，以及 `$ref` 和 `{{mustache}}` 两种引用语法
- **命名查询** — 将 SQL 提取到独立的 `.sql` 文件中，通过名称引用
- **生命周期钩子** — setup/teardown、before_all/after_all 多层次数据准备与清理
- **嵌套用例** — 支持子用例层级，自动扁平化为 `parent.child` 命名
- **重试机制** — 步骤级重试，支持超时设置，适合处理异步/最终一致性场景
- **多格式报告** — console（友好文本）、JSON、JUnit XML
- **CI 友好** — 非零退出码 + JUnit 报告，可集成 Jenkins/GitLab CI/GitHub Actions
- **JSON 自动解析** — 存储过程返回的 JSON 字符串自动展开为结构化列，支持直接断言
- **安全的 Verbose 输出** — DSN 密码自动脱敏，敏感信息不外泄

## 安装

### 前置条件

- Go 1.13+
- PostgreSQL 实例

### 从源码构建

```bash
git clone https://github.com/your-org/pgtest.git
cd pgtest
go build -o pgtest .
```

构建完成后会在当前目录生成 `pgtest` 可执行文件。

### 依赖

核心依赖：
- `github.com/lib/pq` — PostgreSQL 驱动
- `gopkg.in/yaml.v2` — YAML 解析

`dotsql/` 目录中包含一个内嵌的 SQL 命名查询加载器。

## 快速开始

### 1. 准备 SQL 文件

创建 `queries.sql`，将所有测试用的 SQL 语句定义为命名查询：

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

> **语法说明**：每条命名查询以 `-- name: <名称>` 开头，以分号 `;` 结束。支持 PostgreSQL 的 `$1`, `$2` 参数占位符。
>
> **跳过查询**：如果将查询的所有 SQL 内容都改为 `--` 开头的注释行，该查询会被标记为 disabled，执行时自动跳过（标记为 SKIPPED），不会报错。适合临时禁用某条测试：
>
> ```sql
> -- name: client_new
> --CALL proc_web_client_new('{"sys_user_id":1,"method":"POST"}');
> ```

### 2. 编写测试配置

创建 `test.yaml`：

```yaml
driver: postgres
dsn: "$DATABASE_URL"

setup:
  - create-schema

teardown:
  - drop-schema

cases:
  - name: "用户查询测试"
    desc: "验证基本的用户 CRUD 查询"
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

### 3. 运行测试

```bash
# 设置数据库连接
export DATABASE_URL="host=localhost port=5432 user=postgres dbname=testdb sslmode=disable"

# 运行
pgtest -config test.yaml -sql queries.sql

# 或省略 -sql（自动从配置文件名推导 SQL 文件）
pgtest -config test.yaml
```

输出示例：

```
  ✓ 用户查询测试/find-all-users [12ms]
  ✓ 用户查询测试/find-user-by-email [8ms]

━━━━━━━━━━━━━━━━━━━━━━━━━━
Results: 2 total, 2 passed, 0 failed, 0 skipped, 0 errors
Duration: 23ms
Status: PASSED
```

## 配置文件详解

### 顶层配置

| 字段 | 类型 | 说明 |
|------|------|------|
| `driver` | `string` | 数据库驱动，默认 `postgres`，也支持 `pgx`、`pgxpool` |
| `dsn` | `string` | 数据库连接字符串，支持 `$ENV` 环境变量替换 |
| `globals` | `map` | 全局变量，所有测试用例可用 |
| `setup` | `[]string` | 所有测试运行前执行的 SQL（命名查询名或原始 SQL） |
| `teardown` | `[]string` | 所有测试运行后执行的 SQL |
| `cases` | `[]CaseConfig` | 测试用例列表 |

### 测试用例 (CaseConfig)

| 字段 | 类型 | 说明 |
|------|------|------|
| `name` | `string` | 用例名称（必填） |
| `desc` | `string` | 用例描述 |
| `skip` | `bool` | 设为 `true` 跳过此用例 |
| `retry` | `int` | 所有步骤的默认重试次数（步骤级可覆盖） |
| `timeout` | `string` | 用例级超时，例如 `"30s"`、`"1m"` |
| `vars` | `map` | 用例级变量，优先级高于全局变量 |
| `setup` | `[]string` | 用例执行前运行的 SQL（一次性） |
| `teardown` | `[]string` | 用例执行后运行的 SQL（一次性） |
| `before_all` | `[]string` | 用例中**每个步骤**执行前运行 |
| `after_all` | `[]string` | 用例中**每个步骤**执行后运行 |
| `steps` | `[]StepConfig` | 测试步骤列表 |
| `cases` | `[]CaseConfig` | 嵌套子用例 |

### 测试步骤 (StepConfig)

| 字段 | 类型 | 说明 |
|------|------|------|
| `name` | `string` | 步骤名称 |
| `desc` | `string` | 步骤描述 |
| `skip` | `bool` | 跳过此步骤 |
| `query` | `string` | SQL 语句或命名查询名（可选，省略时用 `name` 作为 dotsql 查询名） |
| `args` | `[]any` | 查询参数，支持 `$var`、`{{var}}` 和 `$step.col` 变量引用 |
| `assert` | `string` 或 `[]string` | 简洁断言，如 `"count == 5"`，也支持多行列表 |
| `assertions` | `[]AssertConfig` | 结构化断言列表 |
| `expect` | `any` | 期望第一行第一列的值 |
| `expect_rows` | `int` | 期望返回的行数 |
| `expect_cols` | `[]string` | 期望的列名列表 |
| `retry` | `int` | 此步骤的重试次数（每次间隔 200ms） |
| `timeout` | `string` | 步骤级超时，例如 `"10s"` |

## 断言参考

### 简洁断言格式

在 `assert` 字段中可以使用 `列名 操作符 值` 的格式：

```yaml
assert: "count == 5"
assert: "name != Bob"
assert: "count > 0"
assert: "score >= 60"
assert: "email =~ .*@example\\.com"
```

支持的操作符：`==`、`!=`、`>`、`<`、`>=`、`<=`、`=~`（正则匹配）。

`assert` 字段也支持列表形式，可以同时写多条简洁断言：

```yaml
assert:
  - "count == 5"
  - "name != Bob"
```

### 结构化断言 (AssertConfig)

在 `assertions` 列表中使用完整的断言配置：

| 字段 | 类型 | 说明 |
|------|------|------|
| `type` | `string` | 断言类型 |
| `column` | `string` | 目标列名（或 `count` 表示行数） |
| `value` | `any` | 期望值 |
| `row` | `int` | 行索引（0-based），`-1` 表示所有行 |

### 所有断言类型

| 类型 | 别名 | 说明 | 示例 |
|------|------|------|------|
| `equals` | `eq`, `==` | 值相等（数值类型自动转换） | `{ type: equals, column: name, value: "Alice" }` |
| `not_equals` | `ne`, `!=` | 值不相等 | `{ type: not_equals, column: status, value: "deleted" }` |
| `contains` | | 字符串包含 | `{ type: contains, column: email, value: "@example" }` |
| `gt` | `>` | 大于 | `{ type: gt, column: score, value: 60 }` |
| `lt` | `<` | 小于 | `{ type: lt, column: age, value: 100 }` |
| `gte` | `>=` | 大于等于 | `{ type: gte, column: count, value: 1 }` |
| `lte` | `<=` | 小于等于 | `{ type: lte, column: price, value: 999 }` |
| `matches` | `regex` | 正则匹配 | `{ type: matches, column: email, value: "^[a-z]+@.*" }` |
| `is_null` | `null` | 值为 NULL | `{ type: is_null, column: deleted_at }` |
| `not_null` | | 值非 NULL | `{ type: not_null, column: id }` |
| `in` | | 值在列表中 | `{ type: in, column: status, value: ["active","pending"] }` |
| `count` | | 行数检查 | `{ type: count, value: 5 }` |
| `exists` | | 至少返回一行 | `{ type: exists }` |

> **注意**：`count` 断言也可以简洁地使用 `assert: "count == 5"`，效果相同。

## 变量系统

pgtest 支持多层变量，供 SQL 查询和参数使用：

1. **全局变量** — 在顶层 `globals` 中定义
2. **用例变量** — 在 `vars` 中定义
3. **步骤输出** — 上一步执行结果的第一行数据
4. **环境变量** — 通过 `$VARIABLE` 或 `{{VARIABLE}}` 引用

### 引用语法

```yaml
globals:
  schema: "public"
  limit: 100

cases:
  - name: "变量示例"
    vars:
      email: "test@example.com"
    steps:
      - name: "使用变量"
        query: "SELECT * FROM {{schema}}.users WHERE email = $1 LIMIT $2"
        args:
          - "$email"      # 引用用例变量
          - "{{limit}}"   # 引用全局变量
```

#### 变量解析规则

- **SQL 中（`{{mustache}}` 语法）**：用例变量 > 全局变量 > 步骤输出
- **args 中（`$var` 语法）**：步骤输出 > 用例变量 > 全局变量 > 环境变量
- **args 中（`{{var}}` 语法）**：步骤输出 > 用例变量 > 全局变量

### 环境变量展开

配置文件中可以直接使用 `$ENV` 语法：

```yaml
dsn: "$DATABASE_URL"
```

如果环境变量 `DATABASE_URL` 未设置，将使用内置默认值：

```
host=localhost port=5432 user=postgres dbname=postgres sslmode=disable
```

### 步骤间数据传递

pgtest 支持在同一个测试用例中，将上一个步骤返回的第一行数据作为变量供后续步骤使用。

**引用语法：** 支持三种写法，效果完全相同：

| 语法 | 适用位置 | 示例 |
|------|----------|------|
| `$stepname.col` | args 列表，SQL 中（作为 `$N` 参数传递） | `args: ["$insert.id"]` |
| `{{stepname.col}}` | args 列表，SQL 中直接内联替换 | `query: "UPDATE ... WHERE id = {{insert.id}}"` |
| `stepname.col`（裸引用） | args 列表 | `args: ["insert.id"]` |

> **推荐**：args 中使用 `$stepname.col`（与变量统一），SQL 中直接内联使用 `{{stepname.col}}`。

**工作流程：**
1. 每个步骤成功后，其第一行数据自动存入步骤输出表，key 为步骤的 `name`
2. 后续步骤在 args 和 query SQL 中可以通过 `$步骤名.列名`、`{{步骤名.列名}}` 或裸 `步骤名.列名` 引用这些值

**示例一：args 中引用步骤输出**

```yaml
cases:
  - name: "插入并查询"
    steps:
      - name: "插入新用户"
        query: "INSERT INTO users (name, email) VALUES ('Alice', 'alice@example.com') RETURNING id, name"
        # 步骤成功后，id 和 name 自动保存为步骤输出

      - name: "用上一步返回的id查询"
        query: "SELECT * FROM users WHERE id = $1"
        args:
          - "$insert_new_user.id"     # $变量名.列名 — 通过参数绑定传递

      - name: "三种写法的等价引用"
        query: "SELECT * FROM users WHERE id = $1 AND name = $2"
        args:
          - "insert_new_user.id"      # 裸引用 — 自动识别为步骤输出
          - "{{insert_new_user.name}}" # {{包裹}} — 也支持
```

**示例二：SQL 中直接内联替换**

适用于在 SQL 字符串中直接替换值（如 JSON 字符串内部），无需 `$N` 占位符：

```yaml
cases:
  - name: "内联引用示例"
    steps:
      - name: "创建订单"
        query: "INSERT INTO orders (user_id) VALUES (1) RETURNING id"
        # 步骤输出：id

      - name: "用内联值调用存储过程"
        query: "master_accept_order"
        # SQL 参考：
        # -- name: master_accept_order
        # CALL proc_web_order_hall('{
        #   "sys_user_id":3,
        #   "method":"PUT",
        #   "json":{"id":{{create_order.id}}}
        # }');
        # {{create_order.id}} 在运行时被替换为实际值（如 347），
        # 最终 SQL 为 CALL proc_web_order_hall('{"id":347}')
```

> **自动参数裁剪**：当 `{{step.col}}` 在 SQL 中直接内联替换后，sql 不再需要对应的 `$N` 占位符，pgtest 会自动统计最终 SQL 的实际 `$N` 数量，截断多余的 args，防止 `"got N parameters but the statement requires M"` 错误。因此以下写法也能正常工作（args 中的值会被自动忽略）：
>
> ```yaml
> steps:
>   - name: "混合引用"
>     query: "master_accept_order"    # SQL 中已内联 {{step.id}}
>     args:
>       - "create_order.id"           # 会被自动忽略（SQL 无 $1）
> ```

**示例三：存储过程 JSON 返回值**

```yaml
cases:
  - name: "创建订单后查询"
    steps:
      - name: "调用创建订单存储过程"
        query: "SELECT proc_create_order(1, 100)"
        # 返回 JSON 字符串，自动解析为列：resid, resmsg, order_id

      - name: "用返回的订单ID查询详情"
        query: "SELECT * FROM orders WHERE id = $1"
        args:
          - "$call_create_order_proc.order_id"
```

> **变量优先级：** 在 SQL `{{mustache}}` 替换中，用例变量 > 全局变量 > 步骤输出；在 args `$ref` 解析中，步骤输出 > 用例变量 > 全局变量 > 环境变量。步骤输出使用 `stepname.col` 的点号分隔语法，不会与普通变量冲突。

## 生命周期钩子

pgtest 提供多层级的生命周期控制：

```
全局 setup
  ├─ 用例1 before_all
  │   ├─ 步骤1
  │   │   └─ 用例1 after_all
  │   ├─ 步骤2
  │   │   └─ 用例1 after_all
  │   └─ 用例1 teardown
  ├─ 用例2 setup
  │   └─ ...
全局 teardown
```

| 钩子 | 作用域 | 执行时机 |
|------|--------|----------|
| 全局 `setup` | 所有测试 | 第一个测试之前执行一次 |
| 全局 `teardown` | 所有测试 | 最后一个测试之后执行一次 |
| 用例 `before_all` | 单个用例 | 该用例每个步骤执行前 |
| 用例 `after_all` | 单个用例 | 该用例每个步骤执行后 |
| 用例 `setup` | 单个用例 | 该用例所有步骤执行前（一次） |
| 用例 `teardown` | 单个用例 | 该用例所有步骤执行后（一次） |

```yaml
cases:
  - name: "事务隔离测试"
    setup:
      - create-order-table
    teardown:
      - drop-order-table
    before_all:
      - begin-transaction
    after_all:
      - rollback-transaction
    steps:
      - name: "插入订单"
        query: "INSERT INTO orders (amount) VALUES (100)"
      - name: "验证订单"
        query: "SELECT count(*) AS cnt FROM orders"
        assert: "cnt == 1"
```

## 完整示例

### 存储过程测试

假设你有一个存储过程 `calculate_bonus(employee_id INT, sales_amount DECIMAL)`：

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

对应的 YAML 测试配置：

```yaml
driver: postgres
dsn: "$DATABASE_URL"

globals:
  high_threshold: 100000
  mid_threshold: 50000

setup:
  - create-bonus-proc

cases:
  - name: "奖金计算-高业绩"
    desc: "销售额超过10万应获得10%奖金"
    vars:
      sales: 150000
    steps:
      - name: "调用存储过程"
        query: "SELECT calculate_bonus(1, {{sales}}) AS bonus"
        assertions:
          - type: equals
            column: "bonus"
            value: 15000

  - name: "奖金计算-中等业绩"
    desc: "销售额5-10万应获得5%奖金"
    steps:
      - name: "调用存储过程"
        query: "SELECT calculate_bonus(1, 75000) AS bonus"
        assertions:
          - type: equals
            column: "bonus"
            value: 3750

  - name: "奖金计算-低业绩"
    desc: "销售额低于5万应获得2%奖金"
    steps:
      - name: "调用存储过程"
        query: "SELECT calculate_bonus(1, 30000) AS bonus"
        assertions:
          - type: equals
            column: "bonus"
            value: 600
```

### 存储过程 JSON 返回值测试

当存储过程通过 `RETURN` 返回 JSON 字符串，或通过 `RAISE EXCEPTION` 返回 JSON 错误消息时，pgtest 会自动检测并将 JSON 解析为键值对，每个 JSON 字段映射为一个结果列，方便直接对字段值进行断言。

**成功返回 JSON（RETURN）：**

```sql
-- 存储过程示例：返回 JSON 字符串
CREATE OR REPLACE FUNCTION proc_create_order(
    p_user_id INT,
    p_product_id INT
) RETURNS TEXT AS $$
DECLARE
    v_result TEXT;
BEGIN
    -- 业务逻辑...
    v_result := '{"resid":0,"resmsg":"创建成功","order_id":12345}';
    RETURN v_result;
END;
$$ LANGUAGE plpgsql;
```

```yaml
# test.yaml — 对存储过程成功返回的 JSON 做断言
cases:
  - name: "创建订单成功"
    steps:
      - name: "调用存储过程"
        query: "SELECT proc_create_order(1, 100)"
        assertions:
          - type: equals
            column: resid
            value: 0
          - type: equals
            column: resmsg
            value: "创建成功"
          - type: gt
            column: order_id
            value: 0
```

> **注意**：如果查询结果恰好为 1 行 1 列且该值为合法 JSON 对象字符串，pgtest 会自动将其展开为多列。

**错误返回 JSON（RAISE EXCEPTION）：**

```sql
-- 存储过程示例：通过 RAISE EXCEPTION 返回 JSON 错误
CREATE OR REPLACE PROCEDURE proc_web_client_new(
    p_data JSON
) AS $$
DECLARE
    v_address_id INT;
BEGIN
    v_address_id := (p_data->>'address_id')::INT;
    IF v_address_id IS NULL OR v_address_id <= 0 THEN
        RAISE EXCEPTION '{"resid":-40,"resmsg":"请选择一个地址"}';
    END IF;
    -- 业务逻辑...
END;
$$ LANGUAGE plpgsql;
```

```sql
-- queries.sql
-- name: client_new
CALL proc_web_client_new('{"sys_user_id":1,"method":"POST"}');
```

```yaml
# test.yaml — 对 RAISE EXCEPTION 返回的 JSON 做断言
cases:
  - name: "Web客户端-缺少地址"
    steps:
      - name: "调用存储过程"
        query: "client_new"
        assertions:
          - type: equals
            column: resid
            value: -40
          - type: equals
            column: resmsg
            value: "请选择一个地址"

      - name: "调用存储过程带参数"
        query: "CALL proc_web_client_new($1)"
        args:
          - '{"sys_user_id":1,"method":"POST","address_id":5}'
        assertions:
          - type: equals
            column: resid
            value: 0
          - type: equals
            column: resmsg
            value: "创建成功"
```

> **原理**：当 PostgreSQL 抛出异常时，pq 驱动会将错误消息捕获为 `pq: {"resid":-40,"resmsg":"..."}`。pgtest 自动去掉 `pq: ` 前缀后检测剩余字符串是否为合法 JSON，如果是则解析成结构化的列/行数据，然后执行正常断言流程。非 JSON 格式的普通数据库错误（如语法错误、连接异常）不会受此影响，仍然正常报错。

### 带重试的异步测试

适合测试异步任务、消息队列消费结果等场景：

```yaml
cases:
  - name: "异步任务测试"
    desc: "验证消息队列消费后数据正确更新"
    steps:
      - name: "触发异步任务"
        query: "SELECT trigger_async_job('process_orders')"

      - name: "等待并验证结果"
        retry: 10        # 最多重试10次
        timeout: "30s"   # 超时30秒
        query: "SELECT status FROM orders WHERE id = 1"
        assertions:
          - type: equals
            column: "status"
            value: "completed"
```

### 嵌套用例

适用于按功能模块组织测试：

```yaml
cases:
  - name: "用户模块"
    cases:
      - name: "注册"
        steps:
          - name: "创建用户"
            query: "INSERT INTO users (name, email) VALUES ('Test', 'test@test.com') RETURNING id"
          - name: "验证创建"
            query: "SELECT count(*) AS cnt FROM users WHERE email = 'test@test.com'"
            assert: "cnt == 1"

      - name: "登录"
        steps:
          - name: "验证密码"
            query: "SELECT verify_password('test@test.com', 'password123') AS valid"
            assertions:
              - type: equals
                column: "valid"
                value: true
```

> 嵌套用例会自动展开为 `用户模块.注册` 和 `用户模块.登录`。

## 输出格式

### Console（默认）

```
  ✓ 用例名/步骤名 [15ms]
  ✗ 失败的步骤 [8ms]
    Error: equals: expected "Bob", got "Alice"
  ○ 跳过的用例 []
  ⚠ 错误的步骤 [3ms]
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

输出机器可读的 JSON，包含每个测试的详细信息：

```json
{
  "tests": [
    {
      "status": "PASSED",
      "name": "用例名/步骤名",
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

生成标准 JUnit XML 格式，可直接被 Jenkins、GitLab CI、GitHub Actions 等 CI 系统解析。

## CI/CD 集成

### GitHub Actions

项目包含 GitHub Actions 工作流，当推送 `v*` 标签时自动构建多平台二进制文件并发布 Release：

```bash
# 发布新版本
git tag v0.1.0
git push origin v0.1.0
```

构建产物：

| 平台     | 架构           |
|----------|----------------|
| Linux    | amd64, arm64   |
| Windows  | amd64, arm64   |
| macOS    | amd64, arm64   |

也可以手动触发构建：在 GitHub 上进入 Actions 标签页，选择 "Build and Release" → "Run workflow"。

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

### Docker Compose 本地运行

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

## 命令行参数

```
Usage: pgtest [options]

Options:
  -config string    测试配置文件路径 (default "test.yaml")
  -sql string       SQL命名查询文件路径 (default "queries.sql").
                    当显式指定 -config 而未指定 -sql 时，
                    自动从配置文件名推导 SQL 文件名（扩展名替换为 .sql）
  -format string    输出格式: console, json, junit (default "console")
  -verbose          启用详细输出，打印 DSN、连接状态、用例名称
                    注意：DSN 中的密码会自动脱敏 (default false)
  -name string      仅运行指定名称的测试用例，支持中文名和嵌套用例的 . 分隔格式
  -query string     仅运行 query 字段匹配指定值的测试步骤
  -notice           打印 PostgreSQL 存储过程中的 RAISE NOTICE 消息 (default false)
  -stop-on-error    遇到错误时停止执行后续测试用例 (default true)
  -version          打印版本信息并退出
  -help             显示帮助信息并退出
```

### 过滤与筛选

在开发过程中，经常只需要运行某一个用例或某一条查询。pgtest 提供了 `-name` 和 `-query` 两个筛选参数：

#### 按用例名称筛选 (`-name`)

指定 `-name` 后，仅运行名称匹配的测试用例。支持中文名和嵌套用例（`.` 分隔格式）：

```bash
# 运行名为 "用户查询测试" 的用例
pgtest -name "用户查询测试"

# 运行嵌套用例 "用户模块.注册"（配置中 name 为 "用户模块" 下的子用例 "注册"）
pgtest -name "用户模块.注册"

# 配合 verbose 查看筛选结果
pgtest -name "用户查询测试" -verbose
```

> 用例名找不到时会报错退出：`Error: no test case found with name "xxx"`

#### 按查询名称筛选 (`-query`)

指定 `-query` 后，仅运行 `query` 字段与指定值匹配的步骤；如果步骤未指定 `query` 字段，则使用步骤的 `name` 进行匹配。不匹配的步骤和用例会被自动过滤掉：

```bash
# 仅运行 query 为 "find-all-users" 的步骤（跨所有用例）
pgtest -query "find-all-users"

# 仅运行 query 为 "CALL proc_web_order_hall(...)" 的步骤
pgtest -query "CALL proc_web_order_hall(...)"

# 如果步骤 name 直接对应 dotsql 查询名（query 省略），同样可以匹配
pgtest -query "client_new"

# 配合 verbose
pgtest -query "find-all-users" -verbose
```

> 找不到匹配步骤时会报错退出：`Error: no steps found matching query "xxx"`

#### 组合使用

`-name` 和 `-query` 可以同时使用，先按用例名筛选，再按 query 名筛选步骤：

```bash
# 先定位到具体用例，再筛选其中的某条查询
pgtest -name "用户查询测试" -query "find-all-users"
```

### Verbose 调试模式

当测试配置不工作或连接不上数据库时，使用 `-verbose` 可以快速定位问题：

```bash
pgtest -config test.yaml -verbose
```

输出示例：

```
[verbose] config file: test.yaml
[verbose] sql file: queries.sql
[verbose] database driver: postgres
[verbose] database dsn: host=localhost port=5432 user=postgres dbname=postgres sslmode=disable
[verbose] connecting to database...
[verbose] database connection established
[verbose] running case: 用户查询测试
  ✓ 用户查询测试/查询所有用户 [12ms]
[verbose] running case: 边缘测试
  ✓ 边缘测试/空结果检查 [3ms]
[verbose] skipping case: 跳过的用例

━━━━━━━━━━━━━━━━━━━━━━━━━━
Results: 3 total, 2 passed, 0 failed, 1 skipped, 0 errors
Duration: 15ms
Status: PASSED
```

> **安全说明**：DSN 中的密码字段会被自动遮蔽为 `***`，不会在 verbose 输出中泄露敏感信息。

### Notice 调试模式

当一个查询执行了包含 `RAISE NOTICE` 语句的 PostgreSQL 存储过程或函数时，使用 `-notice` 可以将这些消息打印到 stderr，方便调试存储过程内部逻辑：

```bash
pgtest -config test.yaml -notice
```

输出示例：

```
[notice] 开始计算奖金，员工ID: 1, 销售额: 150000
[notice] 适用高业绩档位，奖金比率: 10%
  ✓ 奖金计算-高业绩/调用存储过程 [15ms]
```

```sql
-- 存储过程示例
CREATE OR REPLACE FUNCTION calculate_bonus(emp_id INT, sales DECIMAL)
RETURNS DECIMAL AS $$
BEGIN
    RAISE NOTICE '开始计算奖金，员工ID: %, 销售额: %', emp_id, sales;
    IF sales > 100000 THEN
        RAISE NOTICE '适用高业绩档位，奖金比率: 10%';
        RETURN sales * 0.10;
    END IF;
    ...
END;
$$ LANGUAGE plpgsql;
```

> **注意**：notice 消息输出到 stderr，不影响 stdout 的测试报告输出（console/json/junit）。

如果数据库连接失败，verbose 会明确指示失败步骤：

```
[verbose] database driver: postgres
[verbose] database dsn: host=localhost port=5432 user=postgres dbname=postgres sslmode=disable
[verbose] connecting to database...
[verbose] database ping failed: dial tcp 127.0.0.1:5432: connect: connection refused
Error creating runner: runner.New: ping db: dial tcp 127.0.0.1:5432: connect: connection refused
```

## 项目结构

```
pgtest/
├── main.go              # CLI 入口
├── config/
│   └── config.go        # YAML 配置解析、DSN 管理、用例扁平化
├── assert/
│   └── assert.go        # 断言引擎、类型转换、结果收集
├── runner/
│   ├── runner.go        # 测试执行器、变量替换、重试逻辑、JSON 解析
│   └── reporter.go      # 多格式报告输出（console/json/junit）
├── dotsql/
│   ├── dotsql.go        # 命名SQL查询加载器
│   └── scanner.go       # SQL 文件扫描与解析
├── go.mod
├── go.sum
└── test.yaml            # 示例配置
```

## 最佳实践

1. **SQL 与配置分离** — 将复杂 SQL 放在 `queries.sql` 中通过名称引用，保持 YAML 简洁
2. **配置文件与 SQL 同名** — 建议将 `.yaml` 配置文件与对应的 `.sql` 查询文件使用相同的基本文件名（如 `user_test.yaml` 和 `user_test.sql`），这样指定 `-config user_test.yaml` 时自动推导 SQL 文件路径，无需额外指定 `-sql` 参数
3. **每用例独立数据** — 使用 `setup`/`teardown` 为每个用例准备和清理数据，避免用例间相互影响
4. **合理使用重试** — 仅在异步或有最终一致性的场景使用重试，避免隐藏真实 bug
5. **覆盖边界条件** — 利用 `is_null`、`not_null`、`matches` 等断言覆盖 NULL 处理、格式验证等边界情况
6. **JUnit 集成** — 在 CI 中使用 `-format junit` 获得可视化的测试趋势图
7. **安全优先** — DSN 中的密码在 verbose 输出中自动脱敏；不要在 YAML 中硬编码密码，始终使用环境变量

## License

MIT License. 详见各子包的 LICENSE 文件。