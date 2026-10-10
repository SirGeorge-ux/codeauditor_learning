package driven

import (
	"context"
	"strconv"
	"sync"
	"testing"

	"github.com/anomalyco/codeauditor/backend/internal/core/domain/models"
)

// makeMsg es un constructor de ChatMessage de prueba. Centralizar el
// shape aqui evita que cada caso repita el struct literal.
func makeMsg(role models.ChatRole, content string) models.ChatMessage {
	return models.ChatMessage{
		Role:    role,
		Content: content,
	}
}

// TestSessionCache_SetGetRoundtrip cubre el camino feliz: Set sobre
// una sessionID devuleve ok=true con el mismo payload. La tupla
// (value, ok) sigue el patron idiomático del stdlib (sync.Map.Load,
// map[string]T lookup) para que un lector Go reconozca el contrato al
// instante.
func TestSessionCache_SetGetRoundtrip(t *testing.T) {
	cache := NewSessionCache()
	ctx := context.Background()
	cache.Set(ctx, "sess-1", makeMsg(models.ChatRoleUser, "hi"))
	got, ok := cache.Get(ctx, "sess-1")
	if !ok {
		t.Fatalf("Get(sess-1) returned ok=false, want true")
	}
	if got.Content != "hi" {
		t.Errorf("Get content = %q, want %q", got.Content, "hi")
	}
}

// TestSessionCache_GetUnknownReturnsZeroValue garantiza que la tupla
// (zero, false) es la respuesta para keys desconocidas, sin panic.
// El zero value de ChatMessage es un struct vacio JSON-safe.
func TestSessionCache_GetUnknownReturnsZeroValue(t *testing.T) {
	cache := NewSessionCache()
	got, ok := cache.Get(context.Background(), "nope")
	if ok {
		t.Errorf("Get(unknown) returned ok=true, want false")
	}
	if got.Content != "" {
		t.Errorf("Get(unknown) content = %q, want empty", got.Content)
	}
}

// TestSessionCache_RingBufferCap cover el contrato central del cache:
// un max de 10 mensajes por sessionID; el mensaje mas viejo se
// descarta en el set numero 11. El spec define "rolling 10-message
// window" como el requisito (spec.md#Session-Memory "Window rolls
// over at 10 messages").
func TestSessionCache_RingBufferCap(t *testing.T) {
	cache := NewSessionCache()
	ctx := context.Background()
	for i := 0; i < 15; i++ {
		cache.Set(ctx, "sess-1", makeMsg(models.ChatRoleUser, msgContent(i)))
	}
	snap := cache.Snapshot(ctx, "sess-1")
	if len(snap) != 10 {
		t.Fatalf("Snapshot len = %d, want 10 (ring buffer cap)", len(snap))
	}
	// El primer mensaje ("m-0") debio ser evictado; el primer
	// sobreviviente es "m-5" (los indices 5..14 sobreviven).
	if snap[0].Content != "m-5" {
		t.Errorf("oldest survivor = %q, want m-5", snap[0].Content)
	}
	if snap[9].Content != "m-14" {
		t.Errorf("newest = %q, want m-14", snap[9].Content)
	}
}

// TestSessionCache_SnapshotIsInsertionOrder garantiza que el snapshot
// itera del mas viejo al mas nuevo (orden cronologico). El LLM y los
// tests downstream asumen este orden; invertirlo rompe la coherencia
// del historial de la charla.
func TestSessionCache_SnapshotIsInsertionOrder(t *testing.T) {
	cache := NewSessionCache()
	ctx := context.Background()
	cache.Set(ctx, "sess-1", makeMsg(models.ChatRoleUser, "first"))
	cache.Set(ctx, "sess-1", makeMsg(models.ChatRoleTutor, "second"))
	cache.Set(ctx, "sess-1", makeMsg(models.ChatRoleUser, "third"))
	snap := cache.Snapshot(ctx, "sess-1")
	want := []string{"first", "second", "third"}
	for i, w := range want {
		if snap[i].Content != w {
			t.Errorf("snap[%d] = %q, want %q", i, snap[i].Content, w)
		}
	}
}

// TestSessionCache_SnapshotUnknownSessionReturnsEmptySlice confirma
// que pedir el snapshot de una session desconocida devuelve un slice
// vacio (no nil), para que el codigo downstream pueda iterarlo
// sin chequeos adicionales.
func TestSessionCache_SnapshotUnknownSessionReturnsEmptySlice(t *testing.T) {
	cache := NewSessionCache()
	snap := cache.Snapshot(context.Background(), "nope")
	if snap == nil {
		t.Errorf("Snapshot returned nil; want empty slice for stable iteration")
	}
	if len(snap) != 0 {
		t.Errorf("Snapshot len = %d, want 0", len(snap))
	}
}

// TestSessionCache_KeysEnumeratesSessions cubre la API administrativa
// del cache: enumerar las sessionIDs activas. El handler /tutor/admin
// lo usa para inspeccionar sesiones vivas.
func TestSessionCache_KeysEnumeratesSessions(t *testing.T) {
	cache := NewSessionCache()
	ctx := context.Background()
	cache.Set(ctx, "sess-A", makeMsg(models.ChatRoleUser, "a"))
	cache.Set(ctx, "sess-B", makeMsg(models.ChatRoleUser, "b"))
	cache.Set(ctx, "sess-C", makeMsg(models.ChatRoleUser, "c"))
	keys := cache.Keys(ctx)
	if len(keys) != 3 {
		t.Fatalf("Keys len = %d, want 3", len(keys))
	}
	want := map[string]bool{"sess-A": true, "sess-B": true, "sess-C": true}
	for _, k := range keys {
		if !want[k] {
			t.Errorf("Keys contained unexpected %q", k)
		}
	}
}

// TestSessionCache_ResetClearsSession verifica que Reset(sessionID)
// elimina la session del cache y libera memoria. Es idempotente en
// sessionIDs desconocidas.
func TestSessionCache_ResetClearsSession(t *testing.T) {
	cache := NewSessionCache()
	ctx := context.Background()
	cache.Set(ctx, "sess-1", makeMsg(models.ChatRoleUser, "hi"))
	cache.Reset(ctx, "sess-1")
	if _, ok := cache.Get(ctx, "sess-1"); ok {
		t.Errorf("after Reset, Get returned ok=true")
	}
	// Idempotente: resetear una session desconocida no debe panic.
	cache.Reset(ctx, "never-existed")
}

// TestSessionCache_KeysAfterResetOmitsClearedSession garantiza que
// Reset() tambien limpia la enumeration de Keys(), no solo Get(). Un
// /tutor/admin que no vea Keys() actualizados filtraria memoria.
func TestSessionCache_KeysAfterResetOmitsClearedSession(t *testing.T) {
	cache := NewSessionCache()
	ctx := context.Background()
	cache.Set(ctx, "sess-A", makeMsg(models.ChatRoleUser, "a"))
	cache.Set(ctx, "sess-B", makeMsg(models.ChatRoleUser, "b"))
	cache.Reset(ctx, "sess-A")
	keys := cache.Keys(ctx)
	if len(keys) != 1 || keys[0] != "sess-B" {
		t.Errorf("Keys after Reset = %v, want [sess-B]", keys)
	}
}

// TestSessionCache_MultipleSessionsIsolated cubre el caso "two
// challenges, two windows". Cada session mantiene su propio ring
// buffer; un Set en sess-A no afecta la longitud de sess-B.
func TestSessionCache_MultipleSessionsIsolated(t *testing.T) {
	cache := NewSessionCache()
	ctx := context.Background()
	for i := 0; i < 5; i++ {
		cache.Set(ctx, "sess-A", makeMsg(models.ChatRoleUser, msgContent(i)))
	}
	for i := 0; i < 3; i++ {
		cache.Set(ctx, "sess-B", makeMsg(models.ChatRoleUser, msgContent(i+100)))
	}
	snapA := cache.Snapshot(ctx, "sess-A")
	snapB := cache.Snapshot(ctx, "sess-B")
	if len(snapA) != 5 {
		t.Errorf("snapA len = %d, want 5", len(snapA))
	}
	if len(snapB) != 3 {
		t.Errorf("snapB len = %d, want 3", len(snapB))
	}
	if snapB[0].Content != "m-100" {
		t.Errorf("snapB[0] = %q, want m-100 (isolation broken)", snapB[0].Content)
	}
}

// TestSessionCache_SatisfiesCachePort es el compile-time check que la
// concrecion satisface el puerto. Si la firma del puerto cambia,
// este test rompe antes de que un adapter adopte la nueva firma
// silenciosamente.
func TestSessionCache_SatisfiesCachePort(t *testing.T) {
	var _ interface {
		Get(context.Context, string) (models.ChatMessage, bool)
		Set(context.Context, string, models.ChatMessage)
		Snapshot(context.Context, string) []models.ChatMessage
		Keys(context.Context) []string
		Reset(context.Context, string)
	} = NewSessionCache()
}

// TestSessionCache_ConcurrentWritesAreSafe lanza N goroutines que
// escriben concurrentemente. El cache debe sobrevivir sin data race
// ni corruption interna. El race detector es -race-only; en este host
// (sin gcc) el test corre igual y verifica solo correctness, no data
// races — PR 2 re-ejecuta este test en CI con -race.
func TestSessionCache_ConcurrentWritesAreSafe(t *testing.T) {
	cache := NewSessionCache()
	ctx := context.Background()
	const goroutines = 50
	const writesPer = 20
	var wg sync.WaitGroup
	wg.Add(goroutines)
	for g := 0; g < goroutines; g++ {
		go func(gid int) {
			defer wg.Done()
			for w := 0; w < writesPer; w++ {
				sessionID := "sess-shared"
				content := "g" + strconv.Itoa(gid) + "-w" + strconv.Itoa(w)
				cache.Set(ctx, sessionID, makeMsg(models.ChatRoleUser, content))
				_, _ = cache.Get(ctx, sessionID)
				_ = cache.Snapshot(ctx, sessionID)
			}
		}(g)
	}
	wg.Wait()
	snap := cache.Snapshot(ctx, "sess-shared")
	if len(snap) == 0 || len(snap) > 10 {
		t.Errorf("after concurrent writes, snap len = %d, want in (0, 10]", len(snap))
	}
	// Snapshot len esta en el ring buffer; clave: cero perdidas.
	if len(cache.Keys(ctx)) == 0 {
		t.Errorf("Keys empty after concurrent writes")
	}
}

// msgContent genera un string estable a partir de un indice. Util
// para los loops del test de ring buffer. Usa strconv.Itoa para
// soportar indices arbitrarios.
func msgContent(i int) string {
	return "m-" + strconv.Itoa(i)
}
