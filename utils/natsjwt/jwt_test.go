package natsjwt

import (
	"strings"
	"testing"
	"time"

	"github.com/nats-io/nkeys"
)

// newTestSigningKeyPair returns a fresh account keypair plus its seed
// in a form suitable for IssueUserJWT (the seed is what the function
// accepts as a string).
func newTestSigningKeyPair(t *testing.T) (string, string) {
	t.Helper()
	kp, err := nkeys.CreateAccount()
	if err != nil {
		t.Fatalf("nkeys.CreateAccount: %v", err)
	}
	seed, err := kp.Seed()
	if err != nil {
		t.Fatalf("kp.Seed: %v", err)
	}
	pub, err := kp.PublicKey()
	if err != nil {
		t.Fatalf("kp.PublicKey: %v", err)
	}
	return string(seed), pub
}

// newTestUserPubKey returns a freshly-created user nkey public key.
func newTestUserPubKey(t *testing.T) string {
	t.Helper()
	kp, err := nkeys.CreateUser()
	if err != nil {
		t.Fatalf("nkeys.CreateUser: %v", err)
	}
	pub, err := kp.PublicKey()
	if err != nil {
		t.Fatalf("kp.PublicKey: %v", err)
	}
	return pub
}

func TestIssueAndParseRoundTrip(t *testing.T) {
	seed, _ := newTestSigningKeyPair(t)
	userPub := newTestUserPubKey(t)

	// Tag keys are lowercased by the NATS library on Add — callers must
	// use lowercase keys upfront if they want exact round-trip equality.
	in := UserClaims{
		Name: "asset-uuid-1234",
		Tags: map[string]string{"orgid": "org-a", "templateid": "tpl-1"},
		Pub: PermissionList{
			Allow: []string{"events.org-a.asset-uuid-1234", "events.org-a.asset-uuid-1234.>"},
		},
		Sub: PermissionList{
			Allow: []string{"_INBOX.>"},
		},
	}

	token, jti, expiresAt, err := IssueUserJWT(seed, userPub, in, 30*24*time.Hour)
	if err != nil {
		t.Fatalf("IssueUserJWT: %v", err)
	}
	if token == "" {
		t.Fatal("token is empty")
	}
	if jti == "" {
		t.Fatal("jti is empty")
	}
	if expiresAt.IsZero() {
		t.Fatal("expiresAt is zero")
	}

	out, parsedJti, parsedExpiresAt, err := ParseUserJWT(token)
	if err != nil {
		t.Fatalf("ParseUserJWT: %v", err)
	}
	if out.Name != in.Name {
		t.Errorf("Name mismatch: got %q, want %q", out.Name, in.Name)
	}
	if parsedJti != jti {
		t.Errorf("jti mismatch: got %q, want %q", parsedJti, jti)
	}
	if !parsedExpiresAt.Equal(expiresAt) {
		t.Errorf("expiresAt mismatch: got %v, want %v", parsedExpiresAt, expiresAt)
	}
	if got, want := out.Tags["orgId"], in.Tags["orgId"]; got != want {
		t.Errorf("Tags[orgId] mismatch: got %q, want %q", got, want)
	}
	if got, want := out.Tags["templateId"], in.Tags["templateId"]; got != want {
		t.Errorf("Tags[templateId] mismatch: got %q, want %q", got, want)
	}
	if got, want := len(out.Pub.Allow), len(in.Pub.Allow); got != want {
		t.Errorf("Pub.Allow length mismatch: got %d, want %d", got, want)
	}
	for i, subj := range in.Pub.Allow {
		if i >= len(out.Pub.Allow) || out.Pub.Allow[i] != subj {
			t.Errorf("Pub.Allow[%d]: got %v, want %q", i, safeIndex(out.Pub.Allow, i), subj)
		}
	}
	if got, want := len(out.Sub.Allow), len(in.Sub.Allow); got != want {
		t.Errorf("Sub.Allow length mismatch: got %d, want %d", got, want)
	}
}

func TestExpiry(t *testing.T) {
	seed, _ := newTestSigningKeyPair(t)
	userPub := newTestUserPubKey(t)

	before := time.Now().UTC().Add(time.Hour).Add(-time.Second)
	_, _, expiresAt, err := IssueUserJWT(seed, userPub, UserClaims{Name: "x"}, time.Hour)
	if err != nil {
		t.Fatalf("IssueUserJWT: %v", err)
	}
	after := time.Now().UTC().Add(time.Hour).Add(time.Second)

	if expiresAt.Before(before) || expiresAt.After(after) {
		t.Errorf("expiresAt %v not within [%v, %v]", expiresAt, before, after)
	}
}

func TestInvalidSeed_Empty(t *testing.T) {
	_, _, _, err := IssueUserJWT("", newTestUserPubKey(t), UserClaims{}, time.Hour)
	if err == nil {
		t.Fatal("expected error for empty seed")
	}
}

func TestInvalidSeed_Malformed(t *testing.T) {
	_, _, _, err := IssueUserJWT("not-a-real-seed", newTestUserPubKey(t), UserClaims{}, time.Hour)
	if err == nil {
		t.Fatal("expected error for malformed seed")
	}
	if strings.Contains(err.Error(), "panic") {
		t.Fatalf("unexpected panic in error: %v", err)
	}
}

func TestParseUserJWT_Empty(t *testing.T) {
	_, _, _, err := ParseUserJWT("")
	if err == nil {
		t.Fatal("expected error for empty jwt")
	}
}

func TestTamperedJWT(t *testing.T) {
	seed, _ := newTestSigningKeyPair(t)
	userPub := newTestUserPubKey(t)

	token, _, _, err := IssueUserJWT(seed, userPub, UserClaims{Name: "x"}, time.Hour)
	if err != nil {
		t.Fatalf("IssueUserJWT: %v", err)
	}

	// Mutate one byte in the middle of the JWT (in the payload section,
	// past the header and dot separator). This breaks the signature.
	mid := len(token) / 2
	tamperedRune := byte('!')
	if token[mid] == '!' {
		tamperedRune = '?'
	}
	tampered := token[:mid] + string(tamperedRune) + token[mid+1:]

	_, _, _, err = ParseUserJWT(tampered)
	if err == nil {
		t.Fatal("expected error parsing tampered jwt")
	}
}

// safeIndex avoids index-out-of-range panics in error messages when a
// length mismatch already failed an earlier assertion.
func safeIndex(xs []string, i int) string {
	if i < 0 || i >= len(xs) {
		return "<missing>"
	}
	return xs[i]
}
