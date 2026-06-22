package providers

import (
	"fmt"
	"sort"

	"github.com/anomalyco/codeauditor/backend/internal/ports"
)

// ProviderRegistry maps language keys to their LanguageProvider implementations.
// It replaces the inline `switch language` blocks in the sandbox adapters with a
// single registry lookup, enabling new languages to be added without modifying
// sandbox code.
type ProviderRegistry struct {
	providers map[string]ports.LanguageProvider
}

// NewProviderRegistry returns an empty registry ready to be populated.
func NewProviderRegistry() *ProviderRegistry {
	return &ProviderRegistry{providers: make(map[string]ports.LanguageProvider)}
}

// NewDefaultRegistry returns a registry populated with every provider
// available: TypeScript, JavaScript, Go, the six scripting languages (Python,
// Ruby, PHP, Lua, Bash, Perl), the four JVM languages (Java, Kotlin, Scala,
// Groovy), the four systems languages (Rust, C, C++, Zig), the six Web+SQL
// languages added in Oleada 4 (HTML, CSS, XML, JSON, YAML, SQL), and the six
// functional/.NET/data/Apple languages added in Oleada 5 (C#, Swift, Haskell,
// Elixir, Clojure, R). Future oleadas register additional providers here.
func NewDefaultRegistry() *ProviderRegistry {
	r := NewProviderRegistry()
	// Existing providers extracted from the original switch (Phase 2).
	_ = r.Register(NewTypeScriptProvider())
	_ = r.Register(NewJavaScriptProvider())
	_ = r.Register(NewGoProvider())
	// New providers added in Phase 3 — one file per language.
	_ = r.Register(NewPythonProvider())
	_ = r.Register(NewRubyProvider())
	_ = r.Register(NewPhpProvider())
	_ = r.Register(NewLuaProvider())
	_ = r.Register(NewBashProvider())
	_ = r.Register(NewPerlProvider())
	// JVM providers added in Oleada 2 — one file per language.
	_ = r.Register(NewJavaProvider())
	_ = r.Register(NewKotlinProvider())
	_ = r.Register(NewScalaProvider())
	_ = r.Register(NewGroovyProvider())
	// Systems providers added in Oleada 3 — one file per language.
	_ = r.Register(NewRustProvider())
	_ = r.Register(NewCProvider())
	_ = r.Register(NewCppProvider())
	_ = r.Register(NewZigProvider())
	// Web+SQL providers added in Oleada 4 — one file per language.
	_ = r.Register(NewHtmlProvider())
	_ = r.Register(NewCssProvider())
	_ = r.Register(NewXmlProvider())
	_ = r.Register(NewJsonProvider())
	_ = r.Register(NewYamlProvider())
	_ = r.Register(NewSqlProvider())
	// Functional/.NET/Data/Apple providers added in Oleada 5 — one file per language.
	_ = r.Register(NewCSharpProvider())
	_ = r.Register(NewSwiftProvider())
	_ = r.Register(NewHaskellProvider())
	_ = r.Register(NewElixirProvider())
	_ = r.Register(NewClojureProvider())
	_ = r.Register(NewRProvider())
	return r
}

// Register adds a provider to the registry, keyed by provider.Language().
// A nil provider is rejected. Registering a language that already exists
// overwrites the previous provider (last-write-wins) so that later oleadas can
// replace an implementation without touching earlier wiring.
func (r *ProviderRegistry) Register(p ports.LanguageProvider) error {
	if p == nil {
		return fmt.Errorf("cannot register a nil provider")
	}
	r.providers[p.Language()] = p
	return nil
}

// Get returns the provider registered for the given language key, or an error
// when the language is unknown to this registry.
func (r *ProviderRegistry) Get(lang string) (ports.LanguageProvider, error) {
	p, ok := r.providers[lang]
	if !ok {
		return nil, fmt.Errorf("unsupported language: %s", lang)
	}
	return p, nil
}

// Languages returns the sorted list of registered language keys.
func (r *ProviderRegistry) Languages() []string {
	keys := make([]string, 0, len(r.providers))
	for k := range r.providers {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
