// Package printer menangani deteksi printer fisik (spooler OS dan
// Bluetooth) serta pengiriman job pencetakan ke printer tujuan.
// Implementasi cross-platform untuk Windows / Linux / macOS.
package printer

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"image"

	"printbridge/internal/config"
	"printbridge/internal/escpos"
	"printbridge/internal/imageproc"
)

// Konstanta tipe printer untuk hasil deteksi.
const (
	TypeSpooler   = "spooler"
	TypeBluetooth = "bluetooth"
)

// Konstanta status printer.
const (
	StatusReady   = "ready"
	StatusOffline = "offline"
)

// Info mendeskripsikan satu printer yang terdeteksi pada sistem.
type Info struct {
	Name   string `json:"name"`
	Type   string `json:"type"`
	Status string `json:"status"`
	// Port adalah identifier transport untuk printer Bluetooth
	// (contoh "COM5" di Windows atau "/dev/rfcomm0" di Linux).
	// Untuk printer spooler, field ini bisa kosong.
	Port string `json:"port,omitempty"`
}

// minRefreshInterval adalah jeda minimum antar dua refresh penuh.
// Permintaan refresh yang datang lebih cepat dari interval ini langsung
// mengembalikan data cache tanpa menjalankan ulang perintah OS.
const minRefreshInterval = 3 * time.Second

// Manager menyimpan hasil deteksi printer terbaru dan menyediakan
// method dispatch (PrintText/PrintImage) yang aman dipakai concurrent.
type Manager struct {
	mu            sync.RWMutex
	printers      []Info
	lastRefreshed time.Time // kapan Refresh() terakhir dijalankan

	// printMu memastikan hanya satu job cetak yang dikirim ke printer
	// pada satu waktu. Ini mencegah:
	//   1. Race antara dua job yang saling menimpa buffer printer.
	//   2. Tumpukan goroutine zombie saat driver printer hang.
	//   3. ESC @ dari job baru yang membatalkan data job sebelumnya.
	// Caller PrintText/PrintImage akan memblok di sini sampai job
	// sebelumnya selesai (atau timeout context mereka habis).
	printMu sync.Mutex
}

// NewManager membuat Manager kosong; daftar printer harus diisi
// melalui Refresh() (biasanya dijalankan di goroutine background saat startup).
func NewManager() *Manager {
	return &Manager{printers: []Info{}}
}

// detectionResult adalah container hasil satu goroutine deteksi.
type detectionResult struct {
	printers []Info
	err      error
}

// Refresh melakukan deteksi ulang seluruh printer (spooler + Bluetooth)
// secara PARALEL menggunakan dua goroutine, sehingga total waktu tunggu
// adalah max(waktu_spooler, waktu_bluetooth) bukan penjumlahannya.
//
// Jika Refresh() sudah dipanggil kurang dari minRefreshInterval yang lalu,
// fungsi ini langsung kembali dengan data cache tanpa menjalankan ulang
// perintah OS, mencegah loading lambat akibat klik berulang.
//
// Context dipakai untuk timeout/cancel keseluruhan proses deteksi.
func (m *Manager) Refresh(ctx context.Context) error {
	// Cek apakah data cache masih cukup segar (debounce).
	m.mu.RLock()
	tooSoon := !m.lastRefreshed.IsZero() && time.Since(m.lastRefreshed) < minRefreshInterval
	m.mu.RUnlock()
	if tooSoon {
		return nil
	}

	spoolCh := make(chan detectionResult, 1)
	btCh := make(chan detectionResult, 1)

	// Jalankan deteksi spooler dan Bluetooth secara paralel.
	go func() {
		p, err := detectSpoolerPrinters(ctx)
		spoolCh <- detectionResult{p, err}
	}()
	go func() {
		p, err := detectBluetoothPrinters(ctx)
		btCh <- detectionResult{p, err}
	}()

	// Tunggu kedua goroutine selesai (atau context timeout).
	var sr, br detectionResult
	select {
	case sr = <-spoolCh:
	case <-ctx.Done():
		return ctx.Err()
	}
	select {
	case br = <-btCh:
	case <-ctx.Done():
		return ctx.Err()
	}

	combined := make([]Info, 0, len(sr.printers)+len(br.printers))
	combined = append(combined, sr.printers...)
	combined = append(combined, br.printers...)

	m.mu.Lock()
	m.printers = combined
	m.lastRefreshed = time.Now()
	m.mu.Unlock()

	// Gabungkan error tanpa membatalkan jika salah satu sumber gagal.
	switch {
	case sr.err != nil && br.err != nil:
		return fmt.Errorf("deteksi spooler & bluetooth gagal: %v ; %v", sr.err, br.err)
	case sr.err != nil:
		return fmt.Errorf("deteksi spooler gagal: %w", sr.err)
	case br.err != nil:
		return fmt.Errorf("deteksi bluetooth gagal: %w", br.err)
	}
	return nil
}

// List mengembalikan salinan daftar printer yang sudah terdeteksi.
func (m *Manager) List() []Info {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]Info, len(m.printers))
	copy(out, m.printers)
	return out
}

// Find mencari printer berdasarkan nama (case-insensitive).
// Mengembalikan info dan true bila ditemukan.
func (m *Manager) Find(name string) (Info, bool) {
	target := strings.ToLower(strings.TrimSpace(name))
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, p := range m.printers {
		if strings.ToLower(p.Name) == target {
			return p, true
		}
	}
	return Info{}, false
}

// TextJob merepresentasikan permintaan cetak teks dari API.
type TextJob struct {
	Text         string
	LogoBitmap   *RasterImage // opsional; sudah dalam bentuk raster monochrome
	LogoPosition string       // "left" | "center" | "right"
	Copies       int
	CutPaper     bool
	Encoding     string
	// Mode menentukan cara pembentukan payload:
	//   "thermal"   → ESC/POS (default bila kosong)
	//   "dotmatrix" → plain text tanpa ESC/POS
	// Bila kosong, diambil dari config.Config.PrinterMode.
	Mode string
}

// ImageJob merepresentasikan permintaan cetak gambar dari API.
type ImageJob struct {
	Bitmap   *RasterImage
	Copies   int
	CutPaper bool
	// Mode: "thermal" didukung; "dotmatrix" akan menghasilkan error
	// karena printer dot matrix tidak mendukung bitmap raster ESC/POS.
	Mode string
}

// ReceiptItem adalah satu elemen dalam payload cetak nota terstruktur.
// Format selaras dengan MaxThermal Android POST /print/receipt.
type ReceiptItem struct {
	Type  string `json:"type"`
	Align string `json:"align"`
	Size  int    `json:"size"`
	Style string `json:"style"`
	Data  string `json:"data"`
}

// ReceiptJob merepresentasikan permintaan cetak nota dari array items.
type ReceiptJob struct {
	Items     []ReceiptItem
	Copies    int
	CutPaper  bool
	Mode      string
	WidthMM   int
	Dithering string
}

// RasterImage menampung data raster monochrome yang sudah siap dicetak
// melalui perintah ESC/POS GS v 0.
type RasterImage struct {
	Data       []byte
	WidthBytes int
	HeightPx   int
	// OffsetDots adalah jumlah dot dari tepi kiri yang harus di-set
	// via GS L (SetLeftMargin) sebelum mencetak bitmap ini, agar logo
	// muncul di posisi left/center/right yang tepat.
	// ESC a TIDAK memengaruhi GS v 0 pada Epson TM series;
	// GS L adalah satu-satunya cara yang reliable.
	OffsetDots int
}

// resolveMode menentukan mode aktif untuk sebuah job. Jika job.Mode
// sudah diisi secara eksplisit, nilai itu yang dipakai. Jika kosong,
// fallback ke cfg.PrinterMode. Ultimate default adalah "thermal".
func resolveMode(jobMode, cfgMode string) string {
	m := strings.ToLower(strings.TrimSpace(jobMode))
	if m == config.PrinterModeThermal || m == config.PrinterModeDotMatrix {
		return m
	}
	m = strings.ToLower(strings.TrimSpace(cfgMode))
	if m == config.PrinterModeThermal || m == config.PrinterModeDotMatrix {
		return m
	}
	return config.PrinterModeThermal
}

// PrintText membentuk payload cetak teks lalu mengirimnya ke printer.
//
// Mode "thermal": payload ESC/POS dengan dukungan logo raster, alignment,
// dan perintah cut paper.
//
// Mode "dotmatrix": payload teks polos (tanpa byte ESC/POS) yang sesuai
// untuk printer dot matrix maupun spooler plain-text. Logo dan cut paper
// diabaikan.
func (m *Manager) PrintText(ctx context.Context, cfg config.Config, job TextJob) error {
	// Kunci antrean secara ketat dari awal proses agar pembuatan payload dan pengiriman
	// berjalan berurutan secara FIFO (first-in-first-out).
	m.printMu.Lock()
	defer m.printMu.Unlock()

	if strings.TrimSpace(cfg.DefaultPrinter) == "" {
		return errors.New("default printer belum dikonfigurasi")
	}
	info, ok := m.Find(cfg.DefaultPrinter)
	if !ok {
		// Tetap coba kirim ke spooler sesuai nama; deteksi mungkin tertinggal.
		info = Info{Name: cfg.DefaultPrinter, Type: cfg.PrinterType, Status: StatusReady}
	}

	if job.Copies < 1 {
		job.Copies = 1
	}
	if job.Copies > 10 {
		job.Copies = 10
	}

	mode := resolveMode(job.Mode, cfg.PrinterMode)

	var payload []byte
	if mode == config.PrinterModeDotMatrix {
		// Plain text: kirim byte mentah tanpa ESC/POS apa pun.
		text := job.Text
		if !strings.HasSuffix(text, "\n") {
			text += "\n"
		}
		payload = []byte(text)
	} else {
		// Thermal (ESC/POS)
		// ESC @ (Init) dikirim di awal setiap job untuk mereset state
		// printer (alignment, font, mode). Sekarang AMAN karena semaphore
		// printMu menjamin tidak ada job lain yang sedang berjalan.
		b := escpos.New().Init()
		if job.LogoBitmap != nil {
			// GS L mengatur left margin untuk posisi cetak raster (GS v 0).
			// ESC a TIDAK memengaruhi GS v 0 pada Epson TM series.
			// OffsetDots sudah dihitung di BuildLogoRaster sesuai posisi.
			b.SetLeftMargin(job.LogoBitmap.OffsetDots)
			if err := b.PrintRasterBitmap(job.LogoBitmap.Data, job.LogoBitmap.WidthBytes, job.LogoBitmap.HeightPx); err != nil {
				return fmt.Errorf("logo raster invalid: %w", err)
			}
			// Reset seluruh state printer dan margin ke default bawaan pabrik
			// agar teks di bawahnya tidak terpotong di tepi kiri.
			b.Init()
			b.LineFeed()
		}
		b.Align(escpos.AlignLeft)
		text := strings.TrimLeft(job.Text, "\n\r")
		if !strings.HasSuffix(text, "\n") {
			text += "\n"
		}
		b.Text(text)
		if job.CutPaper {
			// Feed 3 baris agar teks/footer sepenuhnya melewati pisau cutter sebelum dipotong
			b.FeedLines(3)
			b.CutPaper(true)
		} else {
			// Tanpa cut paper, feed 2 baris agar tulisan terakhir melewati tear bar sobekan
			b.FeedLines(2)
		}
		payload = b.Bytes()
	}

	for i := 0; i < job.Copies; i++ {
		if err := sendToPrinter(ctx, info, payload); err != nil {
			return fmt.Errorf("gagal kirim ke printer (copy %d): %w", i+1, err)
		}
	}

	// Jeda Keamanan Fisik: Beri jeda fisik agar motor printer menyelesaikan cetakan
	// dan pisau cutter menyelesaikan pemotongan sebelum lock dibuka untuk job berikutnya.
	time.Sleep(800 * time.Millisecond)
	return nil
}

// PrintImage mengirim gambar raster ESC/POS ke printer thermal.
// Mode "dotmatrix" tidak didukung karena printer dot matrix tidak
// memiliki perintah cetak bitmap raster; akan dikembalikan error 400
// agar UI dapat menampilkan pesan yang jelas kepada pengguna.
func (m *Manager) PrintImage(ctx context.Context, cfg config.Config, job ImageJob) error {
	// Kunci antrean secara ketat dari awal proses agar pembuatan payload dan pengiriman
	// berjalan berurutan secara FIFO (first-in-first-out).
	m.printMu.Lock()
	defer m.printMu.Unlock()

	if strings.TrimSpace(cfg.DefaultPrinter) == "" {
		return errors.New("default printer belum dikonfigurasi")
	}
	if job.Bitmap == nil {
		return errors.New("bitmap kosong")
	}

	mode := resolveMode(job.Mode, cfg.PrinterMode)
	if mode == config.PrinterModeDotMatrix {
		return errors.New("cetak gambar tidak didukung pada mode Dot Matrix / Plain Text; " +
			"pilih mode Thermal (ESC/POS) untuk mencetak gambar")
	}

	info, ok := m.Find(cfg.DefaultPrinter)
	if !ok {
		info = Info{Name: cfg.DefaultPrinter, Type: cfg.PrinterType, Status: StatusReady}
	}
	if job.Copies < 1 {
		job.Copies = 1
	}
	if job.Copies > 10 {
		job.Copies = 10
	}

	// Menggunakan GS L (Set Left Margin) untuk centering gambar secara reliable,
	// karena perintah ESC a Align tidak memengaruhi GS v 0 pada printer Epson TM.
	b := escpos.New().Init()
	b.SetLeftMargin(job.Bitmap.OffsetDots)
	if err := b.PrintRasterBitmap(job.Bitmap.Data, job.Bitmap.WidthBytes, job.Bitmap.HeightPx); err != nil {
		return fmt.Errorf("bitmap raster invalid: %w", err)
	}
	b.Init() // Reset ke default margin bawaan printer setelah gambar tercetak
	if job.CutPaper {
		// Feed 3 baris agar gambar/footer sepenuhnya melewati pisau cutter sebelum dipotong
		b.FeedLines(3)
		b.CutPaper(true)
	} else {
		// Tanpa cut paper, feed 2 baris agar gambar terakhir melewati tear bar sobekan
		b.FeedLines(2)
	}

	payload := b.Bytes()

	for i := 0; i < job.Copies; i++ {
		if err := sendToPrinter(ctx, info, payload); err != nil {
			return fmt.Errorf("gagal kirim ke printer (copy %d): %w", i+1, err)
		}
	}

	// Jeda Keamanan Fisik: Cetak gambar memerlukan waktu gerak motor mekanik
	// yang lebih lama dibanding teks biasa. Beri jeda 1.5 detik agar cetakan selesai sepenuhnya.
	time.Sleep(1500 * time.Millisecond)
	return nil
}

// PrintReceipt mencetak nota/struk berdasarkan array item terstruktur.
// Mendukung type: text, qr, image, line, feed — selaras dengan MaxThermal Android.
func (m *Manager) PrintReceipt(ctx context.Context, cfg config.Config, job ReceiptJob) error {
	m.printMu.Lock()
	defer m.printMu.Unlock()

	if strings.TrimSpace(cfg.DefaultPrinter) == "" {
		return errors.New("default printer belum dikonfigurasi")
	}
	if len(job.Items) == 0 {
		return errors.New("field 'items' tidak ditemukan atau kosong")
	}

	mode := resolveMode(job.Mode, cfg.PrinterMode)
	widthMM := job.WidthMM
	if widthMM != 58 && widthMM != 80 {
		widthMM = cfg.DefaultPaperWidthMM
	}
	if widthMM != 58 && widthMM != 80 {
		widthMM = 58
	}

	if mode == config.PrinterModeDotMatrix {
		// QR dan Image akan dirender sebagai text [QR] / [IMAGE] di buildReceiptPlainText
	}

	info, ok := m.Find(cfg.DefaultPrinter)
	if !ok {
		info = Info{Name: cfg.DefaultPrinter, Type: cfg.PrinterType, Status: StatusReady}
	}

	copies := job.Copies
	if copies < 1 {
		copies = 1
	}
	if copies > 10 {
		copies = 10
	}

	dither := strings.TrimSpace(job.Dithering)
	if dither == "" {
		dither = imageproc.DitherFloydSteinberg
	}

	charsPerLine := charsPerLineForPaper(widthMM)

	payload, err := buildReceiptPayload(job.Items, mode, widthMM, charsPerLine, dither, job.CutPaper)
	if err != nil {
		return err
	}

	for i := 0; i < copies; i++ {
		if err := sendToPrinter(ctx, info, payload); err != nil {
			return fmt.Errorf("gagal kirim ke printer (copy %d): %w", i+1, err)
		}
	}

	time.Sleep(1200 * time.Millisecond)
	return nil
}

// buildReceiptPayload membentuk byte ESC/POS atau plain text dari array item receipt.
func buildReceiptPayload(items []ReceiptItem, mode string, widthMM, charsPerLine int, dither string, cutPaper bool) ([]byte, error) {
	if mode == config.PrinterModeDotMatrix {
		return buildReceiptPlainText(items, charsPerLine), nil
	}

	b := escpos.New().Init()
	for _, item := range items {
		switch strings.ToLower(strings.TrimSpace(item.Type)) {
		case "text":
			appendReceiptTextItem(b, item, charsPerLine)
		case "line":
			appendReceiptLineItem(b, item, charsPerLine)
		case "feed":
			appendReceiptFeedItem(b, item)
		case "qr":
			raster, err := BuildReceiptQRRaster(item.Data, item.Size, widthMM, item.Align)
			if err != nil {
				return nil, err
			}
			if raster != nil {
				if err := appendReceiptRaster(b, raster); err != nil {
					return nil, err
				}
			}
		case "image":
			if strings.TrimSpace(item.Data) == "" {
				continue
			}
			raster, err := BuildReceiptImageRaster(item.Data, widthMM, item.Align, dither)
			if err != nil {
				return nil, err
			}
			if err := appendReceiptRaster(b, raster); err != nil {
				return nil, err
			}
		}
	}

	b.ResetFormatting()
	if cutPaper {
		b.FeedLines(3)
		b.CutPaper(true)
	} else {
		b.FeedLines(2)
	}
	return b.Bytes(), nil
}

// buildReceiptPlainText membentuk output plain text untuk mode dot matrix.
func buildReceiptPlainText(items []ReceiptItem, charsPerLine int) []byte {
	var buf strings.Builder
	for _, item := range items {
		switch strings.ToLower(strings.TrimSpace(item.Type)) {
		case "text":
			data := item.Data
			if strings.TrimSpace(data) == "" {
				continue
			}
			size := item.Size
			if size < 1 {
				size = 1
			}
			if size > 8 {
				size = 8
			}
			data = formatReceiptTextData(data, item.Align, size, charsPerLine)
			if !strings.HasSuffix(data, "\n") {
				data += "\n"
			}
			buf.WriteString(data)
		case "line":
			c := lineChar(item.Style)
			buf.WriteString(strings.Repeat(string(c), charsPerLine))
			buf.WriteByte('\n')
		case "feed":
			n := parseFeedLines(item.Data)
			for i := 0; i < n; i++ {
				buf.WriteByte('\n')
			}
		case "qr":
			data := formatReceiptTextData("[QR]", "center", 1, charsPerLine)
			if !strings.HasSuffix(data, "\n") {
				data += "\n"
			}
			buf.WriteString(data)
		case "image":
			data := formatReceiptTextData("[IMAGE]", "center", 1, charsPerLine)
			if !strings.HasSuffix(data, "\n") {
				data += "\n"
			}
			buf.WriteString(data)
		}
	}
	return []byte(buf.String())
}

// appendReceiptTextItem menambahkan item teks dengan formatting ESC/POS.
// Untuk align center/right, padding spasi software dipakai agar posisi teks
// mengikuti width_mm (charsPerLine), bukan lebar print head fisik printer.
func appendReceiptTextItem(b *escpos.Builder, item ReceiptItem, charsPerLine int) {
	data := item.Data
	if strings.TrimSpace(data) == "" {
		return
	}
	size := item.Size
	if size < 1 {
		size = 1
	}
	if size > 8 {
		size = 8
	}
	align := strings.ToLower(strings.TrimSpace(item.Align))
	if align == "center" || align == "right" {
		data = formatReceiptTextData(data, align, size, charsPerLine)
	}
	b.Align(escpos.AlignLeft)
	style := strings.ToLower(item.Style)
	b.SetSize(size)
	b.SetBold(strings.Contains(style, "bold"))
	underline := byte(0)
	if strings.Contains(style, "underline") {
		underline = 1
	}
	b.SetUnderline(underline)
	if !strings.HasSuffix(data, "\n") {
		data += "\n"
	}
	b.Text(data)
	b.SetSize(1)
	b.SetBold(false)
	b.SetUnderline(0)
}

// effectiveReceiptTextCols menghitung jumlah kolom teks efektif per baris
// setelah skala ukuran karakter (GS !). Contoh: 32 char @ 58mm dengan size 2 → 16 kolom.
func effectiveReceiptTextCols(charsPerLine, size int) int {
	if size < 1 {
		size = 1
	}
	if size > 8 {
		size = 8
	}
	cols := charsPerLine / size
	if cols < 1 {
		cols = 1
	}
	return cols
}

// padReceiptTextLine menambahkan spasi kiri agar teks tampil center/right
// dalam lebar kolom efektif. Jika teks lebih panjang dari effectiveCols,
// baris dikembalikan tanpa padding (overflow ke kiri, aman di mayoritas printer).
func padReceiptTextLine(line, align string, effectiveCols int) string {
	lineLen := len(line)
	if lineLen >= effectiveCols {
		return line
	}
	switch strings.ToLower(strings.TrimSpace(align)) {
	case "center":
		pad := (effectiveCols - lineLen) / 2
		return strings.Repeat(" ", pad) + line
	case "right":
		pad := effectiveCols - lineLen
		return strings.Repeat(" ", pad) + line
	default:
		return line
	}
}

// formatReceiptTextData menerapkan padding per-baris untuk center/right berdasarkan
// charsPerLine dan size agar alignment mengikuti width_mm request, bukan lebar
// print head fisik printer (kompatibel mayoritas ESC/POS tanpa perintah GS W).
func formatReceiptTextData(data, align string, size, charsPerLine int) string {
	align = strings.ToLower(strings.TrimSpace(align))
	if align != "center" && align != "right" {
		return data
	}
	effectiveCols := effectiveReceiptTextCols(charsPerLine, size)
	hasTrailingNL := strings.HasSuffix(data, "\n")
	lines := strings.Split(data, "\n")
	if hasTrailingNL && len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	var b strings.Builder
	for i, line := range lines {
		if i > 0 {
			b.WriteByte('\n')
		}
		b.WriteString(padReceiptTextLine(line, align, effectiveCols))
	}
	if hasTrailingNL {
		b.WriteByte('\n')
	}
	return b.String()
}

// appendReceiptLineItem menambahkan garis pemisah horizontal.
func appendReceiptLineItem(b *escpos.Builder, item ReceiptItem, charsPerLine int) {
	c := lineChar(item.Style)
	line := strings.Repeat(string(c), charsPerLine) + "\n"
	b.Text(line)
}

// appendReceiptFeedItem menambahkan baris kosong via ESC d n.
func appendReceiptFeedItem(b *escpos.Builder, item ReceiptItem) {
	n := parseFeedLines(item.Data)
	b.FeedLines(byte(n))
}

// appendReceiptRaster mencetak bitmap monochrome ke builder dengan GS L + GS v 0.
func appendReceiptRaster(b *escpos.Builder, raster *RasterImage) error {
	b.SetLeftMargin(raster.OffsetDots)
	if err := b.PrintRasterBitmap(raster.Data, raster.WidthBytes, raster.HeightPx); err != nil {
		return err
	}
	b.Init()
	b.LineFeed()
	return nil
}

// BuildReceiptImageRaster mendecode base64 gambar dan menyiapkan raster untuk item receipt.
func BuildReceiptImageRaster(b64 string, paperWidthMM int, align string, dither string) (*RasterImage, error) {
	img, _, err := imageproc.DecodeBase64Image(b64)
	if err != nil {
		return nil, err
	}
	return buildReceiptRasterFromImage(img, paperWidthMM, align, dither)
}

// BuildReceiptQRRaster menghasilkan QR code lalu menyiapkan raster untuk item receipt.
func BuildReceiptQRRaster(data string, qrSizePx, paperWidthMM int, align string) (*RasterImage, error) {
	if strings.TrimSpace(data) == "" {
		return nil, nil
	}
	img, err := imageproc.GenerateQRCode(data, qrSizePx)
	if err != nil {
		return nil, err
	}
	return buildReceiptRasterFromImage(img, paperWidthMM, align, imageproc.DitherNone)
}

// buildReceiptRasterFromImage menskalakan gambar ke lebar kertas lalu hitung offset GS L.
func buildReceiptRasterFromImage(img image.Image, paperWidthMM int, align string, dither string) (*RasterImage, error) {
	widthDots := imageproc.MMToDots(paperWidthMM)
	correction := 17
	if widthDots > 400 {
		correction = 25
	}
	safeWidth := widthDots - correction
	if safeWidth < 1 {
		safeWidth = widthDots
	}

	resized := imageproc.ResizeToWidthForce(img, safeWidth)
	mono := imageproc.PrepareMonochromeForce(resized, safeWidth, dither)
	data, wb, h := imageproc.ToRasterBitmap(mono)
	imgDots := wb * 8
	offsetDots := computeRasterOffset(widthDots, imgDots, align, correction)

	return &RasterImage{
		Data:       data,
		WidthBytes: wb,
		HeightPx:   h,
		OffsetDots: offsetDots,
	}, nil
}

// computeRasterOffset menghitung offset GS L untuk posisi raster left/center/right.
func computeRasterOffset(paperDots, imgDots int, align string, correction int) int {
	var offset int
	switch strings.ToLower(strings.TrimSpace(align)) {
	case "right":
		offset = paperDots - imgDots
	case "center":
		offset = (paperDots-imgDots)/2 + correction/2
	default:
		offset = correction
	}
	if offset+imgDots > paperDots {
		offset = paperDots - imgDots
	}
	if offset < 0 {
		offset = 0
	}
	return offset
}

// alignByteWithDefault memetakan string align ke byte ESC a dengan default tertentu.
func alignByteWithDefault(align string, defaultAlign byte) byte {
	switch strings.ToLower(strings.TrimSpace(align)) {
	case "left":
		return escpos.AlignLeft
	case "right":
		return escpos.AlignRight
	case "center":
		return escpos.AlignCenter
	default:
		return defaultAlign
	}
}

// charsPerLineForPaper mengembalikan jumlah karakter per baris untuk garis pemisah.
func charsPerLineForPaper(widthMM int) int {
	switch widthMM {
	case 80:
		return 48
	case 58:
		return 32
	case 76:
		return 40
	default:
		// Jika ukuran tidak standar, kita asumsikan 1 karakter memakan ruang sekitar 1.66mm (seperti 80mm / 48 chars)
		// Namun jika lebarnya >= 210mm (ukuran A4/Continuous form 9.5 inch = 241mm), biasanya itu 80 kolom.
		if widthMM >= 210 {
			return 80
		}
		chars := int(float64(widthMM) / 1.66)
		if chars < 32 {
			return 32
		}
		return chars
	}
}

// lineChar mengembalikan karakter garis pemisah sesuai style.
func lineChar(style string) rune {
	switch strings.ToLower(strings.TrimSpace(style)) {
	case "double":
		return '='
	case "dotted":
		return '.'
	default:
		return '-'
	}
}

// parseFeedLines mengurai jumlah baris feed dari string data (default 1, max 10).
func parseFeedLines(data string) int {
	n := 1
	if strings.TrimSpace(data) != "" {
		if v, err := fmt.Sscanf(strings.TrimSpace(data), "%d", &n); err != nil || v != 1 {
			n = 1
		}
	}
	if n < 1 {
		n = 1
	}
	if n > 10 {
		n = 10
	}
	return n
}

// alignmentByteFromString memetakan string "left|center|right" ke byte
// argumen perintah ESC a. Default = center bila tidak dikenali.
func alignmentByteFromString(pos string) byte {
	switch strings.ToLower(strings.TrimSpace(pos)) {
	case "left":
		return escpos.AlignLeft
	case "right":
		return escpos.AlignRight
	default:
		return escpos.AlignCenter
	}
}

// SendRawReset mengirim payload mentah (biasanya ESC @ = 0x1B 0x40)
// langsung ke printer tanpa membangun job ESC/POS apapun.
// Digunakan oleh endpoint /api/printer/recover untuk mereset state
// parser printer yang stuck tanpa harus mematikan printer fisik.
func (m *Manager) SendRawReset(ctx context.Context, info Info, payload []byte) error {
	return sendToPrinter(ctx, info, payload)
}

// BuildLogoRaster melakukan decode base64, resize logo ke maksimum 70%
// lebar kertas, lalu menghitung OffsetDots (GS L) agar logo tampil
// di posisi left/center/right yang benar.
//
// Mengapa 70% dan GS L (bukan ESC a dan full-width canvas):
//   - ESC a (Align) TIDAK memengaruhi GS v 0 pada Epson TM series.
//   - Full-width canvas (384 dots untuk 58mm) menghasilkan data ~4-5KB
//     per job; beberapa job beruntun dapat overflow receive buffer printer
//     (4-8KB) dan menyebabkan printer berhenti tanpa error.
//   - GS L (Set Left Margin) adalah perintah yang BENAR untuk menggeser
//     posisi horizontal raster pada printer Epson TM.
//   - Logo di-cap 70% lebar kertas → data per job ~30% lebih kecil.
//     OffsetDots dihitung agar logo tetap terlihat di posisi yang tepat.
func BuildLogoRaster(b64 string, paperWidthMM int, position string) (*RasterImage, error) {
	if strings.TrimSpace(b64) == "" {
		return nil, nil
	}
	img, _, err := imageproc.DecodeBase64Image(b64)
	if err != nil {
		return nil, err
	}

	paperDots := imageproc.MMToDots(paperWidthMM)
	// Cap logo ke 70% paper width untuk menjaga data per job dalam batas aman.
	maxLogoDots := paperDots * 7 / 10
	mono := imageproc.PrepareMonochrome(img, maxLogoDots, imageproc.DitherFloydSteinberg)
	data, wb, h := imageproc.ToRasterBitmap(mono)
	logoDots := wb * 8

	// Hitung OffsetDots (GS L) sesuai posisi yang diminta.
	var offsetDots int
	switch strings.ToLower(strings.TrimSpace(position)) {
	case "right":
		offsetDots = paperDots - logoDots
		if offsetDots < 0 {
			offsetDots = 0
		}
	case "left":
		offsetDots = 0
	default: // "center"
		// Tambahkan sedikit offset koreksi (+17 dot untuk 58mm, +25 dot untuk 80mm)
		// karena secara fisik print head thermal memiliki margin kiri bawaan pabrik.
		// Ini menyelaraskan center logo dengan center area teks bawaan printer.
		correction := 17
		if paperDots > 400 {
			correction = 25
		}
		offsetDots = (paperDots - logoDots) / 2 + correction
	}

	// Batasan Keamanan Mutlak: Total offset + lebar gambar tidak boleh melebihi kapasitas kertas (paperDots).
	// Melebihi batas ini dapat menyebabkan buffer overflow pada firmware printer dan membuatnya macet/freeze!
	if offsetDots + logoDots > paperDots {
		offsetDots = paperDots - logoDots
	}
	if offsetDots < 0 {
		offsetDots = 0
	}

	return &RasterImage{Data: data, WidthBytes: wb, HeightPx: h, OffsetDots: offsetDots}, nil
}

// BuildImageRaster sama dengan BuildLogoRaster namun memungkinkan
// pemilihan algoritma dithering. Dipakai oleh handler /api/print/image.
func BuildImageRaster(b64 string, paperWidthMM int, dither string) (*RasterImage, error) {
	if strings.TrimSpace(b64) == "" {
		return nil, errors.New("image_base64 wajib diisi")
	}
	img, _, err := imageproc.DecodeBase64Image(b64)
	if err != nil {
		return nil, err
	}
	widthDots := imageproc.MMToDots(paperWidthMM)

	// Tentukan koreksi margin fisik bawaan printer (+17 dot untuk 58mm, +25 dot untuk 80mm)
	correction := 17
	if widthDots > 400 {
		correction = 25
	}

	// Agar gambar tidak terpotong di tepi kiri (karena margin fisik bawaan printer)
	// dan tidak melebihi area cetak fisik di tepi kanan (yang menyebabkan crash),
	// kita scale gambar agar lebarnya pas mengisi area cetak aman: (widthDots - correction).
	safeWidthDots := widthDots - correction
	if safeWidthDots < 1 {
		safeWidthDots = widthDots
	}

	mono := imageproc.PrepareMonochromeForce(img, safeWidthDots, dither)
	data, wb, h := imageproc.ToRasterBitmap(mono)
	logoDots := wb * 8

	// Posisikan margin kiri gambar tepat di batas margin fisik aman agar tidak terpotong
	offsetDots := correction

	// Batasan Keamanan Mutlak: Total offset + lebar gambar tidak boleh melebihi kapasitas kertas (widthDots).
	if offsetDots + logoDots > widthDots {
		offsetDots = widthDots - logoDots
	}
	if offsetDots < 0 {
		offsetDots = 0
	}

	return &RasterImage{Data: data, WidthBytes: wb, HeightPx: h, OffsetDots: offsetDots}, nil
}

// ============================================================
//   DETEKSI PRINTER (SPOOLER)
// ============================================================

// detectSpoolerPrinters mengembalikan daftar printer pada OS spooler.
// Implementasi dispatch berdasarkan runtime.GOOS.
func detectSpoolerPrinters(ctx context.Context) ([]Info, error) {
	switch runtime.GOOS {
	case "windows":
		return detectSpoolerWindows(ctx)
	case "darwin", "linux":
		return detectSpoolerUnix(ctx)
	default:
		return nil, fmt.Errorf("OS %s belum didukung", runtime.GOOS)
	}
}

// detectSpoolerWindows menjalankan PowerShell Get-Printer untuk mendapatkan
// daftar printer Windows beserta status-nya. Jatuh balik ke wmic bila
// PowerShell tidak tersedia atau melebihi timeout.
func detectSpoolerWindows(ctx context.Context) ([]Info, error) {
	// Gunakan PowerShell karena lebih robust dan terstruktur.
	// Timeout 5s sudah cukup; PowerShell biasanya selesai dalam 1-3s.
	psScript := `Get-Printer | Select-Object Name,PrinterStatus | ForEach-Object { "$($_.Name)|$($_.PrinterStatus)" }`
	out, err := runCommand(ctx, 5*time.Second, "powershell", "-NoProfile", "-NonInteractive", "-WindowStyle", "Hidden", "-Command", psScript)
	if err == nil {
		return parseWindowsPrinterList(out), nil
	}

	// Fallback: wmic printer (lebih ringan, tanpa PowerShell runtime).
	out2, err2 := runCommand(ctx, 5*time.Second, "wmic", "printer", "get", "name,printerstatus", "/format:csv")
	if err2 != nil {
		return nil, fmt.Errorf("powershell gagal (%v) dan wmic gagal (%v)", err, err2)
	}
	return parseWmicPrinterList(out2), nil
}

// parseWindowsPrinterList mem-parse output PowerShell "Name|PrinterStatus".
func parseWindowsPrinterList(s string) []Info {
	var out []Info
	for _, line := range strings.Split(s, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "|", 2)
		if len(parts) < 1 || strings.TrimSpace(parts[0]) == "" {
			continue
		}
		status := StatusReady
		if len(parts) == 2 {
			s := strings.ToLower(strings.TrimSpace(parts[1]))
			if s != "" && s != "normal" && s != "0" && s != "idle" {
				// Status non-normal: tetap "ready" kecuali jelas-jelas offline.
				if strings.Contains(s, "offline") || strings.Contains(s, "error") {
					status = StatusOffline
				}
			}
		}
		out = append(out, Info{
			Name:   strings.TrimSpace(parts[0]),
			Type:   TypeSpooler,
			Status: status,
		})
	}
	return out
}

// parseWmicPrinterList mem-parse output CSV dari wmic.
// Format: Node,Name,PrinterStatus
func parseWmicPrinterList(s string) []Info {
	var out []Info
	for _, line := range strings.Split(s, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(strings.ToLower(line), "node,") {
			continue
		}
		fields := strings.Split(line, ",")
		if len(fields) < 2 {
			continue
		}
		name := strings.TrimSpace(fields[1])
		if name == "" {
			continue
		}
		status := StatusReady
		if len(fields) >= 3 {
			s := strings.ToLower(strings.TrimSpace(fields[2]))
			if strings.Contains(s, "offline") || strings.Contains(s, "error") {
				status = StatusOffline
			}
		}
		out = append(out, Info{Name: name, Type: TypeSpooler, Status: status})
	}
	return out
}

// detectSpoolerUnix menggunakan lpstat -p untuk mendapatkan daftar printer
// di Linux/macOS. Format umum: "printer NAMA is idle. enabled since ...".
func detectSpoolerUnix(ctx context.Context) ([]Info, error) {
	out, err := runCommand(ctx, 5*time.Second, "lpstat", "-p")
	if err != nil {
		// lpstat mungkin tidak terinstall; bukan error fatal, kembalikan list kosong.
		return []Info{}, nil
	}
	var printers []Info
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "printer ") {
			continue
		}
		// "printer NAMA is ..."
		rest := strings.TrimPrefix(line, "printer ")
		parts := strings.SplitN(rest, " ", 2)
		if len(parts) < 1 {
			continue
		}
		name := parts[0]
		status := StatusReady
		if strings.Contains(strings.ToLower(line), "disabled") || strings.Contains(strings.ToLower(line), "offline") {
			status = StatusOffline
		}
		printers = append(printers, Info{Name: name, Type: TypeSpooler, Status: status})
	}
	return printers, nil
}

// ============================================================
//   DETEKSI PRINTER (BLUETOOTH)
// ============================================================

// detectBluetoothPrinters dispatch berdasarkan OS. Kegagalan deteksi
// dianggap non-fatal dan menghasilkan list kosong.
func detectBluetoothPrinters(ctx context.Context) ([]Info, error) {
	switch runtime.GOOS {
	case "windows":
		return detectBluetoothWindows(ctx)
	case "linux":
		return detectBluetoothLinux(ctx)
	case "darwin":
		return detectBluetoothMac(ctx)
	default:
		return []Info{}, nil
	}
}

// detectBluetoothWindows menjalankan PowerShell Get-PnpDevice untuk
// menemukan device Bluetooth yang berasosiasi sebagai printer atau
// memiliki COM port virtual. Hasilnya difilter berdasarkan keyword.
func detectBluetoothWindows(ctx context.Context) ([]Info, error) {
	psScript := `Get-PnpDevice -Class Bluetooth -ErrorAction SilentlyContinue | Where-Object { $_.Status -eq 'OK' } | ForEach-Object { $_.FriendlyName }`
	out, err := runCommand(ctx, 5*time.Second, "powershell", "-NoProfile", "-NonInteractive", "-WindowStyle", "Hidden", "-Command", psScript)
	if err != nil {
		return []Info{}, nil
	}
	var infos []Info
	for _, line := range strings.Split(out, "\n") {
		name := strings.TrimSpace(line)
		if name == "" {
			continue
		}
		// Heuristik sederhana: hanya tampilkan device yang kemungkinan printer.
		l := strings.ToLower(name)
		if strings.Contains(l, "printer") || strings.Contains(l, "thermal") ||
			strings.Contains(l, "pos") || strings.Contains(l, "bt-") || strings.Contains(l, "rpp") {
			infos = append(infos, Info{Name: name, Type: TypeBluetooth, Status: StatusReady})
		}
	}
	return infos, nil
}

// detectBluetoothLinux menggunakan bluetoothctl devices untuk mendapatkan
// daftar device Bluetooth yang sudah dipasangkan.
func detectBluetoothLinux(ctx context.Context) ([]Info, error) {
	out, err := runCommand(ctx, 5*time.Second, "bluetoothctl", "devices")
	if err != nil {
		return []Info{}, nil
	}
	var infos []Info
	for _, line := range strings.Split(out, "\n") {
		// Format: "Device XX:XX:XX:XX:XX:XX FriendlyName"
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "Device ") {
			continue
		}
		fields := strings.SplitN(line, " ", 3)
		if len(fields) < 3 {
			continue
		}
		name := fields[2]
		l := strings.ToLower(name)
		if strings.Contains(l, "printer") || strings.Contains(l, "thermal") ||
			strings.Contains(l, "pos") || strings.Contains(l, "bt-") || strings.Contains(l, "rpp") {
			infos = append(infos, Info{Name: name, Type: TypeBluetooth, Status: StatusReady})
		}
	}
	return infos, nil
}

// detectBluetoothMac menggunakan system_profiler SPBluetoothDataType untuk
// menemukan device Bluetooth yang terhubung pada macOS.
func detectBluetoothMac(ctx context.Context) ([]Info, error) {
	out, err := runCommand(ctx, 8*time.Second, "system_profiler", "SPBluetoothDataType")
	if err != nil {
		return []Info{}, nil
	}
	var infos []Info
	// Parser sederhana: cari baris yang berisi nama device & "Connected: Yes"
	// Banyak versi macOS punya format berbeda; kita ambil pendekatan heuristik.
	lines := strings.Split(out, "\n")
	for i, line := range lines {
		t := strings.TrimSpace(line)
		if strings.HasSuffix(t, ":") && !strings.Contains(t, " ") {
			continue
		}
		l := strings.ToLower(t)
		if (strings.Contains(l, "printer") || strings.Contains(l, "thermal") || strings.Contains(l, "pos")) &&
			strings.HasSuffix(t, ":") {
			name := strings.TrimSuffix(t, ":")
			status := StatusReady
			// Cek beberapa baris setelahnya untuk status "Connected".
			for j := i + 1; j < i+8 && j < len(lines); j++ {
				s := strings.ToLower(lines[j])
				if strings.Contains(s, "connected: no") {
					status = StatusOffline
					break
				}
			}
			infos = append(infos, Info{Name: name, Type: TypeBluetooth, Status: status})
		}
	}
	return infos, nil
}

// ============================================================
//   PENGIRIMAN JOB KE PRINTER
// ============================================================

// sendToPrinter mengirim payload byte ke printer tujuan dengan retry
// otomatis (3x, jeda 5 detik) untuk transport bluetooth.
func sendToPrinter(ctx context.Context, info Info, payload []byte) error {
	switch info.Type {
	case TypeBluetooth:
		return sendBluetoothWithRetry(ctx, info, payload)
	case TypeSpooler, "":
		return sendSpooler(ctx, info, payload)
	default:
		return fmt.Errorf("tipe printer tidak dikenal: %s", info.Type)
	}
}

// sendBluetoothWithRetry membungkus sendBluetooth dengan retry 3x
// dan timeout 5 detik per percobaan. Pemilihan port:
//   - Windows: COM port pada info.Port (atau heuristik default COM3..COM9)
//   - Linux:   /dev/rfcommX pada info.Port (default /dev/rfcomm0)
func sendBluetoothWithRetry(ctx context.Context, info Info, payload []byte) error {
	var lastErr error
	for attempt := 1; attempt <= 3; attempt++ {
		attemptCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		err := sendBluetooth(attemptCtx, info, payload)
		cancel()
		if err == nil {
			return nil
		}
		lastErr = err
		// Jangan retry bila context utama sudah cancel.
		if ctx.Err() != nil {
			break
		}
		if attempt < 3 {
			select {
			case <-time.After(1 * time.Second):
			case <-ctx.Done():
				return ctx.Err()
			}
		}
	}
	return fmt.Errorf("bluetooth gagal setelah 3x percobaan: %w", lastErr)
}

// sendBluetooth menulis payload ke device file/COM port yang
// merepresentasikan koneksi RFCOMM ke printer Bluetooth.
func sendBluetooth(ctx context.Context, info Info, payload []byte) error {
	port := info.Port
	if port == "" {
		// Heuristik default per OS bila port tidak diketahui.
		switch runtime.GOOS {
		case "windows":
			port = "COM3"
		case "linux":
			port = "/dev/rfcomm0"
		default:
			return fmt.Errorf("port bluetooth tidak ditentukan untuk OS %s", runtime.GOOS)
		}
	}
	// Pada Windows, COM port juga merupakan file device; bisa dibuka via os.OpenFile.
	f, err := os.OpenFile(port, os.O_WRONLY|os.O_SYNC, 0)
	if err != nil {
		return fmt.Errorf("gagal buka port %s: %w", port, err)
	}
	defer f.Close()

	// Kirim dengan goroutine + context untuk menerapkan timeout.
	// Menggunakan penulisan ter-chunk (512 byte dengan delay 10ms) agar modul
	// Bluetooth printer yang lambat/kecil buffernya tidak mengalami crash.
	doneCh := make(chan error, 1)
	go func() {
		chunkSize := 512
		totalBytes := len(payload)
		written := 0
		for written < totalBytes {
			if ctx.Err() != nil {
				doneCh <- ctx.Err()
				return
			}
			end := written + chunkSize
			if end > totalBytes {
				end = totalBytes
			}
			n, werr := f.Write(payload[written:end])
			if werr != nil {
				doneCh <- werr
				return
			}
			written += n
			if written < totalBytes {
				time.Sleep(10 * time.Millisecond)
			}
		}
		doneCh <- nil
	}()
	select {
	case werr := <-doneCh:
		return werr
	case <-ctx.Done():
		return ctx.Err()
	}
}

// sendSpooler mengirim payload ke spooler OS dalam mode RAW.
//
// Windows: panggil WinSpool API langsung (OpenPrinterW + StartDocPrinter +
// WritePrinter + EndDocPrinter) via package golang.org/x/sys/windows.
// Pendekatan ini menghindari startup PowerShell dan Add-Type (.NET C#)
// yang sebelumnya memakan ratusan ms s/d >1 detik per cetak. Payload
// dikirim langsung dari memory tanpa file sementara.
//
// Jika WinSpool gagal (mis. printer virtual yang menolak datatype RAW,
// permission khusus, dll.) disediakan fallback klasik: tulis payload
// ke file temp lalu jalankan `print /D:` dan terakhir `copy /B` ke
// share UNC printer.
//
// Linux/macOS: tetap memakai `lp -d <printer> -o raw <tempfile>`.
func sendSpooler(ctx context.Context, info Info, payload []byte) error {
	if runtime.GOOS == "windows" {
		// Jalur cepat: WinSpool langsung tanpa file temp. Sukses di sini
		// akan menjadi kasus mayoritas dan menghilangkan overhead proses
		// eksternal sepenuhnya.
		if err := sendWinspoolRaw(ctx, info.Name, payload); err == nil {
			return nil
		} else {
			// Fallback: tulis ke file temp lalu coba `print /D:` dan
			// `copy /B` seperti jalur lama. File temp hanya dibuat
			// di jalur fallback agar jalur cepat tetap bebas I/O disk.
			return spoolWindowsFallback(ctx, info.Name, payload, err)
		}
	}

	// Non-Windows: masih butuh file temp untuk lp/lpr.
	tmp, err := os.CreateTemp("", "printbridge-*.bin")
	if err != nil {
		return fmt.Errorf("gagal membuat file temp: %w", err)
	}
	tmpPath := tmp.Name()
	if _, err := tmp.Write(payload); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpPath)
		return fmt.Errorf("gagal menulis file temp: %w", err)
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("gagal menutup file temp: %w", err)
	}
	defer os.Remove(tmpPath)

	switch runtime.GOOS {
	case "linux", "darwin":
		return spoolUnix(ctx, info.Name, tmpPath)
	default:
		return fmt.Errorf("OS %s belum didukung untuk spooler", runtime.GOOS)
	}
}

// spoolWindowsFallback dipanggil hanya bila jalur cepat WinSpool gagal.
// Fungsi ini menulis payload ke file sementara lalu mencoba dua strategi
// legacy secara berurutan: `print /D:` dan `copy /B` ke share UNC
// printer. Error dari jalur cepat (winspoolErr) disertakan pada pesan
// akhir agar log /api/logs menampilkan konteks lengkap.
func spoolWindowsFallback(ctx context.Context, printerName string, payload []byte, winspoolErr error) error {
	tmp, err := os.CreateTemp("", "printbridge-*.bin")
	if err != nil {
		return fmt.Errorf("winspool: %v ; gagal membuat file temp: %w", winspoolErr, err)
	}
	tmpPath := tmp.Name()
	if _, err := tmp.Write(payload); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpPath)
		return fmt.Errorf("winspool: %v ; gagal menulis file temp: %w", winspoolErr, err)
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("winspool: %v ; gagal menutup file temp: %w", winspoolErr, err)
	}
	defer os.Remove(tmpPath)

	// Strategi 1: utilitas `print /D:` (Windows builtin).
	if _, err2 := runCommand(ctx, 15*time.Second, "cmd", "/C", "print", "/D:"+printerName, tmpPath); err2 == nil {
		return nil
	} else {
		// Strategi 2: `copy /B` ke share lokal printer (\\localhost\<printer>).
		share := `\\localhost\` + printerName
		if _, err3 := runCommand(ctx, 10*time.Second, "cmd", "/C", "copy", "/B", tmpPath, share); err3 == nil {
			return nil
		} else {
			return fmt.Errorf("winspool: %v ; print: %v ; copy: %v", winspoolErr, err2, err3)
		}
	}
}

// spoolUnix mengirim data RAW ke printer pada Linux/macOS.
//
// Strategi prioritas:
//  1. Tulis langsung ke device file USB (/dev/usb/lpN) bila tersedia.
//     Ini adalah cara paling reliable: data masuk langsung ke kernel
//     USB driver, tidak melewati CUPS daemon sama sekali. Keuntungan:
//     - Flow control langsung (write() blok sampai printer siap)
//     - Tidak ada antrian CUPS yang bisa menumpuk
//     - Tidak ada retry otomatis yang membingungkan
//  2. Fallback ke CUPS (lp/lpr) bila device langsung tidak tersedia.
//     Sebelum kirim, resume printer di CUPS (cupsenable) dan hapus
//     job yang stuck agar antrian tidak menumpuk.
func spoolUnix(ctx context.Context, printerName, filePath string) error {
	// Coba temukan device USB langsung untuk printer ini.
	if devPath := findUSBDeviceForPrinter(ctx, printerName); devPath != "" {
		if err := sendDirectUSB(ctx, devPath, filePath); err == nil {
			return nil
		}
		// Jika gagal (misal permission), jatuh ke CUPS.
	}

	// Pastikan printer di CUPS tidak dalam keadaan paused.
	// cupsenable tidak-destruktif: aman dipanggil meskipun sudah enabled.
	_, _ = runCommand(ctx, 3*time.Second, "cupsenable", printerName)

	// Batalkan job CUPS yang stuck (error/held) agar antrian bersih.
	// Ini mencegah tumpukan job dari koneksi printer sebelumnya.
	cancelStuckCUPSJobs(ctx, printerName)

	out, err := runCommand(ctx, 30*time.Second, "lp", "-d", printerName, "-o", "raw",
		"-o", "StopOnError=true",
		filePath)
	if err != nil {
		// Coba lpr sebagai fallback.
		_, err2 := runCommand(ctx, 30*time.Second, "lpr", "-P", printerName, "-l", filePath)
		if err2 != nil {
			return fmt.Errorf("lp: %v ; lpr: %v ; output: %s", err, err2, strings.TrimSpace(out))
		}
	}
	return nil
}

// findUSBDeviceForPrinter mencari device file USB langsung (/dev/usb/lpN
// atau /dev/lpN) yang sesuai dengan nama printer CUPS. Cara kerja:
// lpstat -v mengembalikan URI device, lalu kita cari device file di
// /dev/usb/ yang sesuai.
func findUSBDeviceForPrinter(ctx context.Context, printerName string) string {
	// Kandidat path device USB yang umum di Linux.
	candidates := []string{
		"/dev/usb/lp0", "/dev/usb/lp1", "/dev/usb/lp2",
		"/dev/lp0", "/dev/lp1", "/dev/lp2",
	}
	// Periksa apakah device file ada (os.Stat cukup; open dengan O_WRONLY
	// dilakukan saat pengiriman sebenarnya di sendDirectUSB).
	for _, path := range candidates {
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}
	_ = printerName
	return ""
}

// sendDirectUSB menulis file payload langsung ke device file USB printer.
// Berbeda dengan CUPS, penulisan ini sinkron: write() akan memblok sampai
// printer benar-benar siap menerima data (flow control kernel USB).
// Hasilnya adalah: kita tidak lanjut ke job berikutnya sampai printer
// selesai menerima SEMUA byte — tidak ada antrian tersembunyi.
func sendDirectUSB(ctx context.Context, devPath, filePath string) error {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("gagal baca file payload: %w", err)
	}

	f, err := os.OpenFile(devPath, os.O_WRONLY, 0)
	if err != nil {
		return fmt.Errorf("gagal buka device %s: %w", devPath, err)
	}
	defer f.Close()

	// Tulis dengan goroutine agar bisa dihormati context timeout.
	// Menggunakan penulisan ter-chunk (1024 byte dengan delay 5ms) agar printer
	// thermal USB (terutama printer OEM/murah) tidak mengalami deadlock/buffer overflow
	// saat dikirimi data gambar berukuran besar.
	doneCh := make(chan error, 1)
	go func() {
		chunkSize := 1024
		totalBytes := len(data)
		written := 0
		for written < totalBytes {
			if ctx.Err() != nil {
				doneCh <- ctx.Err()
				return
			}
			end := written + chunkSize
			if end > totalBytes {
				end = totalBytes
			}
			n, werr := f.Write(data[written:end])
			if werr != nil {
				doneCh <- werr
				return
			}
			written += n
			if written < totalBytes {
				time.Sleep(5 * time.Millisecond)
			}
		}
		doneCh <- nil
	}()

	select {
	case err := <-doneCh:
		if err != nil {
			return fmt.Errorf("gagal tulis ke device %s: %w", devPath, err)
		}
		return nil
	case <-ctx.Done():
		return fmt.Errorf("tulis USB timeout/dibatalkan: %w", ctx.Err())
	}
}

// cancelStuckCUPSJobs membatalkan job-job CUPS yang berada dalam status
// error atau held untuk printer tertentu, mencegah antrian menumpuk.
// Dipanggil sebelum mengirim job baru melalui CUPS.
func cancelStuckCUPSJobs(ctx context.Context, printerName string) {
	// Dapatkan daftar job untuk printer ini.
	out, err := runCommand(ctx, 3*time.Second, "lpstat", "-o", printerName)
	if err != nil || strings.TrimSpace(out) == "" {
		return
	}
	// Format: "JobID-N printerName size date"
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		jobID := fields[0]
		// Batalkan job: cancel -a hanya membatalkan job yang bisa dibatalkan.
		_, _ = runCommand(ctx, 3*time.Second, "cancel", jobID)
	}
}

// ============================================================
//   UTIL EKSEKUSI PERINTAH
// ============================================================

// runCommand mengeksekusi sebuah perintah eksternal dengan timeout
// dan mengembalikan stdout-nya sebagai string. Stdin di-set kosong.
func runCommand(parent context.Context, timeout time.Duration, name string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(parent, timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, name, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	cmd.Stdin = strings.NewReader("")
	if err := cmd.Run(); err != nil {
		return stdout.String(), fmt.Errorf("%s gagal: %w (stderr=%s)", filepath.Base(name), err, strings.TrimSpace(stderr.String()))
	}
	return stdout.String(), nil
}

// drainReaderToString sederhana, dipakai bila ingin membaca pipa output
// secara streaming. Disimpan untuk kelengkapan API.
func drainReaderToString(r io.Reader) string {
	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	return buf.String()
}
