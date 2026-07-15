// Package server menyediakan HTTP handler dan router untuk PrintBridge.
// Semua endpoint API memakai net/http standard library, mengembalikan
// JSON, dan menyertakan header CORS sehingga aplikasi web di origin
// lain (port lain) dapat mengonsumsi service ini.
package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os/exec"
	"strings"
	"time"

	"printbridge/internal/config"
	"printbridge/internal/helper"
	"printbridge/internal/logger"
	"printbridge/internal/printer"
	"printbridge/internal/ui"
)

// Server membungkus dependency yang dibutuhkan oleh seluruh handler.
type Server struct {
	Cfg     *config.Manager
	Logs    *logger.Buffer
	Printer *printer.Manager
}

// New membuat Server baru dengan dependency yang sudah diinisialisasi.
func New(cfg *config.Manager, logs *logger.Buffer, p *printer.Manager) *Server {
	return &Server{Cfg: cfg, Logs: logs, Printer: p}
}

// Routes membangun http.Handler dengan seluruh route dan middleware
// (CORS + logging ringan) sudah ter-attach.
//
// Catatan: untuk mengakomodasi konvensi proyek "semua call API
// menggunakan POST" sambil tetap memenuhi spec REST (GET/PUT), endpoint
// read-only menerima GET maupun POST, dan endpoint /api/config menerima
// GET, POST (read), serta PUT (update).
func (s *Server) Routes() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("/api/print/text", s.handleAllowedMethods([]string{http.MethodPost}, s.handlePrintText))
	mux.HandleFunc("/api/print/image", s.handleAllowedMethods([]string{http.MethodPost}, s.handlePrintImage))
	mux.HandleFunc("/api/print/receipt", s.handleAllowedMethods([]string{http.MethodPost}, s.handlePrintReceipt))
	// Alias kompatibilitas MaxThermal Android (POST /print/receipt).
	mux.HandleFunc("/print/receipt", s.handleAllowedMethods([]string{http.MethodPost}, s.handlePrintReceipt))
	mux.HandleFunc("/api/printers", s.handleAllowedMethods([]string{http.MethodGet, http.MethodPost}, s.handlePrinters))
	mux.HandleFunc("/api/logs", s.handleAllowedMethods([]string{http.MethodGet, http.MethodPost}, s.handleLogs))
	mux.HandleFunc("/api/config", s.handleConfig)
	mux.HandleFunc("/api/printers/refresh", s.handleAllowedMethods([]string{http.MethodPost}, s.handlePrintersRefresh))
	// /api/printer/recover: bersihkan antrian CUPS + kirim ESC @ reset.
	// Dipanggil saat printer macet tanpa harus restart printer fisik.
	mux.HandleFunc("/api/printer/recover", s.handleAllowedMethods([]string{http.MethodPost}, s.handlePrinterRecover))
	mux.HandleFunc("/check-bridge-print", s.handleCheckBridgePrint)

	mux.HandleFunc("/", s.handleIndex)

	return s.withCORS(mux)
}

// withCORS adalah middleware yang menambahkan header CORS pada semua
// response dan menangani preflight OPTIONS.
func (s *Server) withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		w.Header().Set("Access-Control-Max-Age", "86400")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// handleAllowedMethods membatasi handler hanya menerima method dari
// daftar allowed; jika tidak cocok, kembalikan 405.
func (s *Server) handleAllowedMethods(allowed []string, h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ok := false
		for _, m := range allowed {
			if r.Method == m {
				ok = true
				break
			}
		}
		if !ok {
			writeJSON(w, http.StatusMethodNotAllowed, map[string]any{
				"success": false,
				"message": fmt.Sprintf("metode %s tidak diizinkan; gunakan %v", r.Method, allowed),
			})
			return
		}
		h(w, r)
	}
}

// handleCheckBridgePrint mengembalikan status kesiapan layanan (ready check).
func (s *Server) handleCheckBridgePrint(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"code": 0,
		"msg":  "ready",
	})
}

// handleIndex menyajikan halaman UI utama; route lain di luar /api
// dialihkan ke 404 sederhana.
func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" && r.URL.Path != "/index.html" {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	_, _ = w.Write([]byte(ui.IndexHTML))
}

// ============================================================
//   HANDLER: /api/printers
// ============================================================

// handlePrinters mengembalikan daftar printer yang sudah dideteksi.
func (s *Server) handlePrinters(w http.ResponseWriter, r *http.Request) {
	list := s.Printer.List()
	writeJSON(w, http.StatusOK, map[string]any{"printers": list})
}

// handlePrintersRefresh memicu deteksi ulang printer secara sinkron.
func (s *Server) handlePrintersRefresh(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()
	if err := s.Printer.Refresh(ctx); err != nil {
		writeJSON(w, http.StatusOK, map[string]any{
			"printers": s.Printer.List(),
			"warning":  err.Error(),
		})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"printers": s.Printer.List()})
}

// ============================================================
//   HANDLER: /api/logs
// ============================================================

// handleLogs mengembalikan seluruh log dari circular buffer (newest first).
func (s *Server) handleLogs(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"logs": s.Logs.GetAll()})
}

// ============================================================
//   HANDLER: /api/printer/recover
// ============================================================

// handlePrinterRecover melakukan pemulihan printer yang macet tanpa
// harus mematikan/menghidupkan printer fisik. Langkah-langkah:
//  1. cupsenable  — memastikan printer tidak dalam kondisi paused di CUPS.
//  2. cancel jobs — membatalkan semua job CUPS yang stuck/error/held.
//  3. ESC @       — mengirim perintah reset ESC/POS ke printer untuk
//                   membersihkan buffer internal dan mereset state parser.
func (s *Server) handlePrinterRecover(w http.ResponseWriter, r *http.Request) {
	cfg := s.Cfg.Get()
	if strings.TrimSpace(cfg.DefaultPrinter) == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{
			"success": false,
			"message": "Belum ada default printer yang dikonfigurasi",
		})
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
	defer cancel()

	printerName := cfg.DefaultPrinter
	steps := []string{}

	// Langkah 1: cupsenable (hanya efektif di Linux/macOS).
	if _, err := runOSCommand(ctx, 3*time.Second, "cupsenable", printerName); err == nil {
		steps = append(steps, "cupsenable: OK")
	} else {
		steps = append(steps, "cupsenable: "+err.Error())
	}

	// Langkah 2: Batalkan semua job yang ada di antrian untuk printer ini.
	cancelledCount := 0
	if out, err := runOSCommand(ctx, 3*time.Second, "lpstat", "-o", printerName); err == nil {
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
			if _, err := runOSCommand(ctx, 3*time.Second, "cancel", jobID); err == nil {
				cancelledCount++
			}
		}
	}
	steps = append(steps, fmt.Sprintf("cancel jobs: %d job dibatalkan", cancelledCount))

	// Langkah 3: Kirim ESC @ (reset) ke printer untuk bersihkan state parser.
	resetPayload := []byte{0x1B, '@'} // ESC @
	info, ok := s.Printer.Find(printerName)
	if !ok {
		info = printer.Info{Name: printerName, Type: cfg.PrinterType, Status: printer.StatusReady}
	}
	if err := s.Printer.SendRawReset(ctx, info, resetPayload); err != nil {
		steps = append(steps, "ESC @ reset: "+err.Error())
	} else {
		steps = append(steps, "ESC @ reset: OK")
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"success": true,
		"message": "Printer recovery selesai",
		"steps":   steps,
	})
}

// runOSCommand adalah alias tipis untuk menjalankan perintah OS dengan timeout.
// Dipisah dari runCommand internal printer package agar server dapat
// memanggil perintah CUPS (cupsenable, cancel, lpstat) tanpa circular import.
func runOSCommand(ctx context.Context, timeout time.Duration, name string, args ...string) (string, error) {
	tctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	cmd := exec.CommandContext(tctx, name, args...)
	var out strings.Builder
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		return out.String(), err
	}
	return out.String(), nil
}

// ============================================================
//   HANDLER: /api/config
// ============================================================

// handleConfig dispatch berdasarkan method:
//   - GET             → kembalikan config saat ini
//   - PUT             → update + tulis ke config.json
//   - POST            → update bila body berisi JSON, atau read bila body kosong
//
// Dukungan POST agar konsisten dengan konvensi proyek "semua call API
// menggunakan POST" sambil tetap memenuhi spec REST.
func (s *Server) handleConfig(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, http.StatusOK, s.Cfg.Get())
		return
	case http.MethodPut, http.MethodPost:
		// Baca body terlebih dahulu agar bisa membedakan POST kosong (read)
		// dengan POST berisi (update).
		var newCfg config.Config
		err := decodeJSON(r, &newCfg)
		if err != nil {
			// POST tanpa body / body kosong → perlakukan sebagai read.
			if r.Method == http.MethodPost && (errors.Is(err, io.EOF) || strings.Contains(err.Error(), "EOF")) {
				writeJSON(w, http.StatusOK, s.Cfg.Get())
				return
			}
			writeJSON(w, http.StatusBadRequest, map[string]any{
				"success": false,
				"message": "JSON tidak valid: " + err.Error(),
			})
			return
		}
		// Samakan printer_type dengan hasil deteksi bila printer dikenali,
		// agar tidak ada mismatch (mis. API mengirim spooler untuk device bluetooth).
		s.syncPrinterTypeFromDetected(&newCfg)
		if err := s.Cfg.Update(newCfg); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]any{
				"success": false,
				"message": err.Error(),
			})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"success": true,
			"message": "Config tersimpan ke config.json",
			"config":  s.Cfg.Get(),
		})
		return
	default:
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{
			"success": false,
			"message": "metode tidak diizinkan; gunakan GET, POST, atau PUT",
		})
	}
}

// ============================================================
//   HANDLER: /api/print/text
// ============================================================

// printTextRequest merepresentasikan body JSON untuk POST /api/print/text.
type printTextRequest struct {
	Text         string `json:"text"`
	LogoBase64   string `json:"logo_base64"`
	LogoPosition string `json:"logo_position"`
	Copies       int    `json:"copies"`
	CutPaper     *bool  `json:"cut_paper"`
	Encoding     string `json:"encoding"`
	// Mode: "thermal" (default) atau "dotmatrix".
	// Bila kosong, diambil dari default_printer_mode di config.
	Mode    string `json:"mode"`
	WidthMM int    `json:"width_mm"`
}

// handlePrintText memproses permintaan cetak teks.
func (s *Server) handlePrintText(w http.ResponseWriter, r *http.Request) {
	var req printTextRequest
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{
			"success": false,
			"message": "JSON tidak valid: " + err.Error(),
		})
		return
	}
	if strings.TrimSpace(req.Text) == "" && strings.TrimSpace(req.LogoBase64) == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{
			"success": false,
			"message": "field 'text' atau 'logo_base64' minimal salah satu wajib diisi",
		})
		return
	}

	cfg := s.Cfg.Get()
	if strings.TrimSpace(cfg.DefaultPrinter) == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{
			"success": false,
			"message": "No default printer configured. Please set one in the UI settings.",
		})
		return
	}

	jobID := helper.NewUUIDv4()
	start := time.Now()

	// Untuk mode dotmatrix, logo tidak diperlukan dan proses decode
	// raster bisa dilewati agar tidak membuang waktu.
	mode := req.Mode
	if mode == "" {
		mode = cfg.PrinterMode
	}

	width := req.WidthMM
	if width != 58 && width != 80 {
		width = cfg.DefaultPaperWidthMM
	}

	var logoRaster *printer.RasterImage
	if mode != "dotmatrix" && strings.TrimSpace(req.LogoBase64) != "" {
		var err error
		logoRaster, err = printer.BuildLogoRaster(req.LogoBase64, width, req.LogoPosition)
		if err != nil {
			s.logFinish(jobID, cfg.DefaultPrinter, logger.JobTypeText, logger.StatusFailed, start, "logo: "+err.Error())
			writeJSON(w, http.StatusBadRequest, map[string]any{
				"success": false,
				"job_id":  jobID,
				"message": "logo_base64 tidak valid: " + err.Error(),
			})
			return
		}
	}

	cutPaper := false
	if req.CutPaper != nil {
		cutPaper = *req.CutPaper
	}

	job := printer.TextJob{
		Text:         req.Text,
		LogoBitmap:   logoRaster,
		LogoPosition: req.LogoPosition,
		Copies:       req.Copies,
		CutPaper:     cutPaper,
		Encoding:     req.Encoding,
		Mode:         mode,
	}

	s.Logs.Add(logger.LogEntry{
		JobID:     jobID,
		Timestamp: start,
		Printer:   cfg.DefaultPrinter,
		Type:      logger.JobTypeText,
		Status:    logger.StatusPending,
		Message:   "memulai cetak teks [" + mode + "]",
	})

	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	if err := s.Printer.PrintText(ctx, cfg, job); err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, context.DeadlineExceeded) {
			status = http.StatusServiceUnavailable
		}
		s.logFinish(jobID, cfg.DefaultPrinter, logger.JobTypeText, logger.StatusFailed, start, err.Error())
		writeJSON(w, status, map[string]any{
			"success": false,
			"job_id":  jobID,
			"message": err.Error(),
		})
		return
	}

	s.logFinish(jobID, cfg.DefaultPrinter, logger.JobTypeText, logger.StatusSuccess, start, "ok")
	writeJSON(w, http.StatusOK, map[string]any{
		"success": true,
		"job_id":  jobID,
		"message": "Job berhasil dikirim ke printer",
	})
}

// ============================================================
//   HANDLER: /api/print/image
// ============================================================

// printImageRequest merepresentasikan body JSON untuk POST /api/print/image.
type printImageRequest struct {
	ImageBase64 string `json:"image_base64"`
	WidthMM     int    `json:"width_mm"`
	Dithering   string `json:"dithering"`
	Copies      int    `json:"copies"`
	CutPaper    *bool  `json:"cut_paper"`
	// Mode: "thermal" didukung; "dotmatrix" akan menghasilkan error 400.
	Mode string `json:"mode"`
}

// handlePrintImage memproses permintaan cetak gambar.
func (s *Server) handlePrintImage(w http.ResponseWriter, r *http.Request) {
	var req printImageRequest
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{
			"success": false,
			"message": "JSON tidak valid: " + err.Error(),
		})
		return
	}
	if strings.TrimSpace(req.ImageBase64) == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{
			"success": false,
			"message": "field 'image_base64' wajib diisi",
		})
		return
	}

	cfg := s.Cfg.Get()
	if strings.TrimSpace(cfg.DefaultPrinter) == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{
			"success": false,
			"message": "No default printer configured. Please set one in the UI settings.",
		})
		return
	}

	width := req.WidthMM
	if width != 58 && width != 80 {
		width = cfg.DefaultPaperWidthMM
	}

	imgMode := req.Mode
	if imgMode == "" {
		imgMode = cfg.PrinterMode
	}

	// Tolak lebih awal sebelum decode gambar bila mode dotmatrix.
	if imgMode == "dotmatrix" {
		writeJSON(w, http.StatusBadRequest, map[string]any{
			"success": false,
			"message": "Cetak gambar tidak didukung pada mode Dot Matrix / Plain Text. Pilih mode Thermal untuk mencetak gambar.",
		})
		return
	}

	jobID := helper.NewUUIDv4()
	start := time.Now()

	raster, err := printer.BuildImageRaster(req.ImageBase64, width, req.Dithering)
	if err != nil {
		s.logFinish(jobID, cfg.DefaultPrinter, logger.JobTypeImage, logger.StatusFailed, start, err.Error())
		writeJSON(w, http.StatusBadRequest, map[string]any{
			"success": false,
			"job_id":  jobID,
			"message": "image_base64 tidak valid: " + err.Error(),
		})
		return
	}

	cutPaper := false
	if req.CutPaper != nil {
		cutPaper = *req.CutPaper
	}
	job := printer.ImageJob{
		Bitmap:   raster,
		Copies:   req.Copies,
		CutPaper: cutPaper,
		Mode:     imgMode,
	}

	s.Logs.Add(logger.LogEntry{
		JobID:     jobID,
		Timestamp: start,
		Printer:   cfg.DefaultPrinter,
		Type:      logger.JobTypeImage,
		Status:    logger.StatusPending,
		Message:   "memulai cetak gambar [thermal]",
	})

	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	if err := s.Printer.PrintImage(ctx, cfg, job); err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, context.DeadlineExceeded) {
			status = http.StatusServiceUnavailable
		}
		s.logFinish(jobID, cfg.DefaultPrinter, logger.JobTypeImage, logger.StatusFailed, start, err.Error())
		writeJSON(w, status, map[string]any{
			"success": false,
			"job_id":  jobID,
			"message": err.Error(),
		})
		return
	}

	s.logFinish(jobID, cfg.DefaultPrinter, logger.JobTypeImage, logger.StatusSuccess, start, "ok")
	writeJSON(w, http.StatusOK, map[string]any{
		"success": true,
		"job_id":  jobID,
		"message": "Job gambar berhasil dikirim ke printer",
	})
}

// ============================================================
//   HANDLER: /api/print/receipt  (+ alias /print/receipt)
// ============================================================

// receiptItemJSON merepresentasikan satu item dalam payload receipt.
type receiptItemJSON struct {
	Type  string `json:"type"`
	Align string `json:"align"`
	Size  int    `json:"size"`
	Style string `json:"style"`
	Data  string `json:"data"`
}

// printReceiptRequest merepresentasikan body JSON untuk POST /api/print/receipt.
// Format selaras dengan MaxThermal Android: { "items": [ ... ] }.
type printReceiptRequest struct {
	Items     []receiptItemJSON `json:"items"`
	Copies    int               `json:"copies"`
	CutPaper  *bool             `json:"cut_paper"`
	Mode      string            `json:"mode"`
	WidthMM   int               `json:"width_mm"`
	Dithering string            `json:"dithering"`
}

// handlePrintReceipt memproses permintaan cetak nota terstruktur (array items).
func (s *Server) handlePrintReceipt(w http.ResponseWriter, r *http.Request) {
	var req printReceiptRequest
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{
			"success": false,
			"message": "JSON tidak valid: " + err.Error(),
		})
		return
	}
	if len(req.Items) == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]any{
			"success": false,
			"message": "field 'items' tidak ditemukan atau kosong",
		})
		return
	}

	cfg := s.Cfg.Get()
	if strings.TrimSpace(cfg.DefaultPrinter) == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{
			"success": false,
			"message": "No default printer configured. Please set one in the UI settings.",
		})
		return
	}

	jobID := helper.NewUUIDv4()
	start := time.Now()

	mode := req.Mode
	if mode == "" {
		mode = cfg.PrinterMode
	}

	width := req.WidthMM
	if mode != config.PrinterModeDotMatrix {
		if width != 58 && width != 80 {
			width = cfg.DefaultPaperWidthMM
		}
	} else {
		if width < 80 {
			width = 80
		}
	}
	cutPaper := false
	if req.CutPaper != nil {
		cutPaper = *req.CutPaper
	}

	items := make([]printer.ReceiptItem, len(req.Items))
	for i, it := range req.Items {
		items[i] = printer.ReceiptItem{
			Type:  it.Type,
			Align: it.Align,
			Size:  it.Size,
			Style: it.Style,
			Data:  it.Data,
		}
	}

	job := printer.ReceiptJob{
		Items:     items,
		Copies:    req.Copies,
		CutPaper:  cutPaper,
		Mode:      mode,
		WidthMM:   width,
		Dithering: req.Dithering,
	}

	s.Logs.Add(logger.LogEntry{
		JobID:     jobID,
		Timestamp: start,
		Printer:   cfg.DefaultPrinter,
		Type:      logger.JobTypeReceipt,
		Status:    logger.StatusPending,
		Message:   "memulai cetak receipt [" + mode + "]",
	})

	ctx, cancel := context.WithTimeout(r.Context(), 60*time.Second)
	defer cancel()

	if err := s.Printer.PrintReceipt(ctx, cfg, job); err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, context.DeadlineExceeded) {
			status = http.StatusServiceUnavailable
		}
		s.logFinish(jobID, cfg.DefaultPrinter, logger.JobTypeReceipt, logger.StatusFailed, start, err.Error())
		writeJSON(w, status, map[string]any{
			"success": false,
			"job_id":  jobID,
			"message": err.Error(),
		})
		return
	}

	s.logFinish(jobID, cfg.DefaultPrinter, logger.JobTypeReceipt, logger.StatusSuccess, start, "ok")
	writeJSON(w, http.StatusOK, map[string]any{
		"success": true,
		"job_id":  jobID,
		"message": "Receipt berhasil dikirim ke printer",
	})
}

// ============================================================
//   UTILITAS INTERNAL
// ============================================================

// syncPrinterTypeFromDetected menyetel cfg.PrinterType agar selaras dengan
// entri terdeteksi di Printer Manager (spooler vs bluetooth). Jika nama
// printer tidak ada di cache deteksi, nilai dari klien dipertahankan.
func (s *Server) syncPrinterTypeFromDetected(cfg *config.Config) {
	name := strings.TrimSpace(cfg.DefaultPrinter)
	if name == "" {
		return
	}
	if info, ok := s.Printer.Find(name); ok {
		switch info.Type {
		case printer.TypeBluetooth:
			cfg.PrinterType = config.PrinterTypeBluetooth
		default:
			cfg.PrinterType = config.PrinterTypeSpooler
		}
	}
}

// logFinish menambahkan entry log final dengan durasi terhitung dari start.
func (s *Server) logFinish(jobID, printerName, jobType, status string, start time.Time, msg string) {
	s.Logs.Add(logger.LogEntry{
		JobID:      jobID,
		Timestamp:  time.Now(),
		Printer:    printerName,
		Type:       jobType,
		Status:     status,
		DurationMS: time.Since(start).Milliseconds(),
		Message:    msg,
	})
}

// writeJSON menulis response JSON dengan Content-Type yang benar.
func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	_ = enc.Encode(payload)
}

// decodeJSON membaca body request sebagai JSON ke target.
// Membatasi ukuran body 32 MB untuk mencegah penyalahgunaan.
func decodeJSON(r *http.Request, target any) error {
	r.Body = http.MaxBytesReader(nil, r.Body, 32<<20)
	dec := json.NewDecoder(r.Body)
	if err := dec.Decode(target); err != nil {
		return err
	}
	return nil
}
