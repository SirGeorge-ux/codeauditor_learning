package llmcascade

import (
	"testing"

	"github.com/anomalyco/codeauditor/backend/internal/ports"
)

// TestDeepSeek_NameCanonical — el nombre canónico usado en
// Cascade.providerOrder.
func TestDeepSeek_NameCanonical(t *testing.T) {
	d := NewDeepSeek("https://openrouter.ai/api/v1", "key", "model")
	if d.Name() != "deepseek" {
		t.Errorf("Name() = %q, want deepseek", d.Name())
	}
}

// TestDeepSeek_EmptyKeyMarksDisabled — un api key vacío marca al
// provider como disabled.
func TestDeepSeek_EmptyKeyMarksDisabled(t *testing.T) {
	d := NewDeepSeek("https://openrouter.ai/api/v1", "", "model")
	if !d.IsDisabled() {
		t.Errorf("empty key should mark deepseek disabled")
	}
}

// TestDeepSeek_BuildChatCompletionURL sanity check.
func TestDeepSeek_BuildChatCompletionURL(t *testing.T) {
	d := NewDeepSeek("https://openrouter.ai/api/v1/", "k", "m")
	got := d.chatCompletionURL()
	if got != "https://openrouter.ai/api/v1/chat/completions" {
		t.Errorf("URL = %q, want .../chat/completions", got)
	}
}

// TestDeepSeek_ImplementsProviderClient — compile-time guard.
func TestDeepSeek_ImplementsProviderClient(t *testing.T) {
	var _ providerClient = (*DeepSeek)(nil)
}

// TestMiniMax_NameCanonical — el nombre canónico usado por el
// frontend cuando el usuario flagea use_minimax.
func TestMiniMax_NameCanonical(t *testing.T) {
	m := NewMiniMax("https://api.MiniMax.ai/v1", "key", "MiniMax-M3")
	if m.Name() != "minimax" {
		t.Errorf("Name() = %q, want minimax", m.Name())
	}
}

// TestMiniMax_EmptyKeyMarksDisabled — sin API key el provider queda
// disabled en la cascada.
func TestMiniMax_EmptyKeyMarksDisabled(t *testing.T) {
	m := NewMiniMax("https://api.MiniMax.ai/v1", "", "")
	if !m.IsDisabled() {
		t.Errorf("empty key should mark MiniMax disabled")
	}
}

// TestMiniMax_BuildChatCompletionURL sanity check.
func TestMiniMax_BuildChatCompletionURL(t *testing.T) {
	m := NewMiniMax("https://api.MiniMax.ai/v1/", "k", "MiniMax-M3")
	got := m.chatCompletionURL()
	if got != "https://api.MiniMax.ai/v1/chat/completions" {
		t.Errorf("URL = %q, want .../chat/completions", got)
	}
}

// TestMiniMax_ImplementsProviderClient — compile-time guard. Un
// drift de firma entre las 3 implementaciones se detecta aquí.
func TestMiniMax_ImplementsProviderClient(t *testing.T) {
	var _ providerClient = (*MiniMax)(nil)
	_ = ports.StreamChatOptions{}
}

// TestProvidersImplementsPortsLLMClientPort — el cascade en su
// conjunto debe satisfacer el port de aplicación.
func TestProvidersImplementsPortsLLMClientPort(t *testing.T) {
	var _ ports.LLMClientPort = (*Cascade)(nil)
}
