# Delta Spec: Web+SQL Audit

## Requirement: Web+SQL Language Audit

The audit system MUST support code auditing for HTML, CSS, XML, JSON, YAML, and SQL. Execution MUST correctly reflect the markup or validation output without errors for valid syntax, and MUST capture standard error and non-zero exit codes for invalid syntax in JSON, YAML, and SQL.

### Scenario: JSON syntax error
- GIVEN a JSON snippet with a missing bracket
- WHEN the JSON sandbox executes it
- THEN it MUST return an error payload
- AND stderr MUST contain a `jq` parse error

### Scenario: SQL execution
- GIVEN a SQL snippet with `CREATE TABLE` and `INSERT` statements
- WHEN the SQL sandbox executes it
- THEN it MUST complete successfully
- AND it MUST NOT persist the schema after execution (in-memory only)

### Scenario: HTML markup echo
- GIVEN an HTML snippet
- WHEN the HTML sandbox executes it
- THEN it MUST complete successfully
- AND stdout MUST match the input snippet exactly
