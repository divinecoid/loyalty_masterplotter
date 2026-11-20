package auth

import (
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func TestPasswordRoundTrip(t *testing.T) {
	h, err := HashPassword("s3cret-pass")
	if err != nil {
		t.Fatal(err)
	}
	if !VerifyPassword("s3cret-pass", h) {
		t.Fatal("correct password rejected")
	}
	if VerifyPassword("wrong", h) {
		t.Fatal("wrong password accepted")
	}
	if VerifyPassword("x", "garbage") {
		t.Fatal("garbage hash accepted")
	}
}

func TestTokenRoundTripAndTamper(t *testing.T) {
	tk := NewTokens("secret-secret-secret-secret-1234", time.Hour)
	s, err := tk.Issue(7, "owner")
	if err != nil {
		t.Fatal(err)
	}
	c, err := tk.Parse(s)
	if err != nil || c.AdminID != 7 || c.Role != "owner" {
		t.Fatalf("parse: %v %+v", err, c)
	}
	if _, err := NewTokens("other-secret-other-secret-other-1", time.Hour).Parse(s); err == nil {
		t.Fatal("token signed with other key accepted")
	}
	expired := NewTokens("secret-secret-secret-secret-1234", -time.Minute)
	e, _ := expired.Issue(1, "staff")
	if _, err := tk.Parse(e); err == nil {
		t.Fatal("expired token accepted")
	}
	none, _ := jwt.NewWithClaims(jwt.SigningMethodNone, Claims{AdminID: 1}).SignedString(jwt.UnsafeAllowNoneSignatureType)
	if _, err := tk.Parse(none); err == nil {
		t.Fatal("alg=none accepted")
	}
}
