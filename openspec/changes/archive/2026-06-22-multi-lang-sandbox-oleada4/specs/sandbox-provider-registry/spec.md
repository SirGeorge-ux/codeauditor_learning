# Delta Spec: Web+SQL Sandbox Providers

## Requirement: Web+SQL Language Providers

The system MUST implement `LanguageProvider` for HTML, CSS, XML, JSON, YAML, and SQL. All 6 MUST use the `alpine:latest` Docker image. HTML, CSS, and XML MUST use `cat /code/<filename>` for their execution command. JSON MUST use a shell wrapper to install and run `jq`. YAML MUST use a shell wrapper to install and run `yq`. SQL MUST use a shell wrapper to install and run `sqlite3` using an in-memory database to execute the SQL script.

### Scenario: Markup and Styles execution
- GIVEN the HTML, CSS, or XML provider
- WHEN `DockerCommand("code.ext")` is called
- THEN it MUST return a command that executes `cat /code/code.ext`

### Scenario: JSON validation
- GIVEN the JSON provider
- WHEN `DockerCommand("code.json")` is called
- THEN it MUST return a command that executes `sh -c "apk add --no-cache jq && jq . /code/code.json"`

### Scenario: YAML validation
- GIVEN the YAML provider
- WHEN `DockerCommand("code.yaml")` is called
- THEN it MUST return a command that executes `sh -c "apk add --no-cache yq && yq . /code/code.yaml"`

### Scenario: SQL execution
- GIVEN the SQL provider
- WHEN `DockerCommand("code.sql")` is called
- THEN it MUST return a command that executes `sh -c "apk add --no-cache sqlite && sqlite3 :memory: '.read /code/code.sql'"`

## Requirement: Registry Registration (Oleada 4)

The `NewDefaultRegistry()` function MUST register the 6 new providers, bringing the total supported languages to 23. `Languages()` MUST return the 23 keys in alphabetical order.

### Scenario: 23 Languages registered
- GIVEN a default `ProviderRegistry`
- WHEN `Languages()` is called
- THEN it MUST return exactly 23 items
- AND the list MUST include "html", "css", "xml", "json", "yaml", and "sql"

### Scenario: Unknown language testing
- GIVEN a default `ProviderRegistry`
- WHEN `Get("cobol")` is called
- THEN it MUST return an unsupported language error
