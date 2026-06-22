# Design: multi-lang-sandbox-oleada4

## Technical Approach
Expand the sandbox support from 17 to 23 languages by adding the Web+SQL ecosystem: HTML, CSS, XML, JSON, YAML, and SQL. 

We will strictly follow the established Provider Pattern by creating one new file per language in the `providers` package. All 6 providers will use `alpine:latest` as their Docker image. 

HTML, CSS, and XML are markup/style languages that don't "execute" or have a standard lightweight linter pre-installed on alpine. Their `DockerCommand` will simply `cat` the file back.

JSON, YAML, and SQL will use a shell wrapper (`sh -c`) to install lightweight CLI tools (`jq`, `yq`, `sqlite3` respectively) to perform basic syntax validation and execution of the provided code snippet within the sandboxed `/tmp` directory or in-memory DB.

## File Changes

| File | Action | Description |
|------|--------|-------------|
| `providers/html.go` | Create | HTML provider (uses `cat`) |
| `providers/html_test.go` | Create | Tests for HTML provider |
| `providers/css.go` | Create | CSS provider (uses `cat`) |
| `providers/css_test.go` | Create | Tests for CSS provider |
| `providers/xml.go` | Create | XML provider (uses `cat`) |
| `providers/xml_test.go` | Create | Tests for XML provider |
| `providers/json.go` | Create | JSON provider (uses `jq`) |
| `providers/json_test.go` | Create | Tests for JSON provider |
| `providers/yaml.go` | Create | YAML provider (uses `yq`) |
| `providers/yaml_test.go` | Create | Tests for YAML provider |
| `providers/sql.go` | Create | SQL provider (uses `sqlite3`) |
| `providers/sql_test.go` | Create | Tests for SQL provider |
| `providers/registry.go` | Modify | Register all 6 new providers |
| `providers/registry_test.go` | Modify | Update sorted keys array (17 → 23) |

*Note: Gogs Handler (`gogs_handler.go`) already maps these file extensions correctly, so no changes are needed there.*
