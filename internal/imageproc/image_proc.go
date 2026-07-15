// Package imageproc menangani decoding base64, decode format gambar
// (PNG/JPG/BMP), resize dengan menjaga aspect ratio, dithering ke
// monochrome (Floyd-Steinberg & Atkinson), dan konversi ke format
// raster bitmap ESC/POS (GS v 0).
package imageproc

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"image"
	"image/color"
	_ "image/jpeg" // register decoder JPEG
	_ "image/png"  // register decoder PNG
	"strings"

	qrcode "github.com/skip2/go-qrcode"
	_ "golang.org/x/image/bmp" // register decoder BMP
)

// Konstanta nama algoritma dithering yang didukung.
const (
	DitherNone           = "none"
	DitherFloydSteinberg = "floyd-steinberg"
	DitherAtkinson       = "atkinson"
)

// MMToDots mengonversi lebar kertas dalam mm ke jumlah dot dengan
// asumsi resolusi printer thermal standar 8 dot/mm (203 DPI).
//
//	58mm -> 384 dot ; 80mm -> 576 dot
func MMToDots(widthMM int) int {
	switch widthMM {
	case 58:
		return 384
	case 80:
		return 576
	default:
		return widthMM * 8
	}
}

// DecodeBase64Image men-decode string base64 menjadi image.Image.
// Mendukung prefix data URL ("data:image/png;base64,..."). Format
// gambar yang didukung: PNG, JPEG, BMP (lewat blank import di atas).
func DecodeBase64Image(b64 string) (image.Image, string, error) {
	cleaned := stripDataURLPrefix(strings.TrimSpace(b64))
	if cleaned == "" {
		return nil, "", fmt.Errorf("input base64 kosong")
	}
	raw, err := base64.StdEncoding.DecodeString(cleaned)
	if err != nil {
		// Coba juga base64 URL-safe sebagai fallback.
		raw, err = base64.URLEncoding.DecodeString(cleaned)
		if err != nil {
			return nil, "", fmt.Errorf("base64 tidak valid: %w", err)
		}
	}
	img, format, err := image.Decode(bytes.NewReader(raw))
	if err != nil {
		return nil, "", fmt.Errorf("gagal decode gambar: %w", err)
	}
	return img, format, nil
}

// stripDataURLPrefix menghapus prefix "data:image/...;base64," bila ada
// agar decoder base64 menerima payload murni.
func stripDataURLPrefix(s string) string {
	if idx := strings.Index(s, ","); idx >= 0 && strings.HasPrefix(strings.ToLower(s), "data:") {
		return s[idx+1:]
	}
	return s
}

// ResizeToWidth mengubah ukuran gambar agar lebar = targetWidth pixel
// sambil menjaga aspect ratio. Menggunakan algoritma nearest-neighbor
// karena cukup untuk grafis sederhana yang nantinya akan di-dither.
// Hanya melakukan downscale (perkecil) jika lebar asli > targetWidth.
// Jika lebar asli <= targetWidth, gambar dikembalikan apa adanya tanpa diperbesar.
func ResizeToWidth(img image.Image, targetWidth int) image.Image {
	if targetWidth <= 0 {
		return img
	}
	bounds := img.Bounds()
	srcW := bounds.Dx()
	srcH := bounds.Dy()
	// Jika gambar sudah pas atau lebih kecil dari target, kembalikan apa adanya.
	// Ini mencegah logo kecil di-upscale menjadi selebar kertas.
	if srcW <= targetWidth {
		return img
	}
	scale := float64(targetWidth) / float64(srcW)
	targetHeight := int(float64(srcH) * scale)
	if targetHeight < 1 {
		targetHeight = 1
	}
	dst := image.NewGray(image.Rect(0, 0, targetWidth, targetHeight))
	for y := 0; y < targetHeight; y++ {
		// Hitung baris sumber yang sesuai.
		srcY := int(float64(y) / scale)
		if srcY >= srcH {
			srcY = srcH - 1
		}
		for x := 0; x < targetWidth; x++ {
			srcX := int(float64(x) / scale)
			if srcX >= srcW {
				srcX = srcW - 1
			}
			c := color.GrayModel.Convert(img.At(bounds.Min.X+srcX, bounds.Min.Y+srcY)).(color.Gray)
			dst.SetGray(x, y, c)
		}
	}
	return dst
}

// EmbedAligned menempatkan gambar src ke dalam canvas baru selebar
// canvasWidth pixel dengan latar belakang putih, sesuai posisi yang
// diberikan ("left", "center", atau "right"). Hasilnya selalu
// selebar canvasWidth sehingga raster bitmap yang dihasilkan akan
// benar-benar ter-posisi sesuai pilihan pengguna — penting karena
// perintah ESC/POS ESC a (Align) tidak berlaku untuk GS v 0 raster.
//
// Jika src sudah selebar (atau lebih lebar dari) canvasWidth,
// fungsi ini mengembalikan src tanpa perubahan.
func EmbedAligned(src image.Image, canvasWidth int, position string) image.Image {
	bounds := src.Bounds()
	srcW := bounds.Dx()
	srcH := bounds.Dy()
	if canvasWidth <= 0 || srcW >= canvasWidth {
		return src
	}

	var offsetX int
	switch strings.ToLower(strings.TrimSpace(position)) {
	case "left":
		offsetX = 0
	case "right":
		offsetX = canvasWidth - srcW
	default: // "center" dan nilai lain
		offsetX = (canvasWidth - srcW) / 2
	}

	// Buat canvas putih penuh selebar kertas.
	canvas := image.NewGray(image.Rect(0, 0, canvasWidth, srcH))
	// Isi putih (255) secara default sudah dilakukan oleh NewGray yang zero-init,
	// namun NewGray mengisi dengan 0 (hitam). Isi eksplisit dengan 255.
	for i := range canvas.Pix {
		canvas.Pix[i] = 255
	}

	// Salin pixel src ke posisi yang tepat.
	for y := 0; y < srcH; y++ {
		for x := 0; x < srcW; x++ {
			c := color.GrayModel.Convert(src.At(bounds.Min.X+x, bounds.Min.Y+y)).(color.Gray)
			canvas.SetGray(offsetX+x, y, c)
		}
	}
	return canvas
}

// toGray mengonversi image.Image apa pun ke *image.Gray penuh dengan
// koordinat dimulai dari (0,0). Disiapkan supaya dithering bisa
// memodifikasi pixel secara in-place.
func toGray(img image.Image) *image.Gray {
	if g, ok := img.(*image.Gray); ok && g.Bounds().Min.X == 0 && g.Bounds().Min.Y == 0 {
		// Salin agar mutasi tidak berimbas ke gambar asli.
		dup := image.NewGray(g.Bounds())
		copy(dup.Pix, g.Pix)
		return dup
	}
	bounds := img.Bounds()
	dst := image.NewGray(image.Rect(0, 0, bounds.Dx(), bounds.Dy()))
	for y := 0; y < bounds.Dy(); y++ {
		for x := 0; x < bounds.Dx(); x++ {
			c := color.GrayModel.Convert(img.At(bounds.Min.X+x, bounds.Min.Y+y)).(color.Gray)
			dst.SetGray(x, y, c)
		}
	}
	return dst
}

// Threshold mengubah gambar grayscale menjadi 1-bit (hitam/putih murni)
// tanpa dithering. Threshold di-set 128.
func Threshold(g *image.Gray) *image.Gray {
	out := image.NewGray(g.Bounds())
	for i, v := range g.Pix {
		if v < 128 {
			out.Pix[i] = 0
		} else {
			out.Pix[i] = 255
		}
	}
	return out
}

// FloydSteinberg menerapkan algoritma dithering Floyd-Steinberg
// pada gambar grayscale dan menghasilkan gambar 1-bit.
//
// Distribusi error tetangga (dx, dy, weight):
//
//	(+1, 0, 7/16) (-1,+1, 3/16) ( 0,+1, 5/16) (+1,+1, 1/16)
func FloydSteinberg(g *image.Gray) *image.Gray {
	w := g.Bounds().Dx()
	h := g.Bounds().Dy()
	// Salin pixel ke buffer int agar bisa menampung nilai negatif/over 255 saat error diffuse.
	buf := make([]int, w*h)
	for i, v := range g.Pix {
		buf[i] = int(v)
	}
	out := image.NewGray(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			idx := y*w + x
			old := buf[idx]
			var newPx int
			if old < 128 {
				newPx = 0
			} else {
				newPx = 255
			}
			out.Pix[idx] = uint8(newPx)
			err := old - newPx
			if x+1 < w {
				buf[idx+1] += err * 7 / 16
			}
			if y+1 < h {
				if x > 0 {
					buf[idx+w-1] += err * 3 / 16
				}
				buf[idx+w] += err * 5 / 16
				if x+1 < w {
					buf[idx+w+1] += err * 1 / 16
				}
			}
		}
	}
	return out
}

// Atkinson menerapkan algoritma dithering Atkinson (mirip Floyd-Steinberg
// tetapi hanya mendistribusikan 6/8 error sehingga gambar terlihat
// lebih kontras dan "bersih").
//
// Pola distribusi (dx, dy, weight=1/8 each, total 6/8):
//
//	(+1,0) (+2,0)
//	(-1,+1) (0,+1) (+1,+1)
//	(0,+2)
func Atkinson(g *image.Gray) *image.Gray {
	w := g.Bounds().Dx()
	h := g.Bounds().Dy()
	buf := make([]int, w*h)
	for i, v := range g.Pix {
		buf[i] = int(v)
	}
	out := image.NewGray(image.Rect(0, 0, w, h))
	offsets := []struct{ dx, dy int }{
		{1, 0}, {2, 0},
		{-1, 1}, {0, 1}, {1, 1},
		{0, 2},
	}
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			idx := y*w + x
			old := buf[idx]
			var newPx int
			if old < 128 {
				newPx = 0
			} else {
				newPx = 255
			}
			out.Pix[idx] = uint8(newPx)
			diff := (old - newPx) / 8
			for _, off := range offsets {
				nx := x + off.dx
				ny := y + off.dy
				if nx < 0 || nx >= w || ny < 0 || ny >= h {
					continue
				}
				buf[ny*w+nx] += diff
			}
		}
	}
	return out
}

// PrepareMonochrome menggabungkan langkah toGray + resize + dithering
// sesuai metode yang dipilih dan mengembalikan *image.Gray 1-bit
// (hanya 0 atau 255 pada Pix) siap dikonversi ke raster.
func PrepareMonochrome(src image.Image, targetWidth int, dither string) *image.Gray {
	resized := ResizeToWidth(src, targetWidth)
	g := toGray(resized)
	switch strings.ToLower(strings.TrimSpace(dither)) {
	case DitherFloydSteinberg, "":
		return FloydSteinberg(g)
	case DitherAtkinson:
		return Atkinson(g)
	case DitherNone:
		return Threshold(g)
	default:
		return FloydSteinberg(g)
	}
}

// PrepareMonochromeAligned sama dengan PrepareMonochrome tetapi setelah
// resize, gambar di-embed ke canvas selebar canvasWidth dengan posisi
// ("left"/"center"/"right") sebelum di-dither. Digunakan untuk logo
// agar posisi center/left/right benar-benar terkodekan dalam data raster,
// bukan mengandalkan perintah ESC a yang tidak berlaku untuk GS v 0.
func PrepareMonochromeAligned(src image.Image, canvasWidth int, position string, dither string) *image.Gray {
	resized := ResizeToWidth(src, canvasWidth)
	aligned := EmbedAligned(resized, canvasWidth, position)
	g := toGray(aligned)
	switch strings.ToLower(strings.TrimSpace(dither)) {
	case DitherFloydSteinberg, "":
		return FloydSteinberg(g)
	case DitherAtkinson:
		return Atkinson(g)
	case DitherNone:
		return Threshold(g)
	default:
		return FloydSteinberg(g)
	}
}

// ToRasterBitmap mengubah gambar 1-bit (*image.Gray dengan nilai 0/255)
// ke format byte yang dipakai oleh perintah ESC/POS GS v 0.
//
// Setiap baris dikemas MSB-first; setiap bit 1 = pixel hitam.
// Mengembalikan slice byte, jumlah byte per baris, dan tinggi pixel.
func ToRasterBitmap(g *image.Gray) (data []byte, widthBytes, heightPx int) {
	w := g.Bounds().Dx()
	h := g.Bounds().Dy()
	widthBytes = (w + 7) / 8
	data = make([]byte, widthBytes*h)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			// Pixel gelap (< 128) dianggap hitam.
			if g.Pix[y*w+x] < 128 {
				byteIdx := y*widthBytes + x/8
				bitIdx := 7 - (x % 8)
				data[byteIdx] |= 1 << bitIdx
			}
		}
	}
	return data, widthBytes, h
}

// ResizeToWidthForce mengubah ukuran gambar agar lebar = targetWidth pixel
// dengan menjaga aspect ratio, baik melakukan downscale maupun upscale.
func ResizeToWidthForce(img image.Image, targetWidth int) image.Image {
	if targetWidth <= 0 {
		return img
	}
	bounds := img.Bounds()
	srcW := bounds.Dx()
	srcH := bounds.Dy()
	if srcW == targetWidth {
		return img
	}
	scale := float64(targetWidth) / float64(srcW)
	targetHeight := int(float64(srcH) * scale)
	if targetHeight < 1 {
		targetHeight = 1
	}
	dst := image.NewGray(image.Rect(0, 0, targetWidth, targetHeight))
	for y := 0; y < targetHeight; y++ {
		srcY := int(float64(y) / scale)
		if srcY >= srcH {
			srcY = srcH - 1
		}
		for x := 0; x < targetWidth; x++ {
			srcX := int(float64(x) / scale)
			if srcX >= srcW {
				srcX = srcW - 1
			}
			c := color.GrayModel.Convert(img.At(bounds.Min.X+srcX, bounds.Min.Y+srcY)).(color.Gray)
			dst.SetGray(x, y, c)
		}
	}
	return dst
}

// PrepareMonochromeForce menggabungkan langkah toGray + force resize + dithering
// sesuai metode yang dipilih dan mengembalikan *image.Gray 1-bit.
func PrepareMonochromeForce(src image.Image, targetWidth int, dither string) *image.Gray {
	resized := ResizeToWidthForce(src, targetWidth)
	g := toGray(resized)
	switch strings.ToLower(strings.TrimSpace(dither)) {
	case DitherFloydSteinberg, "":
		return FloydSteinberg(g)
	case DitherAtkinson:
		return Atkinson(g)
	case DitherNone:
		return Threshold(g)
	default:
		return FloydSteinberg(g)
	}
}

// GenerateQRCode membuat gambar QR code dari string konten (URL/teks).
// sizePx menentukan dimensi output PNG sebelum dikonversi ke raster ESC/POS.
func GenerateQRCode(content string, sizePx int) (image.Image, error) {
	if strings.TrimSpace(content) == "" {
		return nil, fmt.Errorf("konten QR kosong")
	}
	if sizePx < 50 {
		sizePx = 200
	}
	if sizePx > 500 {
		sizePx = 500
	}
	q, err := qrcode.New(content, qrcode.Medium)
	if err != nil {
		return nil, fmt.Errorf("gagal generate QR: %w", err)
	}
	pngBytes, err := q.PNG(sizePx)
	if err != nil {
		return nil, fmt.Errorf("gagal render QR PNG: %w", err)
	}
	img, _, err := image.Decode(bytes.NewReader(pngBytes))
	if err != nil {
		return nil, fmt.Errorf("gagal decode QR PNG: %w", err)
	}
	return img, nil
}

