// Package logger menyediakan circular buffer in-memory yang thread-safe
// untuk menyimpan riwayat job pencetakan PrintBridge. Tidak ada
// persistensi ke file/database; semua log akan hilang saat restart.
package logger

import (
	"fmt"
	"os"
	"sync"
	"time"
)

// Konstanta status job pencetakan yang valid.
const (
	StatusPending = "pending"
	StatusSuccess = "success"
	StatusFailed  = "failed"
)

// Konstanta tipe job pencetakan.
const (
	JobTypeText    = "text"
	JobTypeImage   = "image"
	JobTypeReceipt = "receipt"
)

// Kapasitas maksimum buffer log in-memory.
const MaxEntries = 200

// LogEntry merepresentasikan satu baris log yang ditampilkan
// di endpoint GET /api/logs maupun di terminal.
type LogEntry struct {
	JobID      string    `json:"job_id"`
	Timestamp  time.Time `json:"timestamp"`
	Printer    string    `json:"printer"`
	Type       string    `json:"type"`
	Status     string    `json:"status"`
	DurationMS int64     `json:"duration_ms"`
	Message    string    `json:"message"`
}

// Buffer adalah circular buffer thread-safe untuk LogEntry.
type Buffer struct {
	mu      sync.Mutex
	entries []LogEntry
	max     int
}

// NewBuffer membuat Buffer baru dengan kapasitas maksimum max entry.
// Bila max <= 0, default MaxEntries akan dipakai.
func NewBuffer(max int) *Buffer {
	if max <= 0 {
		max = MaxEntries
	}
	return &Buffer{
		entries: make([]LogEntry, 0, max),
		max:     max,
	}
}

// Add menambahkan entry baru ke buffer. Bila buffer sudah penuh,
// entry tertua akan dibuang. Juga menulis ringkasan ke stderr (untuk
// status failed) atau stdout (untuk status lain) sesuai format
// "[2006-01-02 15:04:05] JOB abc123 → PrinterName | text | SUCCESS | 245ms".
func (b *Buffer) Add(entry LogEntry) {
	if entry.Timestamp.IsZero() {
		entry.Timestamp = time.Now()
	}
	b.mu.Lock()
	if len(b.entries) >= b.max {
		// Buang entry tertua (index 0) untuk menjaga ukuran maksimum.
		b.entries = append(b.entries[:0], b.entries[1:]...)
	}
	b.entries = append(b.entries, entry)
	b.mu.Unlock()

	printToTerminal(entry)
}

// GetAll mengembalikan salinan seluruh entry dengan urutan terbaru
// di indeks paling depan (newest first).
func (b *Buffer) GetAll() []LogEntry {
	b.mu.Lock()
	defer b.mu.Unlock()

	out := make([]LogEntry, len(b.entries))
	// Reverse copy: entry terakhir di slice = paling baru, harus jadi index 0.
	n := len(b.entries)
	for i := 0; i < n; i++ {
		out[i] = b.entries[n-1-i]
	}
	return out
}

// Len mengembalikan jumlah entry yang tersimpan saat ini.
func (b *Buffer) Len() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return len(b.entries)
}

// printToTerminal menulis ringkasan log ke stdout/stderr sesuai status.
func printToTerminal(e LogEntry) {
	short := e.JobID
	if len(short) > 8 {
		short = short[:8]
	}
	line := fmt.Sprintf("[%s] JOB %s -> %s | %s | %s | %dms",
		e.Timestamp.Format("2006-01-02 15:04:05"),
		short,
		e.Printer,
		e.Type,
		toUpper(e.Status),
		e.DurationMS,
	)
	if e.Message != "" {
		line += " | " + e.Message
	}
	if e.Status == StatusFailed {
		fmt.Fprintln(os.Stderr, line)
	} else {
		fmt.Fprintln(os.Stdout, line)
	}
}

// toUpper mengubah string ke huruf besar tanpa import strings tambahan
// untuk meminimalkan dependency footprint pada package ini.
func toUpper(s string) string {
	b := []byte(s)
	for i := 0; i < len(b); i++ {
		if b[i] >= 'a' && b[i] <= 'z' {
			b[i] -= 32
		}
	}
	return string(b)
}
