// PrintBridge — Service Bridge Printer
//
// Entry point yang melakukan:
//  1. Memuat config.json dari direktori binary (atau membuat default)
//  2. Scan port 8080–9090 dan bind ke port pertama yang tersedia
//  3. Menjalankan deteksi printer di goroutine background
//  4. Menjalankan HTTP server (net/http) dengan graceful shutdown
//  5. Membuka browser otomatis ke URL aplikasi
package main

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"sync"
	"syscall"
	"time"

	"printbridge/internal/config"
	"printbridge/internal/helper"
	"printbridge/internal/logger"
	"printbridge/internal/printer"
	"printbridge/internal/server"
)

const (
	appName    = "PrintBridge"
	appVersion = "v1.0.0"
	portStart  = 8080
	portEnd    = 9090
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "FATAL:", err)
		os.Exit(1)
	}
}

// run berisi keseluruhan lifecycle aplikasi. Dipisah dari main agar
// mudah ditest dan agar defer Cleanup tetap berjalan saat error.
func run() error {
	binDir := helper.BinaryDir()

	cfgMgr, err := config.NewManager(binDir)
	if err != nil {
		return fmt.Errorf("gagal load config: %w", err)
	}

	logBuf := logger.NewBuffer(logger.MaxEntries)
	printerMgr := printer.NewManager()

	port, err := helper.FindAvailablePort(portStart, portEnd)
	if err != nil {
		return err
	}

	srv := server.New(cfgMgr, logBuf, printerMgr)
	httpSrv := &http.Server{
		Addr:              net.JoinHostPort("0.0.0.0", strconv.Itoa(port)),
		Handler:           srv.Routes(),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       60 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	// Goroutine deteksi printer awal (non-blocking startup).
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		if err := printerMgr.Refresh(ctx); err != nil {
			fmt.Fprintln(os.Stderr, "[printer-detect] warning:", err)
		} else {
			fmt.Fprintf(os.Stdout, "[printer-detect] %d printer terdeteksi\n", len(printerMgr.List()))
		}
	}()

	url := fmt.Sprintf("http://localhost:%d", port)
	printStartupBanner(port, cfgMgr.Get(), cfgMgr.Path())

	// Jalankan HTTP server di goroutine terpisah agar main goroutine
	// dapat menangani signal shutdown.
	serverErr := make(chan error, 1)
	go func() {
		fmt.Printf("[%s] Listening on %s\n", appName, url)
		if err := httpSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErr <- err
		}
	}()

	// Buka browser sebentar setelah server siap (jeda kecil agar
	// listener sudah benar-benar accept koneksi).
	go func() {
		time.Sleep(400 * time.Millisecond)
		if err := helper.OpenBrowser(url); err != nil {
			fmt.Fprintln(os.Stderr, "[browser] gagal membuka browser otomatis:", err)
		}
	}()

	// Tangani SIGINT/SIGTERM untuk graceful shutdown.
	stopCh := make(chan os.Signal, 1)
	signal.Notify(stopCh, os.Interrupt, syscall.SIGTERM)

	select {
	case sig := <-stopCh:
		fmt.Printf("\n[%s] Menerima sinyal %s, mematikan server (max 10s)...\n", appName, sig)
	case err := <-serverErr:
		return fmt.Errorf("HTTP server error: %w", err)
	}

	// Drain pending request maksimum 10 detik.
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := httpSrv.Shutdown(shutdownCtx); err != nil {
		fmt.Fprintln(os.Stderr, "[shutdown] error:", err)
	}

	// Tunggu goroutine deteksi selesai (tidak terlalu lama).
	doneCh := make(chan struct{})
	go func() { wg.Wait(); close(doneCh) }()
	select {
	case <-doneCh:
	case <-time.After(2 * time.Second):
	}

	fmt.Printf("[%s] Bye.\n", appName)
	return nil
}

// printStartupBanner menampilkan banner ASCII saat startup yang
// memuat info versi, URL, lokasi config, dan default printer.
func printStartupBanner(port int, cfg config.Config, cfgPath string) {
	defaultPrinter := cfg.DefaultPrinter
	if defaultPrinter == "" {
		defaultPrinter = "N/A"
	}
	url := fmt.Sprintf("http://localhost:%d", port)
	lines := []string{
		"",
		"+----------------------------------------------+",
		fmt.Sprintf("|  %-44s |", appName+" "+appVersion),
		fmt.Sprintf("|  %-44s |", url),
		fmt.Sprintf("|  Config: %-36s |", trimTo(cfgPath, 36)),
		fmt.Sprintf("|  Default Printer: %-27s |", trimTo(defaultPrinter, 27)),
		fmt.Sprintf("|  Paper Width: %-31s |", fmt.Sprintf("%d mm", cfg.DefaultPaperWidthMM)),
		fmt.Sprintf("|  Type: %-38s |", trimTo(cfg.PrinterType, 38)),
		"+----------------------------------------------+",
		"",
	}
	for _, l := range lines {
		fmt.Println(l)
	}
}

// trimTo memotong string agar panjangnya tidak melebihi max,
// menambahkan elipsis bila perlu, untuk kebutuhan banner ASCII.
func trimTo(s string, max int) string {
	if len(s) <= max {
		return s
	}
	if max <= 3 {
		return s[:max]
	}
	return s[:max-3] + "..."
}
