# Exploration: multi-lang-sandbox-oleada4

## Current State
The sandbox uses a `ProviderRegistry` with **17 providers**. The `gogs_handler.go` already maps `.html`, `.css`, `.xml`, `.json`, `.yaml`/`.yml`, and `.sql` to their respective language keys.

## Affected Areas
- `backend/internal/infrastructure/driven/sandbox/providers/registry.go` — register 6 new providers
- `backend/internal/infrastructure/driven/sandbox/providers/html.go`
- `backend/internal/infrastructure/driven/sandbox/providers/css.go`
- `backend/internal/infrastructure/driven/sandbox/providers/xml.go`
- `backend/internal/infrastructure/driven/sandbox/providers/json.go`
- `backend/internal/infrastructure/driven/sandbox/providers/yaml.go`
- `backend/internal/infrastructure/driven/sandbox/providers/sql.go`
- `backend/internal/infrastructure/driven/sandbox/providers/registry_test.go` — update expected count 17 → 23

## Approaches
For languages that are not "executed" in a traditional sense (markup, styles, config), we have a few options to fulfill the CodeAuditor sandbox contract:

1. **Validation / Linter via Shell Wrappers**
   - JSON: `sh -c "apk add --no-cache jq && jq . /code/code.json"` (validates and pretty-prints)
   - SQL: `sh -c "apk add --no-cache sqlite && sqlite3 :memory: '.read /code/code.sql'"` (validates SQL syntax)
   - YAML: `sh -c "apk add --no-cache yq && yq . /code/code.yaml"` (validates YAML)
   - HTML/CSS/XML: Just use `cat /code/code.ext` since `alpine` doesn't have built-in minimal linters for these without pulling heavy Node.js images.

2. **Full Tooling Images**
   - Use `node:alpine` and run `npx prettier` for everything.
   - Pros: Standardized output.
   - Cons: Pulls a massive image just to read a file, execution is much slower.

### Recommendation
**Viable — proceed with Approach 1.** 
Use `alpine:latest` for all 6 providers. For JSON, SQL, and YAML, we can add basic syntax validation using `jq`, `sqlite`, and `yq` respectively via `sh -c "apk add..."` wrappers (similar to Zig). For HTML, CSS, and XML, a simple `cat /code/<file>` is sufficient to return the code and satisfy the execution flow without breaking the architecture.

### Risks
- SQL scripts with SQLite-incompatible syntax (e.g. Postgres specific) might fail the syntax check. This is acceptable for a sandbox environment.
- No significant risks identified.

### Ready for Proposal
**Yes.** The orchestrator can proceed to proposal. Total effort is low.