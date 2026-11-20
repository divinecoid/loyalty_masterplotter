package auth

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/argon2"
)

const (
	argonTime    = 2
	argonMemory  = 64 * 1024
	argonThreads = 2
	argonKeyLen  = 32
	saltLen      = 16
)

// HashPassword menghasilkan "argon2id$m$t$p$salt$hash" (base64 raw).
func HashPassword(pw string) (string, error) {
	salt := make([]byte, saltLen)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	h := argon2.IDKey([]byte(pw), salt, argonTime, argonMemory, argonThreads, argonKeyLen)
	return fmt.Sprintf("argon2id$%d$%d$%d$%s$%s", argonMemory, argonTime, argonThreads,
		base64.RawStdEncoding.EncodeToString(salt), base64.RawStdEncoding.EncodeToString(h)), nil
}

func VerifyPassword(pw, encoded string) bool {
	p := strings.Split(encoded, "$")
	if len(p) != 6 || p[0] != "argon2id" {
		return false
	}
	var m uint32
	var t uint32
	var th uint8
	if _, err := fmt.Sscanf(p[1]+" "+p[2]+" "+p[3], "%d %d %d", &m, &t, &th); err != nil {
		return false
	}
	salt, err1 := base64.RawStdEncoding.DecodeString(p[4])
	want, err2 := base64.RawStdEncoding.DecodeString(p[5])
	if err1 != nil || err2 != nil || len(want) == 0 {
		return false
	}
	got := argon2.IDKey([]byte(pw), salt, t, m, th, uint32(len(want)))
	return subtle.ConstantTimeCompare(got, want) == 1
}

type Claims struct {
	AdminID int64  `json:"aid"`
	Role    string `json:"role"`
	jwt.RegisteredClaims
}

type Tokens struct {
	secret []byte
	TTL    time.Duration
}

func NewTokens(secret string, ttl time.Duration) *Tokens {
	return &Tokens{secret: []byte(secret), TTL: ttl}
}

func (t *Tokens) Issue(adminID int64, role string) (string, error) {
	now := time.Now()
	c := Claims{AdminID: adminID, Role: role, RegisteredClaims: jwt.RegisteredClaims{
		IssuedAt:  jwt.NewNumericDate(now),
		ExpiresAt: jwt.NewNumericDate(now.Add(t.TTL)),
	}}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, c).SignedString(t.secret)
}

var ErrInvalidToken = errors.New("invalid token")

func (t *Tokens) Parse(s string) (*Claims, error) {
	var c Claims
	tok, err := jwt.ParseWithClaims(s, &c, func(*jwt.Token) (any, error) { return t.secret, nil },
		jwt.WithValidMethods([]string{"HS256"}), jwt.WithExpirationRequired())
	if err != nil || !tok.Valid {
		return nil, ErrInvalidToken
	}
	return &c, nil
}
