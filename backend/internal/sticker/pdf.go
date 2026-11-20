// Package sticker membuat lembar PDF siap cetak berisi QR code per stiker.
package sticker

import (
	"bytes"
	"fmt"
	"io"

	"github.com/go-pdf/fpdf"
	"github.com/skip2/go-qrcode"
)

// Item satu stiker: Content = isi QR (URL), Label = teks pendek untuk staf (BUKAN UUID).
type Item struct {
	Content string
	Label   string
}

type Options struct {
	CellMM  float64 // sisi sel stiker (mm), default 30
	QRMM    float64 // sisi QR (mm), default CellMM-6
	Title   string  // teks kecil di bawah QR, mis. nama produk
	Guides  bool    // garis potong tipis
	MarginM float64 // margin halaman (mm), default 10
}

const (
	pageW = 210.0
	pageH = 297.0
)

func (o *Options) defaults() error {
	if o.CellMM == 0 {
		o.CellMM = 30
	}
	if o.CellMM < 15 || o.CellMM > 90 {
		return fmt.Errorf("cell harus 15-90 mm")
	}
	if o.QRMM == 0 {
		o.QRMM = o.CellMM - 6
	}
	if o.QRMM < 10 || o.QRMM > o.CellMM {
		return fmt.Errorf("ukuran QR tidak valid")
	}
	if o.MarginM == 0 {
		o.MarginM = 10
	}
	return nil
}

// Render menulis PDF A4 ke w. Error correction level High agar QR tetap terbaca di bawah lapisan hologram.
func Render(w io.Writer, items []Item, o Options) error {
	if err := o.defaults(); err != nil {
		return err
	}
	cols := int((pageW - 2*o.MarginM) / o.CellMM)
	rows := int((pageH - 2*o.MarginM) / o.CellMM)
	if cols < 1 || rows < 1 {
		return fmt.Errorf("sel terlalu besar untuk A4")
	}
	perPage := cols * rows

	pdf := fpdf.New("P", "mm", "A4", "")
	pdf.SetAutoPageBreak(false, 0)
	pdf.SetDrawColor(200, 200, 200)
	pdf.SetLineWidth(0.1)

	for i, it := range items {
		if i%perPage == 0 {
			pdf.AddPage()
		}
		idx := i % perPage
		x := o.MarginM + float64(idx%cols)*o.CellMM
		y := o.MarginM + float64(idx/cols)*o.CellMM

		png, err := qrcode.Encode(it.Content, qrcode.High, 512)
		if err != nil {
			return err
		}
		name := fmt.Sprintf("qr%d", i)
		pdf.RegisterImageOptionsReader(name, fpdf.ImageOptions{ImageType: "PNG"}, bytes.NewReader(png))
		pad := (o.CellMM - o.QRMM) / 2
		pdf.ImageOptions(name, x+pad, y+1.5, o.QRMM, o.QRMM, false, fpdf.ImageOptions{ImageType: "PNG"}, 0, "")

		pdf.SetFont("Helvetica", "B", 6)
		pdf.SetTextColor(17, 47, 96) // navy brand
		ty := y + 1.5 + o.QRMM + 2.2
		pdf.SetXY(x, ty)
		pdf.CellFormat(o.CellMM, 2.5, it.Label, "", 0, "C", false, 0, "")
		if o.Title != "" {
			pdf.SetFont("Helvetica", "", 5)
			pdf.SetTextColor(90, 90, 90)
			pdf.SetXY(x, ty+2.4)
			pdf.CellFormat(o.CellMM, 2.2, o.Title, "", 0, "C", false, 0, "")
		}
		if o.Guides {
			pdf.Rect(x, y, o.CellMM, o.CellMM, "D")
		}
	}
	if len(items) == 0 {
		pdf.AddPage()
	}
	return pdf.Output(w)
}
