// Package helper menyediakan fungsi-fungsi utilitas umum yang digunakan
// di seluruh aplikasi PrintBridge: pembuatan UUID, scan port,
// pembukaan browser, serta pencarian direktori binary.
package helper

import (
	"crypto/rand"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
)

// NewUUIDv4 menghasilkan string UUID versi 4 (RFC 4122) menggunakan crypto/rand.
// Dipakai sebagai identifier unik untuk setiap job pencetakan.
func NewUUIDv4() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		// Fallback ke timestamp bila crypto/rand gagal (sangat jarang).
		return fmt.Sprintf("ts-%d", time.Now().UnixNano())
	}
	// Set versi (4) dan variant (10) sesuai RFC 4122.
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:])
}

// ShortID mengembalikan 8 karakter pertama dari sebuah UUID untuk
// kebutuhan tampilan ringkas pada log dan UI.
func ShortID(id string) string {
	clean := strings.ReplaceAll(id, "-", "")
	if len(clean) >= 8 {
		return clean[:8]
	}
	return clean
}

// FindAvailablePort melakukan scan port secara sekuensial dari startPort
// hingga endPort (inklusif) dan mengembalikan port pertama yang tersedia.
// Mengembalikan error bila tidak ada port tersedia di rentang tersebut.
func FindAvailablePort(startPort, endPort int) (int, error) {
	for p := startPort; p <= endPort; p += 5 {
		addr := net.JoinHostPort("127.0.0.1", strconv.Itoa(p))
		ln, err := net.Listen("tcp", addr)
		if err != nil {
			continue
		}
		_ = ln.Close()
		return p, nil
	}
	return 0, fmt.Errorf("tidak ada port tersedia di rentang %d-%d dengan pola +5", startPort, endPort)
}

// OpenBrowser membuka URL di browser default sistem operasi.
// Mendukung Windows (start), macOS (open), dan Linux (xdg-open).
// Tidak memblokir; error dikembalikan bila perintah tidak bisa dijalankan.
func OpenBrowser(url string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		// rundll32 lebih reliable daripada "start" yang merupakan builtin cmd.
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	case "darwin":
		cmd = exec.Command("open", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	return cmd.Start()
}

// BinaryDir mengembalikan direktori absolut tempat binary yang sedang
// berjalan berada. Digunakan untuk menentukan lokasi config.json.
// Bila gagal, fallback ke working directory saat ini.
func BinaryDir() string {
	exe, err := os.Executable()
	if err == nil {
		// EvalSymlinks supaya symlink terselesaikan dengan benar.
		if real, err2 := filepath.EvalSymlinks(exe); err2 == nil {
			return filepath.Dir(real)
		}
		return filepath.Dir(exe)
	}
	wd, err := os.Getwd()
	if err != nil {
		return "."
	}
	return wd
}

// AtomicWriteFile menulis data ke path tujuan secara atomik dengan cara
// menulis ke file sementara terlebih dahulu lalu melakukan os.Rename.
// Menjamin file tujuan tidak pernah berada dalam keadaan setengah ditulis
// bila proses tiba-tiba mati.
func AtomicWriteFile(path string, data []byte, perm os.FileMode) error {
	dir := filepath.Dir(path)
	tmp := filepath.Join(dir, filepath.Base(path)+".tmp")
	if err := os.WriteFile(tmp, data, perm); err != nil {
		return fmt.Errorf("gagal menulis file sementara: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("gagal rename file: %w", err)
	}
	return nil
}
