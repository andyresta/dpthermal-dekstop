// Package config menangani konfigurasi PrintBridge yang dibaca dan
// disimpan dalam file config.json di direktori binary. Menyediakan
// akses thread-safe ke konfigurasi melalui sync.RWMutex.
package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"printbridge/internal/helper"
)

// Nama file konfigurasi yang persisten di samping binary.
const ConfigFileName = "config.json"

// Tipe printer yang didukung (menentukan transport: antrean OS vs RFCOMM).
const (
	PrinterTypeSpooler   = "spooler"
	PrinterTypeBluetooth = "bluetooth"
)

// Mode cetak printer:
//   - thermal   → gunakan perintah ESC/POS (bitmap, cut, alignment, logo)
//   - dotmatrix → kirim teks plain tanpa ESC/POS; cetak gambar tidak didukung
const (
	PrinterModeThermal   = "thermal"
	PrinterModeDotMatrix = "dotmatrix"
)

// Config merepresentasikan struktur konfigurasi yang disimpan di
// config.json. Field harus selalu di-marshal/unmarshal sebagai JSON.
type Config struct {
	DefaultPrinter      string `json:"default_printer"`
	DefaultPaperWidthMM int    `json:"default_paper_width_mm"`
	PrinterType         string `json:"printer_type"`
	// PrinterMode menentukan cara pembentukan payload cetak:
	// "thermal" (default) = ESC/POS; "dotmatrix" = plain text.
	PrinterMode           string `json:"printer_mode"`
	DotMatrixCharsPerLine int    `json:"dotmatrix_chars_per_line"`
}

// DefaultConfig mengembalikan konfigurasi default yang dipakai bila
// config.json belum ada saat startup.
func DefaultConfig() Config {
	return Config{
		DefaultPrinter:      "",
		DefaultPaperWidthMM: 80,
		PrinterType:           PrinterTypeSpooler,
		PrinterMode:           PrinterModeThermal,
		DotMatrixCharsPerLine: 33,
	}
}

// Manager mengelola state konfigurasi in-memory dan persistensi-nya
// ke disk. Aman digunakan dari banyak goroutine sekaligus.
type Manager struct {
	mu   sync.RWMutex
	cfg  Config
	path string
}

// NewManager membuat Manager baru dan langsung memuat config dari disk.
// Bila file belum ada, file akan dibuat dengan nilai default.
//
// Pencarian config.json dilakukan secara berurutan:
//  1. Working directory (os.Getwd) — berguna saat go run atau service
//     dijalankan dari direktori yang sama dengan config.
//  2. dir (biasanya direktori binary) — berguna saat binary dijalankan
//     dari direktori lain.
//
// Jika tidak ditemukan di keduanya, file baru dibuat di working directory
// (atau dir bila Getwd gagal).
func NewManager(dir string) (*Manager, error) {
	path := resolveConfigPath(dir)
	m := &Manager{path: path}
	if err := m.Load(); err != nil {
		return nil, err
	}
	return m, nil
}

// resolveConfigPath menentukan path config.json yang akan dipakai.
// Prioritas: working directory → binDir. Jika tidak ada di keduanya,
// kembalikan path di working directory (akan dibuat saat Load).
func resolveConfigPath(binDir string) string {
	wd, errWd := os.Getwd()
	candidates := []string{}
	if errWd == nil && wd != "" {
		candidates = append(candidates, filepath.Join(wd, ConfigFileName))
	}
	if binDir != "" && (errWd != nil || binDir != wd) {
		candidates = append(candidates, filepath.Join(binDir, ConfigFileName))
	}
	// Kembalikan path pertama yang filenya sudah ada.
	for _, p := range candidates {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	// Tidak ada yang ada; gunakan working directory untuk membuat baru.
	if errWd == nil && wd != "" {
		return filepath.Join(wd, ConfigFileName)
	}
	return filepath.Join(binDir, ConfigFileName)
}

// Path mengembalikan path absolut file config.json yang dipakai.
func (m *Manager) Path() string {
	return m.path
}

// Load membaca config.json ke dalam memory. Jika file tidak ada,
// konfigurasi default akan di-set dan langsung disimpan ke disk.
func (m *Manager) Load() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	data, err := os.ReadFile(m.path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			// Buat file baru dengan default agar pengguna dapat melihat
			// struktur konfigurasi yang valid.
			m.cfg = DefaultConfig()
			return m.saveLocked()
		}
		return fmt.Errorf("gagal membaca %s: %w", m.path, err)
	}

	var loaded Config
	if err := json.Unmarshal(data, &loaded); err != nil {
		return fmt.Errorf("config.json rusak: %w", err)
	}
	// Sanitasi nilai default jika field kosong setelah unmarshal.
	needsSave := false
	if loaded.DefaultPaperWidthMM == 0 {
		loaded.DefaultPaperWidthMM = 80
		needsSave = true
	}
	if loaded.PrinterType == "" {
		loaded.PrinterType = PrinterTypeSpooler
		needsSave = true
	}
	if loaded.PrinterMode == "" {
		// Field baru; tulis kembali ke disk agar config.json selalu up-to-date.
		loaded.PrinterMode = PrinterModeThermal
		needsSave = true
	}
	if loaded.DotMatrixCharsPerLine <= 0 {
		loaded.DotMatrixCharsPerLine = 33
		needsSave = true
	}
	m.cfg = loaded
	if needsSave {
		// Simpan atomik agar field yang baru ditambahkan ikut tertulis ke file.
		return m.saveLocked()
	}
	return nil
}

// Get mengembalikan salinan konfigurasi saat ini agar caller tidak
// bisa memodifikasi state internal tanpa lock.
func (m *Manager) Get() Config {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.cfg
}

// Update mengganti konfigurasi in-memory lalu menulisnya ke disk
// secara atomik. Validasi dilakukan via Validate() sebelum disimpan.
func (m *Manager) Update(newCfg Config) error {
	if err := Validate(newCfg); err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.cfg = newCfg
	return m.saveLocked()
}

// saveLocked menyimpan konfigurasi ke disk; pemanggil harus sudah
// memegang m.mu (Lock, bukan RLock).
func (m *Manager) saveLocked() error {
	data, err := json.MarshalIndent(m.cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("gagal marshal config: %w", err)
	}
	return helper.AtomicWriteFile(m.path, data, 0o644)
}

// Validate memeriksa nilai konfigurasi sebelum disimpan.
// Mengembalikan error deskriptif bila ada nilai yang tidak valid.
func Validate(c Config) error {
	if c.DefaultPaperWidthMM != 58 && c.DefaultPaperWidthMM != 80 {
		return fmt.Errorf("default_paper_width_mm harus 58 atau 80, dapat %d", c.DefaultPaperWidthMM)
	}
	if c.PrinterType != PrinterTypeSpooler && c.PrinterType != PrinterTypeBluetooth {
		return fmt.Errorf("printer_type harus 'spooler' atau 'bluetooth', dapat %q", c.PrinterType)
	}
	if c.PrinterMode != PrinterModeThermal && c.PrinterMode != PrinterModeDotMatrix {
		return fmt.Errorf("printer_mode harus 'thermal' atau 'dotmatrix', dapat %q", c.PrinterMode)
	}
	if c.DotMatrixCharsPerLine <= 0 {
		return fmt.Errorf("dotmatrix_chars_per_line harus lebih besar dari 0, dapat %d", c.DotMatrixCharsPerLine)
	}
	// default_printer boleh kosong (artinya belum dipilih).
	return nil
}
