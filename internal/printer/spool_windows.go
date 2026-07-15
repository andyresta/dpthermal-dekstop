//go:build windows

// File ini berisi implementasi pengiriman data RAW ke printer Windows
// melalui panggilan langsung ke WinSpool API (winspool.drv).
//
// Pendekatan ini menggantikan jalur lama yang memanggil PowerShell +
// Add-Type (.NET C# inline) pada tiap cetak. Dengan panggilan langsung:
//   - Tidak ada startup proses PowerShell (hemat ratusan ms s/d >1 detik
//     pada cetak pertama karena PowerShell+JIT .NET di-skip).
//   - Tidak ada kompilasi RawPrinterHelper C# via Add-Type.
//   - Tidak ada file sementara: payload dikirim langsung dari memory.
//
// API yang dipanggil: OpenPrinterW, StartDocPrinterW, StartPagePrinter,
// WritePrinter, EndPagePrinter, EndDocPrinter, ClosePrinter. Datatype
// selalu "RAW" agar stream ESC/POS atau plain text dikirim apa adanya
// tanpa konversi oleh driver.

package printer

import (
	"context"
	"fmt"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

// docInfo1W adalah binding Go untuk struktur DOC_INFO_1W WinSpool.
// Semua pointer adalah wide string (UTF-16) karena kita memakai
// varian -W (Unicode) dari API agar nama printer dengan karakter
// non-ASCII tetap aman.
type docInfo1W struct {
	DocName    *uint16
	OutputFile *uint16
	Datatype   *uint16
}

// Lazy-load winspool.drv dan proc-nya sekali saja per proses.
// NewLazySystemDLL memastikan DLL diambil dari %SystemRoot%\System32
// (mencegah DLL hijack dari direktori kerja).
var (
	winspool              = windows.NewLazySystemDLL("winspool.drv")
	procOpenPrinterW      = winspool.NewProc("OpenPrinterW")
	procClosePrinter      = winspool.NewProc("ClosePrinter")
	procStartDocPrinterW  = winspool.NewProc("StartDocPrinterW")
	procEndDocPrinter     = winspool.NewProc("EndDocPrinter")
	procStartPagePrinter  = winspool.NewProc("StartPagePrinter")
	procEndPagePrinter    = winspool.NewProc("EndPagePrinter")
	procWritePrinter      = winspool.NewProc("WritePrinter")
)

// sendWinspoolRaw mengirim payload mentah (RAW) ke printer Windows
// dengan nama printerName. Pemanggilan dibungkus goroutine + context
// agar timeout/cancel dari handler tetap dihormati. Catatan: Go tidak
// bisa membatalkan syscall yang sedang berjalan; jika driver benar-
// benar hang, fungsi akan kembali saat ctx terlewati dan goroutine
// pekerja selesai sendiri ketika driver merespons.
func sendWinspoolRaw(ctx context.Context, printerName string, payload []byte) error {
	if printerName == "" {
		return fmt.Errorf("nama printer kosong")
	}
	if len(payload) == 0 {
		return fmt.Errorf("payload kosong")
	}

	done := make(chan error, 1)
	go func() {
		// ctx diteruskan agar loop WritePrinter bisa cek ctx.Done()
		// di antara iterasi partial write.
		done <- rawPrintViaWinspool(ctx, printerName, payload)
	}()

	select {
	case err := <-done:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

// rawPrintViaWinspool melakukan sekuens lengkap: OpenPrinter -> StartDoc
// -> StartPage -> WritePrinter -> EndPage -> EndDoc -> ClosePrinter.
// ctx dipakai untuk cek pembatalan di antara iterasi partial write.
// Setiap error dipetakan ke pesan Bahasa Indonesia supaya log /api/logs
// tetap mudah dibaca oleh pengguna.
func rawPrintViaWinspool(ctx context.Context, printerName string, payload []byte) error {
	pnPtr, err := syscall.UTF16PtrFromString(printerName)
	if err != nil {
		return fmt.Errorf("nama printer tidak valid: %w", err)
	}

	var hPrinter windows.Handle
	r1, _, callErr := procOpenPrinterW.Call(
		uintptr(unsafe.Pointer(pnPtr)),
		uintptr(unsafe.Pointer(&hPrinter)),
		0,
	)
	if r1 == 0 {
		return fmt.Errorf("gagal buka printer %q: %s", printerName, translateWinErr(callErr))
	}
	defer procClosePrinter.Call(uintptr(hPrinter))

	docName, _ := syscall.UTF16PtrFromString("DPThermal RAW")
	datatype, _ := syscall.UTF16PtrFromString("RAW")
	di := docInfo1W{
		DocName:    docName,
		OutputFile: nil,
		Datatype:   datatype,
	}

	r1, _, callErr = procStartDocPrinterW.Call(
		uintptr(hPrinter),
		uintptr(1),
		uintptr(unsafe.Pointer(&di)),
	)
	if r1 == 0 {
		return fmt.Errorf("gagal memulai dokumen cetak: %s", translateWinErr(callErr))
	}
	defer procEndDocPrinter.Call(uintptr(hPrinter))

	r1, _, callErr = procStartPagePrinter.Call(uintptr(hPrinter))
	if r1 == 0 {
		return fmt.Errorf("gagal memulai halaman cetak: %s", translateWinErr(callErr))
	}
	defer procEndPagePrinter.Call(uintptr(hPrinter))

	// WritePrinter menerima pointer ke buffer byte + panjangnya dan
	// mengisi "written" dengan jumlah byte yang benar-benar ditulis.
	// Loop menangani partial write (langka tapi dimungkinkan oleh API).
	// Batas maxWriteIter mencegah loop tak terbatas saat driver hang.
	const maxWriteIter = 1000
	var (
		totalWritten uint32
		iter         int
	)
	for totalWritten < uint32(len(payload)) {
		if iter >= maxWriteIter {
			return fmt.Errorf(
				"WritePrinter: terlalu banyak iterasi (%d) saat mengirim %d/%d byte; "+
					"kemungkinan driver printer hang atau printer offline",
				maxWriteIter, totalWritten, len(payload),
			)
		}
		iter++

		// Cek context di antara iterasi agar timeout dari HTTP handler
		// (30 detik) tetap dihormati meski syscall tidak bisa dicancel.
		select {
		case <-ctx.Done():
			return fmt.Errorf("WritePrinter dibatalkan setelah %d/%d byte: %w",
				totalWritten, len(payload), ctx.Err())
		default:
		}

		var written uint32
		chunk := payload[totalWritten:]
		r1, _, callErr = procWritePrinter.Call(
			uintptr(hPrinter),
			uintptr(unsafe.Pointer(&chunk[0])),
			uintptr(len(chunk)),
			uintptr(unsafe.Pointer(&written)),
		)
		if r1 == 0 {
			return fmt.Errorf("gagal menulis ke printer: %s", translateWinErr(callErr))
		}
		if written == 0 {
			return fmt.Errorf("printer menolak menulis (0 byte); coba cek status printer")
		}
		totalWritten += written
	}

	return nil
}

// translateWinErr mengubah error syscall/Win32 menjadi pesan singkat
// dalam Bahasa Indonesia untuk kasus-kasus yang umum ditemui ketika
// mencetak. Untuk error yang tidak dikenali, pesan asli dikembalikan.
func translateWinErr(err error) string {
	if err == nil {
		return "error tidak diketahui"
	}
	errno, ok := err.(syscall.Errno)
	if !ok {
		return err.Error()
	}
	switch errno {
	case 0:
		// r1 = 0 tapi GetLastError tidak menyimpan kode. Jarang,
		// biasanya karena parameter invalid atau printer tidak ada.
		return "operasi gagal (tanpa kode error)"
	case 5: // ERROR_ACCESS_DENIED
		return "akses ditolak (5): periksa hak akses printer / user"
	case 1801: // ERROR_INVALID_PRINTER_NAME
		return "nama printer tidak valid (1801)"
	case 1722: // RPC_S_SERVER_UNAVAILABLE
		return "layanan Print Spooler tidak tersedia (1722)"
	case 1721: // RPC_S_CALL_FAILED
		return "pemanggilan spooler gagal (1721); cek service Print Spooler"
	case 87: // ERROR_INVALID_PARAMETER
		return "parameter tidak valid (87)"
	case 2: // ERROR_FILE_NOT_FOUND
		return "printer tidak ditemukan (2)"
	case 6: // ERROR_INVALID_HANDLE
		return "handle printer tidak valid (6)"
	case 1804: // ERROR_INVALID_DATATYPE
		return "datatype tidak didukung (1804); printer mungkin tidak menerima RAW"
	}
	return fmt.Sprintf("%v (%d)", errno.Error(), uint32(errno))
}
