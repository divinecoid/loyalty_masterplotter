package qr

import (
	"testing"

	"github.com/google/uuid"
)

func TestNewCodeIsV4AndUnique(t *testing.T) {
	seen := map[uuid.UUID]bool{}
	for i := 0; i < 10000; i++ {
		c := NewCode()
		if c.Version() != 4 {
			t.Fatalf("want v4, got v%d", c.Version())
		}
		if seen[c] {
			t.Fatal("duplicate code")
		}
		seen[c] = true
	}
}

func TestURL(t *testing.T) {
	c := uuid.MustParse("123e4567-e89b-42d3-a456-426614174000")
	got := URL("https://x.test/", c)
	if got != "https://x.test/s/123e4567-e89b-42d3-a456-426614174000" {
		t.Fatal(got)
	}
}
