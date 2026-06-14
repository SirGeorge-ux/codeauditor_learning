package supabase

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

// SupabaseClient provides methods to interact with Supabase Auth REST API.
type SupabaseClient struct {
	baseURL string
	apiKey  string
	http    *http.Client
}

// NewSupabaseClient creates a new SupabaseClient.
func NewSupabaseClient(baseURL, apiKey string) *SupabaseClient {
	return &SupabaseClient{
		baseURL: strings.TrimSuffix(baseURL, "/"),
		apiKey:  apiKey,
		http:    &http.Client{},
	}
}

// AuthResponse represents the response from Supabase auth endpoints.
type AuthResponse struct {
	AccessToken  string   `json:"access_token"`
	RefreshToken string   `json:"refresh_token"`
	TokenType    string   `json:"token_type"`
	ExpiresIn    int      `json:"expires_in"`
	ExpiresAt    int64    `json:"expires_at"`
	User         UserData `json:"user"`
}

// UserData represents the user data from Supabase.
type UserData struct {
	ID               string `json:"id"`
	Email            string `json:"email"`
	CreatedAt        string `json:"created_at"`
	EmailConfirmedAt string `json:"email_confirmed_at,omitempty"`
	LastSignInAt     string `json:"last_sign_in_at,omitempty"`
}

// SignUp registers a new user with email and password.
func (c *SupabaseClient) SignUp(email, password string) (*AuthResponse, error) {
	payload := map[string]string{
		"email":    email,
		"password": password,
	}
	return c.post("/auth/v1/signup", payload)
}

// SignIn authenticates a user with email and password.
func (c *SupabaseClient) SignIn(email, password string) (*AuthResponse, error) {
	payload := map[string]string{
		"email":    email,
		"password": password,
	}
	return c.post("/auth/v1/token?grant_type=password", payload)
}

// SignOut logs out the user using the access token.
func (c *SupabaseClient) SignOut(accessToken string) error {
	req, err := http.NewRequest("POST", c.baseURL+"/auth/v1/logout", nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("apikey", c.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("signout failed with status %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

func (c *SupabaseClient) post(path string, payload map[string]string) (*AuthResponse, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal payload: %w", err)
	}

	req, err := http.NewRequest("POST", c.baseURL+path, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("apikey", c.apiKey)
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("request failed with status %d: %s", resp.StatusCode, string(respBody))
	}

	var authResp AuthResponse
	if err := json.Unmarshal(respBody, &authResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return &authResp, nil
}

// BuildQueryURL builds a URL with query parameters.
func BuildQueryURL(basePath string, params map[string]string) string {
	if len(params) == 0 {
		return basePath
	}
	query := url.Values{}
	for k, v := range params {
		query.Set(k, v)
	}
	return basePath + "?" + query.Encode()
}
