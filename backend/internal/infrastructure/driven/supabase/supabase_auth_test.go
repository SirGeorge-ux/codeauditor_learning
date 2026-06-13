package supabase

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const testSecret = "test-jwt-secret-for-unit-tests"

func newTestAdapter() *SupabaseAuthAdapter {
	return NewSupabaseAuthAdapter(testSecret)
}

func makeToken(secret string, claims jwt.MapClaims, method jwt.SigningMethod) string {
	t := jwt.NewWithClaims(method, claims)
	s, _ := t.SignedString([]byte(secret))
	return s
}

func TestValidateToken_EmptyToken(t *testing.T) {
	a := newTestAdapter()
	err := a.ValidateToken(context.Background(), "")
	if err == nil {
		t.Fatal("expected error for empty token")
	}
	if !strings.Contains(err.Error(), "empty") {
		t.Errorf("expected 'empty' in error, got: %v", err)
	}
}

func TestValidateToken_BearerPrefix(t *testing.T) {
	a := newTestAdapter()
	token := makeToken(testSecret, jwt.MapClaims{
		"sub": "user-1",
		"exp": time.Now().Add(time.Hour).Unix(),
	}, jwt.SigningMethodHS256)

	// Token with "Bearer " prefix should work
	err := a.ValidateToken(context.Background(), "Bearer "+token)
	if err != nil {
		t.Fatalf("expected no error for Bearer-prefixed token, got: %v", err)
	}

	// Token without prefix should also work
	err = a.ValidateToken(context.Background(), token)
	if err != nil {
		t.Fatalf("expected no error for bare token, got: %v", err)
	}
}

func TestValidateToken_ValidHS256(t *testing.T) {
	a := newTestAdapter()
	token := makeToken(testSecret, jwt.MapClaims{
		"sub": "user-1",
		"exp": time.Now().Add(time.Hour).Unix(),
	}, jwt.SigningMethodHS256)

	err := a.ValidateToken(context.Background(), token)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
}

func TestValidateToken_WrongSecret(t *testing.T) {
	a := newTestAdapter()
	token := makeToken("wrong-secret", jwt.MapClaims{
		"sub": "user-1",
		"exp": time.Now().Add(time.Hour).Unix(),
	}, jwt.SigningMethodHS256)

	err := a.ValidateToken(context.Background(), token)
	if err == nil {
		t.Fatal("expected error for token signed with wrong secret")
	}
}

func TestValidateToken_WrongAlgorithm(t *testing.T) {
	a := newTestAdapter()
	// HS384 is not accepted — only HS256 (both are HMAC, but different algorithms)
	token := makeToken(testSecret, jwt.MapClaims{
		"sub": "user-1",
		"exp": time.Now().Add(time.Hour).Unix(),
	}, jwt.SigningMethodHS384)

	err := a.ValidateToken(context.Background(), token)
	if err == nil {
		t.Fatal("expected error for non-HS256 algorithm")
	}
	if !strings.Contains(err.Error(), "unexpected signing method") {
		t.Errorf("expected 'unexpected signing method', got: %v", err)
	}
}

func TestValidateToken_Expired(t *testing.T) {
	a := newTestAdapter()
	token := makeToken(testSecret, jwt.MapClaims{
		"sub": "user-1",
		"exp": time.Now().Add(-time.Hour).Unix(),
	}, jwt.SigningMethodHS256)

	err := a.ValidateToken(context.Background(), token)
	if err == nil {
		t.Fatal("expected error for expired token")
	}
}

func TestValidateToken_Malformed(t *testing.T) {
	a := newTestAdapter()
	err := a.ValidateToken(context.Background(), "not-a-jwt")
	if err == nil {
		t.Fatal("expected error for malformed token")
	}
}

func TestUserIDFromToken_Valid(t *testing.T) {
	a := newTestAdapter()
	token := makeToken(testSecret, jwt.MapClaims{
		"sub": "user-42",
		"exp": time.Now().Add(time.Hour).Unix(),
	}, jwt.SigningMethodHS256)

	userID, err := a.UserIDFromToken(token)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if userID != "user-42" {
		t.Errorf("expected user-42, got: %s", userID)
	}
}

func TestUserIDFromToken_EmptyToken(t *testing.T) {
	a := newTestAdapter()
	_, err := a.UserIDFromToken("")
	if err == nil {
		t.Fatal("expected error for empty token")
	}
}

func TestUserIDFromToken_MissingSubClaim(t *testing.T) {
	a := newTestAdapter()
	token := makeToken(testSecret, jwt.MapClaims{
		"exp": time.Now().Add(time.Hour).Unix(),
	}, jwt.SigningMethodHS256)

	_, err := a.UserIDFromToken(token)
	if err == nil {
		t.Fatal("expected error for missing sub claim")
	}
	if !strings.Contains(err.Error(), "missing sub claim") {
		t.Errorf("expected 'missing sub claim', got: %v", err)
	}
}

func TestUserIDFromToken_Expired(t *testing.T) {
	a := newTestAdapter()
	token := makeToken(testSecret, jwt.MapClaims{
		"sub": "user-1",
		"exp": time.Now().Add(-time.Hour).Unix(),
	}, jwt.SigningMethodHS256)

	_, err := a.UserIDFromToken(token)
	if err == nil {
		t.Fatal("expected error for expired token")
	}
}

func TestUserIDFromToken_BearerPrefix(t *testing.T) {
	a := newTestAdapter()
	token := makeToken(testSecret, jwt.MapClaims{
		"sub": "user-99",
		"exp": time.Now().Add(time.Hour).Unix(),
	}, jwt.SigningMethodHS256)

	userID, err := a.UserIDFromToken("Bearer " + token)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if userID != "user-99" {
		t.Errorf("expected user-99, got: %s", userID)
	}
}
