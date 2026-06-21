package providers

import "github.com/anomalyco/codeauditor/backend/internal/ports"

// LuaProvider audits Lua code with luacheck, using the
// `nickblah/luacheck:latest` image for Docker execution and the `luacheck`
// binary for the local healthcheck.
type LuaProvider struct{}

// NewLuaProvider returns a ports.LanguageProvider for Lua.
func NewLuaProvider() ports.LanguageProvider { return &LuaProvider{} }

func (p *LuaProvider) Language() string      { return "lua" }
func (p *LuaProvider) FileExtension() string  { return ".lua" }
func (p *LuaProvider) DockerImage() string    { return "nickblah/luacheck:latest" }

// DockerCommand returns the argv appended after the image in `docker run`:
//   luacheck <filename>
func (p *LuaProvider) DockerCommand(filename string) []string {
	return []string{"luacheck", filename}
}

// LocalCommand returns the executable run by LocalSandbox ("luacheck").
func (p *LuaProvider) LocalCommand() string { return "luacheck" }

// InstallHint is shown when the local executable is missing.
func (p *LuaProvider) InstallHint() string {
	return "luarocks install luacheck"
}

// Compile-time guard.
var _ ports.LanguageProvider = (*LuaProvider)(nil)