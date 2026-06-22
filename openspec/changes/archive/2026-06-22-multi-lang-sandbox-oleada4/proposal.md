# Proposal: multi-lang-sandbox-oleada4

## Intent
Expand the sandbox from 17 to 23 languages by adding the Web+SQL ecosystem: HTML, CSS, XML, JSON, YAML, and SQL. 

## Scope
**In scope:**
- Implement `LanguageProvider` for html, css, xml, json, yaml, sql.
- Register the 6 new providers in `NewDefaultRegistry()`.
- Update `registry_test.go` (17 → 23 languages).
- Update spec for sandbox-provider-registry.

**Out of scope:**
- Other language ecosystems.
- Heavy linters or NodeJS-based formatters (e.g. Prettier, ESLint).

## Approach
Reuse the established Provider Pattern. One file per language under `providers`. Since these languages are declarative/markup/data formats rather than executable logic, we will use `alpine:latest` for all of them to ensure fast execution.

### Execution Strategy
- **HTML, CSS, XML**: Command `cat /code/<file>`. Simply echoes the content back to satisfy the sandbox execution flow.
- **JSON**: Wrapper `sh -c "apk add --no-cache jq && jq . /code/code.json"`. Validates and pretty-prints JSON.
- **YAML**: Wrapper `sh -c "apk add --no-cache yq && yq . /code/code.yaml"`. Validates YAML syntax.
- **SQL**: Wrapper `sh -c "apk add --no-cache sqlite && sqlite3 :memory: '.read /code/code.sql'"`. Validates SQL syntax against SQLite.

## Risks
- SQL validation is SQLite-specific. If the user provides Postgres or MySQL specific syntax, it might report a syntax error. This is acceptable for a sandbox environment.
- `yq` package in alpine uses the `mikefarah/yq` Go implementation, which is highly compatible.

## Rollback Plan
Remove the 6 new files and revert the `NewDefaultRegistry` additions. No state or external systems are modified.