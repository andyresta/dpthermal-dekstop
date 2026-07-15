// Package escpos menyediakan builder perintah ESC/POS untuk printer
// thermal. Hanya menggunakan Go standard library; output berupa
// []byte yang siap dikirim ke printer (via spooler RAW atau RFCOMM).
package escpos

import (
	"bytes"
	"fmt"
)

// Konstanta byte kontrol ESC/POS umum.
const (
	ESC = 0x1B
	GS  = 0x1D
	LF  = 0x0A
	CR  = 0x0D
)

// Konstanta posisi alignment teks.
const (
	AlignLeft   = 0
	AlignCenter = 1
	AlignRight  = 2
)

// Builder memudahkan pembentukan stream byte ESC/POS secara berurutan.
// Tidak thread-safe; gunakan satu Builder per job pencetakan.
type Builder struct {
	buf bytes.Buffer
}

// New membuat Builder baru yang siap dipakai. Tidak otomatis melakukan
// Init(); pemanggil dapat memanggil Init secara eksplisit jika perlu.
func New() *Builder {
	return &Builder{}
}

// Bytes mengembalikan seluruh byte yang sudah dibangun.
func (b *Builder) Bytes() []byte {
	return b.buf.Bytes()
}

// Write menyalin byte mentah apa adanya ke buffer (tanpa transformasi).
func (b *Builder) Write(p []byte) {
	b.buf.Write(p)
}

// Init mengirim perintah ESC @ untuk reset printer ke kondisi awal.
// Selalu disarankan dipanggil di awal stream.
func (b *Builder) Init() *Builder {
	b.buf.Write([]byte{ESC, '@'})
	return b
}

// Align mengatur alignment teks/gambar berikutnya.
// pos: AlignLeft (0), AlignCenter (1), AlignRight (2).
func (b *Builder) Align(pos byte) *Builder {
	if pos > 2 {
		pos = 0
	}
	b.buf.Write([]byte{ESC, 'a', pos})
	return b
}

// SetMode mengatur mode cetak (bold, underline, double-height,
// double-width) via perintah ESC ! n. Bit-bit n:
//   - bit 3 = bold
//   - bit 4 = double height
//   - bit 5 = double width
//   - bit 7 = underline
func (b *Builder) SetMode(bold, underline, doubleHeight, doubleWidth bool) *Builder {
	var n byte
	if bold {
		n |= 1 << 3
	}
	if doubleHeight {
		n |= 1 << 4
	}
	if doubleWidth {
		n |= 1 << 5
	}
	if underline {
		n |= 1 << 7
	}
	b.buf.Write([]byte{ESC, '!', n})
	return b
}

// SetBold meng-aktifkan/menonaktifkan bold via ESC E n.
func (b *Builder) SetBold(on bool) *Builder {
	v := byte(0)
	if on {
		v = 1
	}
	b.buf.Write([]byte{ESC, 'E', v})
	return b
}

// SetUnderline mengatur ketebalan garis bawah (0=off, 1=tipis, 2=tebal).
func (b *Builder) SetUnderline(level byte) *Builder {
	if level > 2 {
		level = 0
	}
	b.buf.Write([]byte{ESC, '-', level})
	return b
}

// SetSize mengatur skala karakter via GS ! n (lebar dan tinggi bersama).
// size 1 = normal, 2 = 2x, hingga maksimum 8x — selaras dengan MaxThermal Android.
func (b *Builder) SetSize(size int) *Builder {
	if size < 1 {
		size = 1
	}
	if size > 8 {
		size = 8
	}
	s := byte(size - 1)
	n := (s << 4) | s
	b.buf.Write([]byte{GS, '!', n})
	return b
}

// ResetFormatting mengembalikan ukuran, bold, underline, dan alignment ke default.
func (b *Builder) ResetFormatting() *Builder {
	b.SetSize(1)
	b.SetBold(false)
	b.SetUnderline(0)
	b.Align(AlignLeft)
	return b
}

// Text menulis string disusul line feed (LF). Encoding harus sudah
// sesuai dengan codepage printer (default umumnya CP437 atau UTF-8
// pada printer modern).
func (b *Builder) Text(s string) *Builder {
	b.buf.WriteString(s)
	return b
}

// LineFeed menambahkan satu karakter LF.
func (b *Builder) LineFeed() *Builder {
	b.buf.WriteByte(LF)
	return b
}

// FeedLines memajukan kertas sebanyak n baris menggunakan ESC d n.
func (b *Builder) FeedLines(n byte) *Builder {
	b.buf.Write([]byte{ESC, 'd', n})
	return b
}

// CutPaper mengirim perintah pemotong kertas.
//
//	full=true  -> full cut (GS V 0)
//	full=false -> partial cut (GS V 1)
func (b *Builder) CutPaper(full bool) *Builder {
	mode := byte(1)
	if full {
		mode = 0
	}
	b.buf.Write([]byte{GS, 'V', mode})
	return b
}

// SetLeftMargin mengatur batas kiri area cetak menggunakan perintah GS L.
// Perintah ini memengaruhi posisi horizontal untuk teks, ESC * (bit image),
// maupun GS v 0 (raster bit image) — berbeda dengan ESC a yang TIDAK
// memengaruhi GS v 0 pada printer Epson TM series.
//
// dots adalah jumlah dot (piksel) dari tepi kiri kertas.
// Untuk reset, panggil SetLeftMargin(0).
func (b *Builder) SetLeftMargin(dots int) *Builder {
	if dots < 0 {
		dots = 0
	}
	nL := byte(dots & 0xFF)
	nH := byte((dots >> 8) & 0xFF)
	b.buf.Write([]byte{GS, 'L', nL, nH})
	return b
}

// PrintRasterBitmap mengirim gambar monochrome dalam format
// "GS v 0" (Print raster bit image).
//
//	widthBytes adalah jumlah byte per baris (pixel width / 8, bulat ke atas).
//	heightPx   adalah jumlah baris pixel.
//	data       adalah bitmap MSB-first: 1 bit = pixel hitam, 0 = putih.
//
// Total panjang data harus sama dengan widthBytes * heightPx.
func (b *Builder) PrintRasterBitmap(data []byte, widthBytes, heightPx int) error {
	expected := widthBytes * heightPx
	if len(data) != expected {
		return fmt.Errorf("ukuran data raster tidak sesuai: butuh %d byte, dapat %d", expected, len(data))
	}
	if widthBytes <= 0 || heightPx <= 0 {
		return fmt.Errorf("widthBytes dan heightPx harus > 0")
	}
	if widthBytes > 0xFFFF || heightPx > 0xFFFF {
		return fmt.Errorf("dimensi raster di luar batas 16-bit (max 65535)")
	}

	xL := byte(widthBytes & 0xFF)
	xH := byte((widthBytes >> 8) & 0xFF)
	yL := byte(heightPx & 0xFF)
	yH := byte((heightPx >> 8) & 0xFF)

	// GS v 0 m xL xH yL yH d1 .. dn ; m=0 (normal mode).
	b.buf.Write([]byte{GS, 'v', '0', 0x00, xL, xH, yL, yH})
	b.buf.Write(data)
	return nil
}
