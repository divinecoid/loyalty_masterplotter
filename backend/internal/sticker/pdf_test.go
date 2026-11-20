package sticker

import (
	"bytes"
	"fmt"
	"testing"
)

func TestRenderProducesPDFWithPages(t *testing.T) {
	var items []Item
	for i := 0; i < 120; i++ { // 54 per halaman pada sel 30mm -> 3 halaman
		items = append(items, Item{Content: fmt.Sprintf("https://x.test/s/%036d", i), Label: fmt.Sprintf("B1-%04d", i+1)})
	}
	var buf bytes.Buffer
	if err := Render(&buf, items, Options{Title: "Tinta Epson", Guides: true}); err != nil {
		t.Fatal(err)
	}
	b := buf.Bytes()
	if !bytes.HasPrefix(b, []byte("%PDF-")) {
		t.Fatal("bukan PDF")
	}
	if n := bytes.Count(b, []byte("/Type /Page\n")); n != 3 {
		t.Fatalf("want 3 pages, got %d", n)
	}
}

func TestRenderRejectsBadSize(t *testing.T) {
	if err := Render(&bytes.Buffer{}, nil, Options{CellMM: 5}); err == nil {
		t.Fatal("ukuran 5mm seharusnya ditolak")
	}
}
