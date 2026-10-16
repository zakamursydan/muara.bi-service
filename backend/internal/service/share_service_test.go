package service

import (
	"encoding/base64"
	"strings"
	"testing"
	"time"

	"golang.org/x/crypto/bcrypt"
	"muara.bi/core/internal/config"
	"muara.bi/core/internal/domain/entity"
)

// ─── test helpers ─────────────────────────────────────────────────────────────

func newTestShareSvc(t *testing.T) *ShareService {
	t.Helper()
	return &ShareService{
		cfg: &config.Config{
			AppSecretKey:     "test-secret-key-32-chars-exactly!",
			AppPublicBaseURL: "https://app.example.com",
		},
	}
}

// ─── generateShareToken ────────────────────────────────────────────────────────

func TestShareToken_Length(t *testing.T) {
	tok, err := generateShareToken()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// 32 raw bytes → base64.RawURL → 43 chars (ceil(32*4/3))
	if len(tok) != 43 {
		t.Errorf("expected 43 chars, got %d: %q", len(tok), tok)
	}
}

func TestShareToken_URLSafe(t *testing.T) {
	tok, err := generateShareToken()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, err := base64.RawURLEncoding.DecodeString(tok); err != nil {
		t.Errorf("token is not valid base64url: %v", err)
	}
	if strings.ContainsAny(tok, "+/=") {
		t.Errorf("token contains non-URL-safe char: %q", tok)
	}
}

func TestShareToken_Entropy(t *testing.T) {
	const n = 100
	seen := make(map[string]struct{}, n)
	for i := 0; i < n; i++ {
		tok, err := generateShareToken()
		if err != nil {
			t.Fatalf("error on iteration %d: %v", i, err)
		}
		if _, dup := seen[tok]; dup {
			t.Fatalf("duplicate token on iteration %d", i)
		}
		seen[tok] = struct{}{}
	}
}

// ─── normalizeAllowedDomain ───────────────────────────────────────────────────

func TestNormalizeAllowedDomain_Accept(t *testing.T) {
	cases := []struct{ input, want string }{
		{"https://company.com", "https://company.com"},
		{"https://app.company.com", "https://app.company.com"},
		{"https://app.company.com/", "https://app.company.com"},
		{"https://app.company.com:8443", "https://app.company.com:8443"},
		{"http://localhost", "http://localhost"},
		{"http://localhost:3000", "http://localhost:3000"},
		{"http://127.0.0.1", "http://127.0.0.1"},
		{"http://my.localhost", "http://my.localhost"},
	}
	for _, tc := range cases {
		got, err := normalizeAllowedDomain(tc.input)
		if err != nil {
			t.Errorf("input=%q: unexpected error: %v", tc.input, err)
			continue
		}
		if got != tc.want {
			t.Errorf("input=%q: got %q, want %q", tc.input, got, tc.want)
		}
	}
}

func TestNormalizeAllowedDomain_Reject(t *testing.T) {
	cases := []string{
		"",
		"ftp://app.company.com",
		"http://app.company.com",      // non-localhost http → must fail
		"http://external.example.com", // non-localhost http
		"https://company.com/path",
		"https://company.com?q=1",
		"https://company.com#frag",
		"https://*.company.com",
		"not-a-url",
		"//company.com", // missing scheme
	}
	for _, input := range cases {
		_, err := normalizeAllowedDomain(input)
		if err == nil {
			t.Errorf("input=%q: expected error but got nil", input)
		}
	}
}

func TestNormalizeAllowedDomains_Dedup(t *testing.T) {
	raw := []string{
		"https://a.example.com",
		"https://b.example.com",
		"https://a.example.com", // duplicate
	}
	got, err := normalizeAllowedDomains(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 2 {
		t.Errorf("expected 2 unique domains, got %d: %v", len(got), got)
	}
}

// ─── SignEmbedToken / VerifyEmbedToken ────────────────────────────────────────

func makeShare(expiresAt *time.Time) *entity.DashboardShare {
	return &entity.DashboardShare{
		ID:             "share-1",
		DashboardID:    "dash-abc",
		ShareToken:     "testtoken123",
		AccessMode:     entity.AccessModeEmbedIFrame,
		EnvTag:         "production",
		AllowedDomains: []string{"https://partner.company.com"},
		IsActive:       true,
		ExpiresAt:      expiresAt,
	}
}

func TestSignAndVerifyEmbedToken_Valid(t *testing.T) {
	svc := newTestShareSvc(t)
	future := time.Now().Add(24 * time.Hour)
	share := makeShare(&future)

	tokenStr, err := svc.SignEmbedToken(share)
	if err != nil {
		t.Fatalf("SignEmbedToken: %v", err)
	}
	if tokenStr == "" {
		t.Fatal("expected non-empty token")
	}

	claims, err := svc.VerifyEmbedToken(tokenStr)
	if err != nil {
		t.Fatalf("VerifyEmbedToken: %v", err)
	}
	if claims.ShareToken != share.ShareToken {
		t.Errorf("share_token mismatch: got %q want %q", claims.ShareToken, share.ShareToken)
	}
	if claims.DashboardID != share.DashboardID {
		t.Errorf("dashboard_id mismatch: got %q want %q", claims.DashboardID, share.DashboardID)
	}
	if claims.EnvTag != share.EnvTag {
		t.Errorf("env_tag mismatch: got %q want %q", claims.EnvTag, share.EnvTag)
	}
	if claims.Issuer != embedTokenIssuer {
		t.Errorf("issuer mismatch: got %q want %q", claims.Issuer, embedTokenIssuer)
	}
}

func TestSignEmbedToken_CappedAtShareExpiry(t *testing.T) {
	svc := newTestShareSvc(t)
	shortExpiry := time.Now().Add(1 * time.Minute)
	share := makeShare(&shortExpiry)

	tokenStr, err := svc.SignEmbedToken(share)
	if err != nil {
		t.Fatalf("SignEmbedToken: %v", err)
	}

	claims, err := svc.VerifyEmbedToken(tokenStr)
	if err != nil {
		t.Fatalf("VerifyEmbedToken: %v", err)
	}
	// Token expiry must not exceed share expiry (allow 1s tolerance for clock)
	if claims.ExpiresAt.Time.After(shortExpiry.Add(time.Second)) {
		t.Errorf("token expiry %v exceeds share expiry %v", claims.ExpiresAt.Time, shortExpiry)
	}
}

func TestVerifyEmbedToken_ExpiredToken(t *testing.T) {
	svc := newTestShareSvc(t)
	past := time.Now().Add(-1 * time.Second)
	share := makeShare(&past)

	tokenStr, err := svc.SignEmbedToken(share)
	if err != nil {
		t.Fatalf("SignEmbedToken: %v", err)
	}

	_, err = svc.VerifyEmbedToken(tokenStr)
	if err == nil {
		t.Error("expected error for expired token, got nil")
	}
}

func TestVerifyEmbedToken_WrongKey(t *testing.T) {
	svc := newTestShareSvc(t)
	future := time.Now().Add(1 * time.Hour)
	share := makeShare(&future)

	tokenStr, err := svc.SignEmbedToken(share)
	if err != nil {
		t.Fatalf("SignEmbedToken: %v", err)
	}

	svc2 := &ShareService{cfg: &config.Config{AppSecretKey: "completely-different-signing-key!"}}
	_, err = svc2.VerifyEmbedToken(tokenStr)
	if err == nil {
		t.Error("expected error for wrong signing key, got nil")
	}
}

func TestVerifyEmbedToken_Malformed(t *testing.T) {
	svc := newTestShareSvc(t)
	_, err := svc.VerifyEmbedToken("not.a.valid.jwt")
	if err == nil {
		t.Error("expected error for malformed token, got nil")
	}
}

// ─── verifySharePassword ──────────────────────────────────────────────────────

func TestVerifySharePassword_Unprotected(t *testing.T) {
	svc := newTestShareSvc(t)
	share := &entity.DashboardShare{PasswordHash: ""}

	if !svc.verifySharePassword(share, "") {
		t.Error("unprotected share must pass with no password")
	}
	if !svc.verifySharePassword(share, "anything") {
		t.Error("unprotected share must pass regardless of input")
	}
}

func TestVerifySharePassword_Protected(t *testing.T) {
	svc := newTestShareSvc(t)
	const password = "correct-horse-battery"
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.MinCost)
	if err != nil {
		t.Fatalf("bcrypt: %v", err)
	}
	share := &entity.DashboardShare{PasswordHash: string(hash)}

	if !svc.verifySharePassword(share, password) {
		t.Error("correct password must pass")
	}
	if svc.verifySharePassword(share, "wrong-password") {
		t.Error("wrong password must fail")
	}
	if svc.verifySharePassword(share, "") {
		t.Error("empty password on protected share must fail")
	}
}
