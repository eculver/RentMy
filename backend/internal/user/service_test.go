package user_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Brett2thered/RentMy/backend/internal/platform/auth"
	"github.com/Brett2thered/RentMy/backend/internal/user"
)

// fakeRepo is an in-memory user repository for unit tests.
type fakeRepo struct {
	mu    sync.Mutex
	users map[string]*user.User
	hashes map[string]string // email -> hash
}

func newFakeRepo() *fakeRepo {
	return &fakeRepo{
		users:  make(map[string]*user.User),
		hashes: make(map[string]string),
	}
}

func (f *fakeRepo) Insert(_ context.Context, u *user.User, hash string) (*user.User, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if u.Email != nil {
		if _, exists := f.hashes[*u.Email]; exists {
			return nil, user.ErrEmailTaken
		}
		f.hashes[*u.Email] = hash
	}
	cp := *u
	f.users[u.ID] = &cp
	return &cp, nil
}

func (f *fakeRepo) FindByID(_ context.Context, id string) (*user.User, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	u, ok := f.users[id]
	if !ok {
		return nil, user.ErrNotFound
	}
	cp := *u
	return &cp, nil
}

func (f *fakeRepo) FindByEmail(_ context.Context, email string) (*user.User, string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	hash, ok := f.hashes[email]
	if !ok {
		return nil, "", user.ErrNotFound
	}
	for _, u := range f.users {
		if u.Email != nil && *u.Email == email {
			cp := *u
			return &cp, hash, nil
		}
	}
	return nil, "", user.ErrNotFound
}

func (f *fakeRepo) UpdateLastActive(_ context.Context, _ string) error { return nil }

func (f *fakeRepo) UpdateIdentityStatus(_ context.Context, id string, status user.IdentityStatus) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if u, ok := f.users[id]; ok {
		u.IdentityStatus = status
	}
	return nil
}

func (f *fakeRepo) AddReputationScore(_ context.Context, id string, delta int) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if u, ok := f.users[id]; ok {
		score := u.ReputationScore + delta
		if score < 0 {
			score = 0
		}
		if score > 1000 {
			score = 1000
		}
		u.ReputationScore = score
	}
	return nil
}

func (f *fakeRepo) Update(_ context.Context, id string, in user.UpdateInput) (*user.User, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	u, ok := f.users[id]
	if !ok {
		return nil, user.ErrNotFound
	}
	if in.Name != nil {
		u.Name = *in.Name
	}
	if in.AvatarURL != nil {
		u.AvatarURL = in.AvatarURL
	}
	cp := *u
	return &cp, nil
}

// fakeRedis is an in-memory key/value store for testing.
type fakeRedis struct {
	mu    sync.Mutex
	data  map[string]string
}

func newFakeRedis() *fakeRedis {
	return &fakeRedis{data: make(map[string]string)}
}

func (f *fakeRedis) Set(_ context.Context, key, value string, _ time.Duration) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.data[key] = value
	return nil
}

func (f *fakeRedis) Get(_ context.Context, key string) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	v, ok := f.data[key]
	if !ok {
		return "", &notFoundErr{}
	}
	return v, nil
}

func (f *fakeRedis) Del(_ context.Context, key string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.data, key)
	return nil
}

type notFoundErr struct{}

func (e *notFoundErr) Error() string { return "not found" }

// repoAdapter adapts fakeRepo to the user.RepositoryInterface used by Service.
// (Service takes *Repository directly; we test via the exported Register/Login
// surface which calls the concrete methods. For unit tests we use a stub
// service built around a fake via the unexported constructor approach.)
//
// Instead of fighting the concrete dependency, we build the service with a
// real Repository wired to a testcontainers Postgres in integration tests.
// Here we just test the logic that doesn't touch the DB directly.

func makeIssuer() *auth.Issuer {
	return auth.NewIssuer("test-secret", 15*time.Minute, 7*24*time.Hour)
}

// serviceWithFakes builds a *user.Service backed by fakes.
// This requires the Service to accept interfaces — which it does via RedisStore.
// For the repository we use the concrete *Repository which hits Postgres;
// these tests validate bcrypt, JWT, and validation logic only.

func TestRegister_ValidationRejectsShortPassword(t *testing.T) {
	// We can test input validation without a real repo because validation
	// happens before any repo call.
	// Build a service with nil repo to confirm it panics only if we reach repo.
	issuer := makeIssuer()
	redis := newFakeRedis()

	// nil repo is fine here — validation will fail before repo is called.
	svc := user.NewServiceWithInterfaces(nil, issuer, redis)

	_, err := svc.Register(context.Background(), user.RegisterInput{
		Email:    "test@example.com",
		Password: "short",
		Name:     "Test User",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "validation")
}

func TestRegister_ValidationRejectsInvalidEmail(t *testing.T) {
	issuer := makeIssuer()
	redis := newFakeRedis()
	svc := user.NewServiceWithInterfaces(nil, issuer, redis)

	_, err := svc.Register(context.Background(), user.RegisterInput{
		Email:    "not-an-email",
		Password: "ValidPass1!",
		Name:     "Test User",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "validation")
}

func TestLogin_BadCredentials(t *testing.T) {
	issuer := makeIssuer()
	redis := newFakeRedis()
	repo := newFakeRepo()
	svc := user.NewServiceWithInterfaces(repo, issuer, redis)

	// Register first.
	_, err := svc.Register(context.Background(), user.RegisterInput{
		Email:    "user@example.com",
		Password: "ValidPass1!",
		Name:     "Test User",
	})
	require.NoError(t, err)

	// Wrong password.
	_, err = svc.Login(context.Background(), user.LoginInput{
		Email:    "user@example.com",
		Password: "WrongPassword!",
	})
	require.Error(t, err)
	assert.ErrorIs(t, err, user.ErrBadCredentials)
}

func TestRegisterAndLogin_RoundTrip(t *testing.T) {
	issuer := makeIssuer()
	redis := newFakeRedis()
	repo := newFakeRepo()
	svc := user.NewServiceWithInterfaces(repo, issuer, redis)

	resp, err := svc.Register(context.Background(), user.RegisterInput{
		Email:    "roundtrip@example.com",
		Password: "ValidPass1!",
		Name:     "Round Trip",
	})
	require.NoError(t, err)
	require.NotEmpty(t, resp.AccessToken)
	require.NotEmpty(t, resp.RefreshToken)
	assert.Equal(t, "Round Trip", resp.User.Name)
	assert.Equal(t, user.IdentityStatusPending, resp.User.IdentityStatus)

	loginResp, err := svc.Login(context.Background(), user.LoginInput{
		Email:    "roundtrip@example.com",
		Password: "ValidPass1!",
	})
	require.NoError(t, err)
	assert.NotEmpty(t, loginResp.AccessToken)
}

func TestRefresh_TokenRotation(t *testing.T) {
	issuer := makeIssuer()
	redis := newFakeRedis()
	repo := newFakeRepo()
	svc := user.NewServiceWithInterfaces(repo, issuer, redis)

	reg, err := svc.Register(context.Background(), user.RegisterInput{
		Email:    "refresh@example.com",
		Password: "ValidPass1!",
		Name:     "Refresh User",
	})
	require.NoError(t, err)

	refreshResp, err := svc.Refresh(context.Background(), user.RefreshInput{
		RefreshToken: reg.RefreshToken,
	})
	require.NoError(t, err)
	assert.NotEmpty(t, refreshResp.AccessToken)
	// Old token should no longer work after rotation.
	_, err = svc.Refresh(context.Background(), user.RefreshInput{
		RefreshToken: reg.RefreshToken,
	})
	require.Error(t, err)
}

func TestRegister_DuplicateEmail(t *testing.T) {
	issuer := makeIssuer()
	redis := newFakeRedis()
	repo := newFakeRepo()
	svc := user.NewServiceWithInterfaces(repo, issuer, redis)

	in := user.RegisterInput{
		Email:    "dup@example.com",
		Password: "ValidPass1!",
		Name:     "First",
	}
	_, err := svc.Register(context.Background(), in)
	require.NoError(t, err)

	_, err = svc.Register(context.Background(), in)
	require.Error(t, err)
	assert.ErrorIs(t, err, user.ErrEmailTaken)
}
