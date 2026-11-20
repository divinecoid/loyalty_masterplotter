package notify

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestFonnteSend(t *testing.T) {
	var got struct{ auth, target, msg, cc, ctype string }
	reply := `{"detail":"success! message in queue","status":true,"id":["1"]}`
	code := 200
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.ParseForm()
		got.auth, got.target, got.msg, got.cc, got.ctype = r.Header.Get("Authorization"), r.PostForm.Get("target"), r.PostForm.Get("message"), r.PostForm.Get("countryCode"), r.Header.Get("Content-Type")
		w.WriteHeader(code)
		w.Write([]byte(reply))
	}))
	defer srv.Close()
	f := NewFonnte("TOK123")
	f.URL = srv.URL

	if err := f.Send(context.Background(), "+6281234567890", "Kode: 123456 & lainnya"); err != nil {
		t.Fatal(err)
	}
	if got.auth != "TOK123" || got.target != "6281234567890" || got.msg != "Kode: 123456 & lainnya" || got.cc != "0" {
		t.Fatalf("request salah: %+v", got)
	}

	for name, tc := range map[string]struct {
		code  int
		reply string
	}{
		"status false":   {200, `{"reason":"token invalid","status":false}`},
		"Status kapital": {200, `{"reason":"token invalid","Status":false}`},
		"http error":     {500, `{"status":true}`},
		"tanpa status":   {200, `{"detail":"x"}`},
		"bukan json":     {200, `<html>`},
		"kuota habis":    {200, `{"reason":"insufficient quota","status":false}`},
	} {
		code, reply = tc.code, tc.reply
		if err := f.Send(context.Background(), "+6281234567890", "x"); err == nil {
			t.Errorf("%s: harusnya error", name)
		}
	}
}
