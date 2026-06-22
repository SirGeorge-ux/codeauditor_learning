# Tasks: multi-lang-sandbox-oleada4

## Review Workload Forecast

| Field | Value |
|-------|-------|
| Estimated changed lines | ~450 (6 providers * ~75 lines) |
| 400-line budget risk | High |
| Chained PRs recommended | No (mechanical boilerplate, single-pr acceptable with exception) |
| Suggested split | Single PR |
| Delivery strategy | single-pr-default |
| Chain strategy | pending |

*Exception approved for >400 lines: changes are purely additive mechanical boilerplate following existing patterns.*

## Phase 1: Web Markup Providers

- [x] 1.1 Create `html.go` and `html_test.go` — `cat /code/code.html`, alpine:latest
- [x] 1.2 Create `css.go` and `css_test.go` — `cat /code/code.css`, alpine:latest
- [x] 1.3 Create `xml.go` and `xml_test.go` — `cat /code/code.xml`, alpine:latest

## Phase 2: Data & Config Providers

- [x] 2.1 Create `json.go` and `json_test.go` — `sh -c` with `apk add --no-cache jq && jq . /code/code.json`, alpine:latest
- [x] 2.2 Create `yaml.go` and `yaml_test.go` — `sh -c` with `apk add --no-cache yq && yq . /code/code.yaml`, alpine:latest

## Phase 3: SQL Provider

- [x] 3.1 Create `sql.go` and `sql_test.go` — `sh -c` with `apk add --no-cache sqlite && sqlite3 :memory: '.read /code/code.sql'`, alpine:latest

## Phase 4: Registry Integration

- [ ] 4.1 Update `registry.go` — Register HTML, CSS, XML, JSON, YAML, SQL in `NewDefaultRegistry()`
- [ ] 4.2 Update `registry_test.go` — Assert 23 languages sorted, update `UnknownKey` test to use `"cobol"` instead of "rust" (which is now registered).