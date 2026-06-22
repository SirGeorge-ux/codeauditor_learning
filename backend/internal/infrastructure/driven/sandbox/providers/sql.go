package providers

import "github.com/anomalyco/codeauditor/backend/internal/ports"

// SqlProvider audits SQL by executing it with sqlite3 against an in-memory
// database. alpine:latest does not ship sqlite, so the Docker sandbox installs
// it on the fly with apk inside a shell wrapper and reads the script mounted at
// /code/code.sql.
type SqlProvider struct{}

// NewSqlProvider returns a ports.LanguageProvider for SQL.
func NewSqlProvider() ports.LanguageProvider { return &SqlProvider{} }

func (p *SqlProvider) Language() string     { return "sql" }
func (p *SqlProvider) FileExtension() string { return ".sql" }
func (p *SqlProvider) DockerImage() string    { return "alpine:latest" }

// DockerCommand returns the argv appended after the image in `docker run`:
//
//	sh -c "apk add --no-cache sqlite && sqlite3 :memory: '.read /code/code.sql'"
//
// The in-memory database (:memory:) guarantees the schema does not persist
// after execution — each run is isolated, matching the SQL execution audit
// scenario.
func (p *SqlProvider) DockerCommand(_ string) []string {
	return []string{"sh", "-c", "apk add --no-cache sqlite && sqlite3 :memory: '.read /code/code.sql'"}
}

// LocalCommand returns the executable run by LocalSandbox ("sqlite3").
func (p *SqlProvider) LocalCommand() string { return "sqlite3" }

// InstallHint is shown when the local executable is missing.
func (p *SqlProvider) InstallHint() string {
	return "Install sqlite3: https://www.sqlite.org/download.html or `apk add sqlite` / `apt install sqlite3`"
}

// Compile-time guard.
var _ ports.LanguageProvider = (*SqlProvider)(nil)