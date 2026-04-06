package integration

import (
	"net/http"
	"testing"
)

// TestRegisterAndLogin verifies the full register → login → JWT flow.
func TestRegisterAndLogin(t *testing.T) {
	ts, client := NewTestServer(t)
	pool := NewTestDB(t)
	CleanupDB(t, pool)

	// Register a new user.
	regResp := DoJSON(t, client, http.MethodPost, ts.URL+"/api/v1/auth/register", map[string]any{
		"email":    "alice@example.com",
		"password": "supersecret1",
		"name":     "Alice Test",
	}, "")
	defer DrainBody(regResp)

	if regResp.StatusCode != http.StatusCreated {
		body, _ := readBody(regResp)
		t.Fatalf("register: expected 201, got %d: %s", regResp.StatusCode, body)
	}

	var regBody struct {
		User struct {
			ID    string `json:"id"`
			Name  string `json:"name"`
			Email string `json:"email"`
		} `json:"user"`
		AccessToken  string `json:"accessToken"`
		RefreshToken string `json:"refreshToken"`
	}
	MustDecodeJSON(t, regResp, &regBody)

	if regBody.User.ID == "" {
		t.Error("register: user.id is empty")
	}
	if regBody.User.Name != "Alice Test" {
		t.Errorf("register: user.name = %q, want %q", regBody.User.Name, "Alice Test")
	}
	if regBody.AccessToken == "" {
		t.Error("register: accessToken is empty")
	}
	if regBody.RefreshToken == "" {
		t.Error("register: refreshToken is empty")
	}

	// Login with the same credentials.
	loginResp := DoJSON(t, client, http.MethodPost, ts.URL+"/api/v1/auth/login", map[string]any{
		"email":    "alice@example.com",
		"password": "supersecret1",
	}, "")
	defer DrainBody(loginResp)

	if loginResp.StatusCode != http.StatusOK {
		body, _ := readBody(loginResp)
		t.Fatalf("login: expected 200, got %d: %s", loginResp.StatusCode, body)
	}

	var loginBody struct {
		AccessToken  string `json:"accessToken"`
		RefreshToken string `json:"refreshToken"`
	}
	MustDecodeJSON(t, loginResp, &loginBody)

	if loginBody.AccessToken == "" {
		t.Error("login: accessToken is empty")
	}
}

// TestRegisterDuplicateEmail verifies that registering with an existing email returns 409.
func TestRegisterDuplicateEmail(t *testing.T) {
	ts, client := NewTestServer(t)
	pool := NewTestDB(t)
	CleanupDB(t, pool)

	body := map[string]any{
		"email":    "dup@example.com",
		"password": "supersecret1",
		"name":     "Dup User",
	}

	// First registration should succeed.
	resp1 := DoJSON(t, client, http.MethodPost, ts.URL+"/api/v1/auth/register", body, "")
	DrainBody(resp1)
	if resp1.StatusCode != http.StatusCreated {
		t.Fatalf("first register: expected 201, got %d", resp1.StatusCode)
	}

	// Second registration with the same email should return 409.
	resp2 := DoJSON(t, client, http.MethodPost, ts.URL+"/api/v1/auth/register", body, "")
	defer DrainBody(resp2)
	if resp2.StatusCode != http.StatusConflict {
		t.Fatalf("duplicate register: expected 409, got %d", resp2.StatusCode)
	}
}

// TestGetProfile verifies that GET /users/me returns the authenticated user's data.
func TestGetProfile(t *testing.T) {
	ts, client := NewTestServer(t)
	pool := NewTestDB(t)
	CleanupDB(t, pool)

	u := CreateTestUser(t, pool)
	token := LoginTestUser(t, client, ts.URL, *u.Email, "password123")

	resp := DoJSON(t, client, http.MethodGet, ts.URL+"/api/v1/users/me", nil, token)
	defer DrainBody(resp)

	if resp.StatusCode != http.StatusOK {
		body, _ := readBody(resp)
		t.Fatalf("GET /users/me: expected 200, got %d: %s", resp.StatusCode, body)
	}

	var body struct {
		User struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"user"`
	}
	MustDecodeJSON(t, resp, &body)

	if body.User.ID != u.ID {
		t.Errorf("user.id = %q, want %q", body.User.ID, u.ID)
	}
	if body.User.Name != u.Name {
		t.Errorf("user.name = %q, want %q", body.User.Name, u.Name)
	}
}

// TestUpdateProfile verifies that PUT /users/me updates name successfully.
func TestUpdateProfile(t *testing.T) {
	ts, client := NewTestServer(t)
	pool := NewTestDB(t)
	CleanupDB(t, pool)

	u := CreateTestUser(t, pool)
	token := LoginTestUser(t, client, ts.URL, *u.Email, "password123")

	newName := "Updated Name"
	resp := DoJSON(t, client, http.MethodPut, ts.URL+"/api/v1/users/me", map[string]any{
		"name": newName,
	}, token)
	defer DrainBody(resp)

	if resp.StatusCode != http.StatusOK {
		body, _ := readBody(resp)
		t.Fatalf("PUT /users/me: expected 200, got %d: %s", resp.StatusCode, body)
	}

	var body struct {
		User struct {
			Name string `json:"name"`
		} `json:"user"`
	}
	MustDecodeJSON(t, resp, &body)

	if body.User.Name != newName {
		t.Errorf("user.name = %q, want %q", body.User.Name, newName)
	}
}

// TestInvalidToken verifies that requests with a bad JWT token return 401.
func TestInvalidToken(t *testing.T) {
	ts, client := NewTestServer(t)

	resp := DoJSON(t, client, http.MethodGet, ts.URL+"/api/v1/users/me", nil, "not-a-valid-token")
	defer DrainBody(resp)

	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", resp.StatusCode)
	}
}

// TestMissingToken verifies that requests without any Authorization header return 401.
func TestMissingToken(t *testing.T) {
	ts, client := NewTestServer(t)

	resp := DoJSON(t, client, http.MethodGet, ts.URL+"/api/v1/users/me", nil, "")
	defer DrainBody(resp)

	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", resp.StatusCode)
	}
}
