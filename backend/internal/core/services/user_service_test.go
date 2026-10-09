package services

// Tests for UserService. PR 3 ships the minimal UserService that backs
// mcp-user.get_profile; the rest of the mcp-user tools (get_language_progress,
// get_history, get_learning_profile, record_audit_attempt,
// update_learning_profile) wrap existing services via the mcp/user
// production adapter — they are exercised at the tool / transport layer,
// not in service-level tests.

import (
	"context"
	"errors"
	"testing"

	"github.com/anomalyco/codeauditor/backend/internal/core/domain/models"
	"github.com/anomalyco/codeauditor/backend/internal/ports"
)

// fakeUserRepo is an in-memory UserProfileRepository. Tests are sequential,
// not concurrent — the test harness enforces that.
type fakeUserRepo struct {
	profiles map[string]*models.UserProfile
	err      error
	notFound bool
}

func (f *fakeUserRepo) GetProfile(_ context.Context, userID string) (*models.UserProfile, error) {
	if f.err != nil {
		return nil, f.err
	}
	if f.notFound {
		return nil, ports.ErrUserProfileNotFound
	}
	p, ok := f.profiles[userID]
	if !ok {
		return nil, ports.ErrUserProfileNotFound
	}
	cp := *p
	return &cp, nil
}

func TestUserService_GetProfile_HappyPath(t *testing.T) {
	repo := &fakeUserRepo{profiles: map[string]*models.UserProfile{
		"alice": {ID: "alice", Email: "alice@example.com", DisplayName: "Alice", RachaDias: 7, PuntosMaestria: 250, RangoActual: "C"},
	}}
	svc := NewUserService(repo)
	got, err := svc.GetProfile(context.Background(), "alice")
	if err != nil {
		t.Fatalf("GetProfile: %v", err)
	}
	if got.ID != "alice" || got.Email != "alice@example.com" {
		t.Errorf("got = %+v, want id=alice email=alice@example.com", got)
	}
	if got.RangoActual != "C" {
		t.Errorf("RangoActual = %q, want C", got.RangoActual)
	}
}

func TestUserService_GetProfile_NotFound_ReturnsSentinel(t *testing.T) {
	repo := &fakeUserRepo{profiles: map[string]*models.UserProfile{}}
	svc := NewUserService(repo)
	_, err := svc.GetProfile(context.Background(), "ghost")
	if err == nil {
		t.Fatal("GetProfile(ghost) returned nil err; want ErrUserProfileNotFound")
	}
	if !errors.Is(err, ports.ErrUserProfileNotFound) {
		t.Errorf("err = %v, want ErrUserProfileNotFound", err)
	}
}

func TestUserService_GetProfile_EmptyUserID_ReturnsInvalidInput(t *testing.T) {
	repo := &fakeUserRepo{profiles: map[string]*models.UserProfile{}}
	svc := NewUserService(repo)
	_, err := svc.GetProfile(context.Background(), "")
	if err == nil {
		t.Fatal("GetProfile(empty) returned nil err; want ErrUserValidation")
	}
	if !errors.Is(err, ErrUserValidation) {
		t.Errorf("err = %v, want ErrUserValidation", err)
	}
}
