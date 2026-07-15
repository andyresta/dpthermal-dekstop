//go:build !windows

// Stub untuk platform non-Windows. Fungsi sendWinspoolRaw tidak akan
// dipanggil di Linux/macOS karena dispatch di sendSpooler memilih
// spoolUnix untuk OS tersebut. Stub ini hanya memastikan build
// cross-platform tetap sukses tanpa menarik paket windows.

package printer

import (
	"context"
	"fmt"
	"runtime"
)

// sendWinspoolRaw pada platform non-Windows selalu mengembalikan error
// "tidak didukung" agar bila ada kekeliruan rute, error-nya jelas.
func sendWinspoolRaw(_ context.Context, _ string, _ []byte) error {
	return fmt.Errorf("winspool tidak didukung di OS %s", runtime.GOOS)
}
