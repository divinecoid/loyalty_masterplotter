package phone

import "testing"

func TestNormalize(t *testing.T) {
	ok := map[string]string{
		"081234567890":       "+6281234567890",
		"81234567890":        "+6281234567890",
		"6281234567890":      "+6281234567890",
		"+62 812-3456-7890":  "+6281234567890",
		" (0812) 3456 7890 ": "+6281234567890",
	}
	for in, want := range ok {
		got, err := Normalize(in)
		if err != nil || got != want {
			t.Errorf("%q => %q, %v (want %q)", in, got, err, want)
		}
	}
	for _, in := range []string{"", "abc", "0212345678", "08123", "+1 415 555 0100", "0812345678901234", "08123x567890"} {
		if got, err := Normalize(in); err == nil {
			t.Errorf("%q seharusnya ditolak, dapat %q", in, got)
		}
	}
}

func TestMask(t *testing.T) {
	if got := Mask("+6281234567890"); got != "+62812*****890" {
		t.Fatal(got)
	}
}
