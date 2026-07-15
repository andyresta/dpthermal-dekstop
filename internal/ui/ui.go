// Package ui menyimpan HTML single-page UI PrintBridge sebagai string
// constant Go murni. Pendekatan ini sengaja dipakai (alih-alih
// //go:embed file terpisah) agar binary final benar-benar self-contained
// tanpa membutuhkan file pendamping ketika di-distribusikan.
package ui

// IndexHTML berisi seluruh markup HTML + CSS + JavaScript yang
// digunakan untuk halaman utama PrintBridge. Tidak ada CDN eksternal;
// semua aset di-inline.
const IndexHTML = `<!DOCTYPE html>
<html lang="id">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>PrintBridge - Service Bridge Printer</title>
<style>
  * { box-sizing: border-box; margin: 0; padding: 0; }
  :root {
    --bg: #0f1419;
    --bg-elev: #1a1f2e;
    --bg-card: #232a3d;
    --border: #2d3548;
    --text: #e6e9ef;
    --text-dim: #8a93a6;
    --accent: #4f8cff;
    --accent-hover: #6ba0ff;
    --success: #3fb950;
    --warning: #d29922;
    --danger: #f85149;
    --shadow: 0 4px 12px rgba(0,0,0,0.35);
  }
  html, body {
    background: var(--bg);
    color: var(--text);
    font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, "Helvetica Neue", Arial, sans-serif;
    min-height: 100vh;
    font-size: 14px;
    line-height: 1.5;
  }
  a { color: var(--accent); text-decoration: none; }
  .container {
    max-width: 1100px;
    margin: 0 auto;
    padding: 24px 16px 80px 16px;
  }
  header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 12px 0 24px 0;
    border-bottom: 1px solid var(--border);
    margin-bottom: 24px;
  }
  header h1 {
    font-size: 20px;
    font-weight: 600;
    letter-spacing: 0.3px;
    display: flex;
    align-items: center;
    gap: 10px;
  }
  header h1 .dot {
    width: 10px; height: 10px; border-radius: 50%;
    background: var(--success);
    box-shadow: 0 0 8px var(--success);
  }
  header .ver { color: var(--text-dim); font-size: 12px; }
  .tabs {
    display: flex;
    gap: 4px;
    margin-bottom: 20px;
    background: var(--bg-elev);
    border: 1px solid var(--border);
    padding: 4px;
    border-radius: 10px;
    overflow-x: auto;
  }
  .tab {
    flex: 1;
    min-width: 130px;
    padding: 10px 14px;
    text-align: center;
    cursor: pointer;
    border-radius: 7px;
    font-weight: 500;
    color: var(--text-dim);
    background: transparent;
    border: none;
    font-size: 14px;
    transition: all 0.15s ease;
  }
  .tab:hover { color: var(--text); background: rgba(255,255,255,0.04); }
  .tab.active {
    color: #fff;
    background: var(--accent);
    box-shadow: var(--shadow);
  }
  .panel { display: none; }
  .panel.active { display: block; }
  .card {
    background: var(--bg-card);
    border: 1px solid var(--border);
    border-radius: 10px;
    padding: 20px;
    box-shadow: var(--shadow);
    margin-bottom: 16px;
  }
  .card h2 {
    font-size: 16px;
    font-weight: 600;
    margin-bottom: 16px;
    color: var(--text);
    display: flex;
    align-items: center;
    gap: 10px;
  }
  .card h2 .badge { font-size: 11px; padding: 2px 8px; border-radius: 10px; background: var(--accent); color: #fff; font-weight: 500; }
  .row { display: flex; flex-wrap: wrap; gap: 12px; align-items: center; margin-bottom: 14px; }
  .row > label { color: var(--text-dim); min-width: 130px; font-size: 13px; }
  .row > .grow { flex: 1; min-width: 220px; }
  input[type="text"], input[type="number"], textarea, select {
    background: var(--bg-elev);
    color: var(--text);
    border: 1px solid var(--border);
    padding: 9px 12px;
    border-radius: 7px;
    font-size: 13px;
    font-family: inherit;
    width: 100%;
    outline: none;
    transition: border-color 0.15s ease;
  }
  input:focus, textarea:focus, select:focus { border-color: var(--accent); }
  textarea { min-height: 110px; resize: vertical; font-family: ui-monospace, "Cascadia Code", "Consolas", monospace; }
  input[type="file"] { color: var(--text-dim); font-size: 12px; }
  input[type="checkbox"], input[type="radio"] { accent-color: var(--accent); width: 16px; height: 16px; cursor: pointer; }
  .checkbox-row, .radio-row { display: flex; align-items: center; gap: 8px; cursor: pointer; }
  .radio-group { display: flex; gap: 16px; }
  button {
    background: var(--accent);
    color: #fff;
    border: none;
    padding: 9px 18px;
    border-radius: 7px;
    cursor: pointer;
    font-size: 13px;
    font-weight: 500;
    transition: background 0.15s ease;
  }
  button:hover:not(:disabled) { background: var(--accent-hover); }
  button:disabled { opacity: 0.5; cursor: not-allowed; }
  button.secondary { background: var(--bg-elev); border: 1px solid var(--border); color: var(--text); }
  button.secondary:hover:not(:disabled) { background: var(--border); }
  button.danger { background: var(--danger); }
  .btn-group { display: flex; gap: 10px; flex-wrap: wrap; }
  .status-pill {
    display: inline-flex;
    align-items: center;
    gap: 6px;
    padding: 4px 10px;
    border-radius: 12px;
    font-size: 12px;
    font-weight: 500;
    background: var(--bg-elev);
    border: 1px solid var(--border);
  }
  .status-pill.success { color: var(--success); border-color: rgba(63,185,80,0.4); }
  .status-pill.failed  { color: var(--danger);  border-color: rgba(248,81,73,0.4); }
  .status-pill.pending { color: var(--warning); border-color: rgba(210,153,34,0.4); }
  .active-printer-banner {
    display: flex; align-items: center; gap: 12px;
    padding: 12px 14px;
    background: linear-gradient(180deg, rgba(79,140,255,0.15), rgba(79,140,255,0.05));
    border: 1px solid rgba(79,140,255,0.35);
    border-radius: 8px;
    margin-bottom: 16px;
    font-size: 13px;
  }
  .active-printer-banner .label { color: var(--text-dim); font-size: 12px; text-transform: uppercase; letter-spacing: 0.5px; }
  .active-printer-banner .value { font-weight: 600; }
  table { width: 100%; border-collapse: collapse; font-size: 12.5px; }
  th, td { padding: 10px 8px; text-align: left; border-bottom: 1px solid var(--border); }
  th { color: var(--text-dim); font-weight: 500; text-transform: uppercase; font-size: 11px; letter-spacing: 0.5px; background: var(--bg-elev); position: sticky; top: 0; }
  tbody tr:hover { background: rgba(255,255,255,0.02); }
  .table-wrap { overflow-x: auto; max-height: 480px; overflow-y: auto; border: 1px solid var(--border); border-radius: 8px; }
  td.success { color: var(--success); font-weight: 500; }
  td.failed  { color: var(--danger);  font-weight: 500; }
  td.pending { color: var(--warning); font-weight: 500; }
  td .mono { font-family: ui-monospace, "Cascadia Code", "Consolas", monospace; font-size: 11.5px; color: var(--text-dim); }
  .response-box {
    background: var(--bg-elev);
    border: 1px solid var(--border);
    border-radius: 7px;
    padding: 12px;
    font-family: ui-monospace, "Cascadia Code", "Consolas", monospace;
    font-size: 12px;
    margin-top: 12px;
    white-space: pre-wrap;
    word-break: break-word;
    max-height: 260px; overflow: auto;
  }
  .preview-box {
    background: var(--bg-elev);
    border: 1px dashed var(--border);
    border-radius: 7px;
    padding: 12px;
    margin-top: 8px;
    text-align: center;
  }
  .preview-box img { max-width: 200px; max-height: 200px; border-radius: 4px; }
  .toast-stack { position: fixed; top: 20px; right: 20px; display: flex; flex-direction: column; gap: 10px; z-index: 999; }
  .toast {
    background: var(--bg-card);
    border: 1px solid var(--border);
    border-left: 4px solid var(--accent);
    padding: 12px 16px;
    border-radius: 7px;
    box-shadow: var(--shadow);
    min-width: 280px;
    max-width: 380px;
    font-size: 13px;
    animation: slideIn 0.25s ease;
  }
  .toast.success { border-left-color: var(--success); }
  .toast.error   { border-left-color: var(--danger); }
  .toast.warning { border-left-color: var(--warning); }
  @keyframes slideIn { from { transform: translateX(20px); opacity: 0; } to { transform: translateX(0); opacity: 1; } }
  .empty { padding: 30px; text-align: center; color: var(--text-dim); }
  .footer-note { color: var(--text-dim); font-size: 11.5px; margin-top: 6px; }
  /* ===== Mode toggle (Thermal / Dot Matrix) ===== */
  .mode-toggle {
    display: inline-flex;
    gap: 4px;
    background: var(--bg-elev);
    border: 1px solid var(--border);
    padding: 3px;
    border-radius: 8px;
    margin-bottom: 16px;
  }
  .mode-btn {
    padding: 7px 18px;
    border-radius: 6px;
    font-size: 13px;
    font-weight: 500;
    color: var(--text-dim);
    background: transparent;
    border: none;
    cursor: pointer;
    transition: all 0.15s ease;
    white-space: nowrap;
  }
  .mode-btn:hover:not(:disabled) { color: var(--text); background: rgba(255,255,255,0.04); }
  .mode-btn.active {
    background: var(--bg-card);
    color: var(--text);
    box-shadow: 0 1px 5px rgba(0,0,0,0.35);
  }
  .mode-btn.active.thermal  { color: var(--accent); }
  .mode-btn.active.dotmatrix { color: var(--success); }
  .mode-notice {
    display: flex;
    align-items: flex-start;
    gap: 10px;
    padding: 11px 13px;
    background: rgba(210,153,34,0.08);
    border: 1px solid rgba(210,153,34,0.3);
    border-radius: 7px;
    font-size: 12.5px;
    color: var(--warning);
    line-height: 1.5;
    margin-bottom: 14px;
  }
  .printer-list-status {
    display: none;
    align-items: center;
    gap: 10px;
    padding: 10px 12px;
    margin-bottom: 14px;
    background: var(--bg-elev);
    border: 1px solid var(--border);
    border-radius: 7px;
    font-size: 13px;
    color: var(--text-dim);
  }
  .printer-list-status.visible { display: flex; }
  .printer-list-status .spinner {
    flex-shrink: 0;
    width: 18px; height: 18px;
    border: 2px solid var(--border);
    border-top-color: var(--accent);
    border-radius: 50%;
    animation: pb-spin 0.65s linear infinite;
  }
  @keyframes pb-spin { to { transform: rotate(360deg); } }
  /* ===== Dokumentasi API ===== */
  .doc-intro { color: var(--text-dim); font-size: 13px; margin-bottom: 20px; line-height: 1.6; }
  .doc-section { margin-bottom: 28px; padding-bottom: 24px; border-bottom: 1px solid var(--border); }
  .doc-section:last-child { border-bottom: none; margin-bottom: 0; padding-bottom: 0; }
  .doc-section h3 { font-size: 15px; font-weight: 600; margin-bottom: 10px; color: var(--text); display: flex; align-items: center; gap: 8px; flex-wrap: wrap; }
  .doc-section h4 { font-size: 13px; font-weight: 600; margin: 16px 0 8px; color: var(--accent); }
  .doc-section p { color: var(--text-dim); font-size: 13px; margin-bottom: 10px; line-height: 1.55; }
  .doc-badge { font-size: 10px; padding: 2px 7px; border-radius: 4px; background: var(--accent); color: #fff; font-weight: 600; }
  .doc-badge.new { background: var(--success); }
  .doc-pre {
    background: #0d1117;
    border: 1px solid var(--border);
    border-radius: 8px;
    padding: 14px 16px;
    font-family: ui-monospace, "Cascadia Code", "Consolas", monospace;
    font-size: 12px;
    line-height: 1.55;
    overflow-x: auto;
    white-space: pre-wrap;
    word-break: break-word;
    color: #c9d1d9;
    margin: 10px 0;
  }
  .doc-table { width: 100%; border-collapse: collapse; font-size: 12.5px; margin: 10px 0; }
  .doc-table th, .doc-table td { padding: 8px 10px; text-align: left; border: 1px solid var(--border); }
  .doc-table th { background: var(--bg-elev); color: var(--text-dim); font-weight: 500; }
  .doc-table td { color: var(--text); }
  .doc-table code { font-family: ui-monospace, monospace; font-size: 11.5px; color: var(--accent); }
  .doc-endpoint { font-family: ui-monospace, monospace; font-size: 13px; color: var(--success); background: rgba(63,185,80,0.1); padding: 2px 8px; border-radius: 4px; }
  .doc-note { padding: 10px 12px; background: rgba(79,140,255,0.08); border: 1px solid rgba(79,140,255,0.25); border-radius: 7px; font-size: 12.5px; color: var(--text-dim); margin: 12px 0; line-height: 1.5; }
  .doc-warn { padding: 10px 12px; background: rgba(210,153,34,0.08); border: 1px solid rgba(210,153,34,0.3); border-radius: 7px; font-size: 12.5px; color: var(--warning); margin: 12px 0; line-height: 1.5; }
  code { font-family: ui-monospace, monospace; font-size: 12px; color: var(--accent); background: rgba(79,140,255,0.1); padding: 1px 5px; border-radius: 3px; }
  @media (max-width: 720px) {
    .row > label { min-width: 100%; }
    .tabs { flex-wrap: wrap; }
    .tab { flex: 1 1 48%; }
  }
</style>
</head>
<body>

<div class="container">
  <header>
    <h1><span class="dot"></span> PrintBridge <span class="ver">v1.0.0</span></h1>
    <div class="status-pill" id="serverStatus">terhubung</div>
  </header>

  <div class="tabs" role="tablist">
    <button class="tab active" data-tab="settings">Pengaturan</button>
    <button class="tab" data-tab="logs">Log Cetak</button>
    <button class="tab" data-tab="testText">Test Cetak Teks</button>
    <button class="tab" data-tab="testImage">Test Cetak Gambar</button>
    <button class="tab" data-tab="testReceipt">Test Receipt</button>
    <button class="tab" data-tab="docs">Dokumentasi API</button>
  </div>

  <!-- ============ TAB: SETTINGS ============ -->
  <section class="panel active" id="panel-settings">
    <div class="active-printer-banner" id="activeBanner">
      <div>
        <div class="label">Printer Aktif</div>
        <div class="value" id="activePrinterText">Belum dikonfigurasi</div>
      </div>
    </div>

    <div class="card">
      <h2>Pengaturan Printer <span class="badge">Persisten</span></h2>
      <div class="row">
        <label>Pilih Printer</label>
        <div class="grow">
          <select id="printerSelect"><option value="">— Memuat printer —</option></select>
        </div>
      </div>
      <div id="printerListStatus" class="printer-list-status" role="status" aria-live="polite" aria-atomic="true">
        <span class="spinner" aria-hidden="true"></span>
        <span id="printerListStatusText">Memuat daftar printer…</span>
      </div>
      <div class="row">
        <label>Mode Printer</label>
        <div class="grow radio-group">
          <label class="radio-row">
            <input type="radio" name="printerMode" value="thermal" checked>
            <span>Thermal <span style="color:var(--text-dim);font-size:12px;">(ESC/POS — bitmap, logo, cut)</span></span>
          </label>
          <label class="radio-row">
            <input type="radio" name="printerMode" value="dotmatrix">
            <span>Dot Matrix <span style="color:var(--text-dim);font-size:12px;">(plain text — tanpa ESC/POS)</span></span>
          </label>
        </div>
      </div>
      <div class="row">
        <label>Lebar Kertas</label>
        <div class="grow radio-group">
          <label class="radio-row"><input type="radio" name="paperWidth" value="58"> 58 mm</label>
          <label class="radio-row"><input type="radio" name="paperWidth" value="80" checked> 80 mm</label>
        </div>
      </div>
      <div class="btn-group" style="margin-top: 8px;">
        <button id="btnSaveSettings">Simpan Pengaturan</button>
        <button id="btnRefreshPrinters" class="secondary">Refresh Daftar Printer</button>
      </div>
      <div class="footer-note">Tipe transport (Spooler / Bluetooth) otomatis mengikuti jenis printer yang dipilih. Tersimpan ke <code>config.json</code>.</div>
    </div>
  </section>

  <!-- ============ TAB: LOGS ============ -->
  <section class="panel" id="panel-logs">
    <div class="card">
      <h2>Log Cetak <span class="badge" id="logCount">0</span></h2>
      <div class="btn-group" style="margin-bottom: 12px;">
        <button id="btnRefreshLogs" class="secondary">Refresh Sekarang</button>
        <button id="btnClearDisplay" class="secondary">Bersihkan Tampilan</button>
        <span class="footer-note" style="align-self:center;">Auto-refresh setiap 5 detik</span>
      </div>
      <div class="table-wrap">
        <table id="logsTable">
          <thead>
            <tr>
              <th>Waktu</th><th>Job ID</th><th>Printer</th><th>Tipe</th>
              <th>Status</th><th>Durasi</th><th>Pesan</th>
            </tr>
          </thead>
          <tbody><tr><td colspan="7" class="empty">Belum ada log…</td></tr></tbody>
        </table>
      </div>
    </div>
  </section>

  <!-- ============ TAB: TEST TEXT ============ -->
  <section class="panel" id="panel-testText">
    <div class="card">
      <h2>Test Cetak Teks</h2>

      <!-- Mode toggle -->
      <div class="mode-toggle" id="textModeToggle" role="group" aria-label="Mode cetak teks">
        <button class="mode-btn thermal active" data-mode="thermal">Thermal (ESC/POS)</button>
        <button class="mode-btn dotmatrix" data-mode="dotmatrix">Dot Matrix / Plain Text</button>
      </div>

      <!-- Konten teks (selalu tampil) -->
      <div class="row">
        <label>Konten Teks</label>
        <div class="grow">
          <textarea id="textContent" placeholder="Enter text to print here..."></textarea>
        </div>
      </div>

      <!-- Opsi khusus Thermal -->
      <div id="textThermalOpts">
        <div class="row">
          <label>Logo (opsional)</label>
          <div class="grow">
            <input type="file" id="textLogoFile" accept="image/png,image/jpeg,image/bmp">
            <div class="preview-box" id="textLogoPreview" style="display:none;"><img id="textLogoImg"></div>
          </div>
        </div>
        <div class="row">
          <label>Posisi Logo</label>
          <div class="grow">
            <select id="textLogoPos">
              <option value="left">Kiri</option>
              <option value="center" selected>Tengah</option>
              <option value="right">Kanan</option>
            </select>
          </div>
        </div>
        <div class="row">
          <label>Cut Kertas</label>
          <div class="grow"><label class="checkbox-row"><input type="checkbox" id="textCut" checked> Potong kertas setelah cetak</label></div>
        </div>
      </div>

      <!-- Keterangan mode dot matrix -->
      <div class="mode-notice" id="textDotmatrixNotice" style="display:none;">
        Mode <strong>Dot Matrix / Plain Text</strong>: teks dikirim langsung sebagai byte polos tanpa ESC/POS.
        Logo, alignment, dan perintah cut paper tidak tersedia pada mode ini.
      </div>

      <div class="row">
        <label>Copies</label>
        <div class="grow"><input type="number" id="textCopies" value="1" min="1" max="10"></div>
      </div>
      <div class="btn-group">
        <button id="btnPrintText">Kirim Test Print</button>
      </div>
      <div class="response-box" id="textResponse" style="display:none;"></div>
      <div class="footer-note">Printer yang dipakai: printer aktif pada tab Pengaturan. Mode default diambil dari Pengaturan.</div>
    </div>
  </section>

  <!-- ============ TAB: TEST IMAGE ============ -->
  <section class="panel" id="panel-testImage">
    <div class="card">
      <h2>Test Cetak Gambar</h2>

      <!-- Mode toggle -->
      <div class="mode-toggle" id="imgModeToggle" role="group" aria-label="Mode cetak gambar">
        <button class="mode-btn thermal active" data-mode="thermal">Thermal (ESC/POS)</button>
        <button class="mode-btn dotmatrix" data-mode="dotmatrix">Dot Matrix / Plain Text</button>
      </div>

      <!-- Opsi Thermal -->
      <div id="imgThermalOpts">
        <div class="row">
          <label>File Gambar</label>
          <div class="grow">
            <input type="file" id="imgFile" accept="image/png,image/jpeg,image/bmp">
            <div class="preview-box" id="imgPreview" style="display:none;"><img id="imgPreviewImg"></div>
          </div>
        </div>
        <div class="row">
          <label>Dithering</label>
          <div class="grow">
            <select id="imgDither">
              <option value="none">None (Threshold)</option>
              <option value="floyd-steinberg" selected>Floyd-Steinberg</option>
              <option value="atkinson">Atkinson</option>
            </select>
          </div>
        </div>
        <div class="row">
          <label>Cut Kertas</label>
          <div class="grow"><label class="checkbox-row"><input type="checkbox" id="imgCut" checked> Potong kertas setelah cetak</label></div>
        </div>
        <div class="row">
          <label>Copies</label>
          <div class="grow"><input type="number" id="imgCopies" value="1" min="1" max="10"></div>
        </div>
      </div>

      <!-- Pesan mode dot matrix (gambar tidak didukung) -->
      <div class="mode-notice" id="imgDotmatrixNotice" style="display:none;">
        Mode <strong>Dot Matrix / Plain Text</strong> tidak mendukung cetak gambar raster (ESC/POS bitmap).
        Untuk mencetak gambar, pilih mode <strong>Thermal (ESC/POS)</strong>.
      </div>

      <div class="btn-group">
        <button id="btnPrintImage">Kirim Test Print</button>
      </div>
      <div class="response-box" id="imgResponse" style="display:none;"></div>
      <div class="footer-note">Printer & lebar kertas mengikuti konfigurasi pada tab Pengaturan. Mode default diambil dari Pengaturan.</div>
    </div>
  </section>

  <!-- ============ TAB: TEST RECEIPT ============ -->
  <section class="panel" id="panel-testReceipt">
    <div class="card">
      <h2>Test Cetak Receipt <span class="badge">POST /api/print/receipt</span></h2>
      <div class="footer-note" style="margin-bottom:12px;">Format payload selaras dengan MaxThermal Android — array <code>items</code> dengan type: text, line, qr, image, feed.</div>
      <div class="row">
        <label>Payload JSON</label>
        <div class="grow">
          <textarea id="receiptPayload" style="min-height:260px;font-family:monospace;font-size:12px;">{
  "items": [
    { "type": "text", "align": "center", "size": 2, "style": "bold", "data": "NAMA TOKO" },
    { "type": "text", "align": "center", "data": "Jl. Contoh No. 123" },
    { "type": "line", "style": "double" },
    { "type": "text", "data": "Item 1          x2   20.000" },
    { "type": "text", "data": "Item 2          x1   15.000" },
    { "type": "line" },
    { "type": "text", "align": "right", "size": 2, "style": "bold", "data": "TOTAL: Rp 35.000" },
    { "type": "feed", "data": "1" },
    { "type": "qr", "align": "center", "size": 200, "data": "https://toko.example.com/inv/12345" },
    { "type": "feed", "data": "1" },
    { "type": "text", "align": "center", "style": "bold", "data": "Terima kasih!" },
    { "type": "feed", "data": "3" }
  ],
  "cut_paper": true
}</textarea>
        </div>
      </div>
      <div class="row">
        <label>Sisipkan Gambar</label>
        <div class="grow">
          <input type="file" id="receiptImgFile" accept="image/png,image/jpeg,image/bmp">
          <div class="footer-note">Upload gambar untuk otomatis menambahkan item <code>image</code> ke payload.</div>
        </div>
      </div>
      <div class="row">
        <label>Cut Kertas</label>
        <div class="grow"><label class="checkbox-row"><input type="checkbox" id="receiptCut" checked> Potong kertas setelah cetak</label></div>
      </div>
      <div class="btn-group">
        <button id="btnPrintReceipt">Kirim Test Receipt</button>
      </div>
      <div class="response-box" id="receiptResponse" style="display:none;"></div>
    </div>

    <div class="card" style="margin-top:16px;">
      <h2>Panduan Singkat</h2>
      <p class="doc-intro" style="margin-bottom:12px;">Receipt adalah cetak nota terstruktur — kirim array <code>items</code> berisi perintah cetak (teks, garis, QR, gambar, feed). Urutan item = urutan di kertas.</p>
      <div class="doc-note">Gunakan tab <strong>Dokumentasi API</strong> untuk spesifikasi lengkap semua field, tipe item, dan contoh kode JavaScript.</div>
    </div>
  </section>

  <!-- ============ TAB: DOKUMENTASI API ============ -->
  <section class="panel" id="panel-docs">
    <div class="card">
      <h2>Dokumentasi API PrintBridge</h2>
      <p class="doc-intro">
        Spesifikasi teknis untuk integrasi website/aplikasi dengan service PrintBridge.
        Base URL: <code id="docBaseUrl">http://localhost:8080</code> (port mengikuti konfigurasi service).
        Semua endpoint print menggunakan method <strong>POST</strong> dengan header <code>Content-Type: application/json</code>.
        CORS diaktifkan (<code>Access-Control-Allow-Origin: *</code>).
      </p>

      <div class="doc-section">
        <h3>Health Check</h3>
        <p><span class="doc-endpoint">GET /check-bridge-print</span></p>
        <div class="doc-pre">Response: { "code": 0, "msg": "ready" }</div>
      </div>

      <div class="doc-section">
        <h3>1. Cetak Teks + Logo</h3>
        <p><span class="doc-endpoint">POST /api/print/text</span></p>
        <div class="doc-pre">{
  "text": "Nama Toko\\n================\\nTotal  Rp 50.000",
  "logo_base64": "data:image/png;base64,...",   // opsional
  "logo_position": "center",                     // left | center | right
  "copies": 1,
  "cut_paper": true,
  "mode": "thermal",                             // thermal | dotmatrix
  "width_mm": 58                                 // 58 | 80
}</div>
      </div>

      <div class="doc-section">
        <h3>2. Cetak Gambar</h3>
        <p><span class="doc-endpoint">POST /api/print/image</span></p>
        <div class="doc-pre">{
  "image_base64": "data:image/png;base64,...",
  "width_mm": 58,
  "dithering": "floyd-steinberg",   // none | floyd-steinberg | atkinson
  "copies": 1,
  "cut_paper": true,
  "mode": "thermal"
}</div>
        <div class="doc-warn">Mode <code>dotmatrix</code> tidak mendukung cetak gambar.</div>
      </div>

      <div class="doc-section">
        <h3>3. Cetak Receipt Terstruktur <span class="doc-badge new">NEW</span></h3>
        <p>
          <span class="doc-endpoint">POST /api/print/receipt</span>
          &nbsp;|&nbsp; alias kompatibilitas MaxThermal:
          <span class="doc-endpoint">POST /print/receipt</span>
        </p>
        <p>Cetak nota/struk dari array perintah cetak. Format payload selaras dengan MaxThermal Android.</p>
        <div class="doc-pre">{
  "items": [
    { "type": "text", "align": "center", "size": 2, "style": "bold", "data": "NAMA TOKO" },
    { "type": "text", "align": "center", "data": "Jl. Contoh No. 123" },
    { "type": "line", "style": "double" },
    { "type": "text", "data": "Item 1          x2   20.000" },
    { "type": "text", "data": "Item 2          x1   15.000" },
    { "type": "line" },
    { "type": "text", "align": "right", "size": 2, "style": "bold", "data": "TOTAL: Rp 35.000" },
    { "type": "feed", "data": "1" },
    { "type": "qr", "align": "center", "size": 200, "data": "https://toko.example.com/inv/12345" },
    { "type": "image", "align": "center", "data": "data:image/png;base64,..." },
    { "type": "text", "align": "center", "style": "bold", "data": "Terima kasih!" },
    { "type": "feed", "data": "3" }
  ],
  "cut_paper": true,
  "width_mm": 58,
  "copies": 1,
  "dithering": "floyd-steinberg",
  "mode": "thermal"
}</div>

        <h4>Field root (opsional)</h4>
        <table class="doc-table">
          <thead><tr><th>Field</th><th>Tipe</th><th>Default</th><th>Keterangan</th></tr></thead>
          <tbody>
            <tr><td><code>items</code></td><td>array</td><td>—</td><td><strong>Wajib.</strong> Daftar perintah cetak berurutan.</td></tr>
            <tr><td><code>cut_paper</code></td><td>boolean</td><td>false</td><td>Potong kertas setelah selesai cetak.</td></tr>
            <tr><td><code>width_mm</code></td><td>number</td><td>config</td><td>Lebar kertas: <code>58</code> atau <code>80</code>.</td></tr>
            <tr><td><code>copies</code></td><td>number</td><td>1</td><td>Jumlah salinan (maks 10).</td></tr>
            <tr><td><code>dithering</code></td><td>string</td><td>floyd-steinberg</td><td>Algoritma dither untuk item <code>image</code>.</td></tr>
            <tr><td><code>mode</code></td><td>string</td><td>config</td><td><code>thermal</code> (ESC/POS) atau <code>dotmatrix</code> (plain text).</td></tr>
          </tbody>
        </table>

        <h4>Tipe item yang didukung</h4>

        <p><strong>text</strong> — cetak baris teks dengan formatting.</p>
        <div class="doc-pre">{
  "type": "text",
  "align": "left | center | right",     // default: left
  "size": 1,                             // 1–8 (skala ukuran karakter)
  "style": "bold | underline | bold,underline",
  "data": "Teks yang dicetak"
}</div>
        <table class="doc-table">
          <thead><tr><th>size</th><th>Keterangan</th></tr></thead>
          <tbody>
            <tr><td><code>1</code></td><td>Normal (default)</td></tr>
            <tr><td><code>2</code></td><td>2× lebar &amp; tinggi</td></tr>
            <tr><td><code>3–8</code></td><td>Semakin besar, semakin besar teks</td></tr>
          </tbody>
        </table>

        <p><strong>line</strong> — garis pemisah horizontal.</p>
        <div class="doc-pre">{
  "type": "line",
  "style": "single | double | dotted"   // default: single
  // single = --------  (32 char @ 58mm, 48 @ 80mm)
  // double = ========
  // dotted = ........
}</div>

        <p><strong>qr</strong> — generate &amp; cetak QR Code.</p>
        <div class="doc-pre">{
  "type": "qr",
  "align": "left | center | right",     // default: center
  "size": 200,                           // resolusi generate QR (px), default 200
  "data": "https://contoh.com/inv/123"  // konten QR (URL/teks)
}</div>

        <p><strong>image</strong> — cetak gambar dari base64.</p>
        <div class="doc-pre">{
  "type": "image",
  "align": "left | center | right",     // default: center
  "data": "data:image/png;base64,..."   // PNG/JPEG/BMP base64
}</div>

        <p><strong>feed</strong> — baris kosong (feed kertas).</p>
        <div class="doc-pre">{
  "type": "feed",
  "data": "3"                           // jumlah baris, default 1, maks 10
}</div>

        <div class="doc-warn">Mode <code>dotmatrix</code>: hanya <code>text</code>, <code>line</code>, <code>feed</code> yang didukung. Item <code>qr</code> dan <code>image</code> akan ditolak.</div>
      </div>

      <div class="doc-section">
        <h3>Format Response</h3>
        <table class="doc-table">
          <thead><tr><th>HTTP</th><th>Response</th><th>Keterangan</th></tr></thead>
          <tbody>
            <tr><td>200</td><td><code>{ "success": true, "job_id": "...", "message": "..." }</code></td><td>Cetak berhasil</td></tr>
            <tr><td>400</td><td><code>{ "success": false, "message": "..." }</code></td><td>Payload tidak valid</td></tr>
            <tr><td>500</td><td><code>{ "success": false, "message": "..." }</code></td><td>Error saat cetak</td></tr>
          </tbody>
        </table>
      </div>

      <div class="doc-section">
        <h3>Contoh Implementasi JavaScript</h3>
        <div class="doc-pre">/**
 * Cetak receipt terstruktur ke PrintBridge.
 * @param {string} baseUrl - mis. 'http://localhost:8080'
 * @param {Array} items - array item receipt
 */
async function printReceipt(baseUrl, items) {
  const res = await fetch(baseUrl + '/api/print/receipt', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({
      items,
      cut_paper: true,
      width_mm: 58
    })
  });
  return await res.json();
}

// Contoh nota penjualan
printReceipt('http://localhost:8080', [
  { type: 'text', align: 'center', size: 2, style: 'bold', data: 'WARUNG MAKAN' },
  { type: 'text', align: 'center', data: 'Jl. Raya No. 1' },
  { type: 'line', style: 'double' },
  { type: 'text', data: 'Nasi Goreng  x1   15.000' },
  { type: 'text', data: 'Es Teh       x2   10.000' },
  { type: 'line' },
  { type: 'text', align: 'right', size: 2, style: 'bold', data: 'TOTAL Rp 25.000' },
  { type: 'feed', data: '1' },
  { type: 'qr', align: 'center', size: 200, data: 'https://pay.example/123' },
  { type: 'feed', data: '3' }
]).then(r => console.log(r));</div>
        <div class="doc-note">
          <strong>Kompatibilitas MaxThermal Android:</strong> payload <code>items</code> identik.
          Ganti URL ke <code>/print/receipt</code> jika memanggil service Android MaxThermal langsung.
        </div>
      </div>

      <div class="doc-section">
        <h3>Endpoint Lainnya</h3>
        <table class="doc-table">
          <thead><tr><th>Method</th><th>Endpoint</th><th>Fungsi</th></tr></thead>
          <tbody>
            <tr><td>GET/POST</td><td><code>/api/printers</code></td><td>Daftar printer terdeteksi</td></tr>
            <tr><td>POST</td><td><code>/api/printers/refresh</code></td><td>Refresh deteksi printer</td></tr>
            <tr><td>GET/POST</td><td><code>/api/config</code></td><td>Baca konfigurasi</td></tr>
            <tr><td>POST/PUT</td><td><code>/api/config</code></td><td>Update konfigurasi</td></tr>
            <tr><td>GET/POST</td><td><code>/api/logs</code></td><td>Riwayat job cetak</td></tr>
            <tr><td>POST</td><td><code>/api/printer/recover</code></td><td>Recovery printer macet (CUPS + ESC @)</td></tr>
          </tbody>
        </table>
      </div>
    </div>
  </section>

</div>

<div class="toast-stack" id="toastStack"></div>

<script>
'use strict';

const API_BASE = '';
let currentConfig = null;
let knownPrinters = [];
let textLogoBase64 = '';
let imageBase64 = '';
let logsTimer = null;
// Mode aktif masing-masing panel test: 'thermal' atau 'dotmatrix'.
let textPrintMode = 'thermal';
let imgPrintMode  = 'thermal';

// ============================
//  Utilitas umum
// ============================
function $(sel) { return document.querySelector(sel); }
function $$(sel) { return Array.from(document.querySelectorAll(sel)); }

function showToast(msg, kind) {
  const t = document.createElement('div');
  t.className = 'toast ' + (kind || '');
  t.textContent = msg;
  $('#toastStack').appendChild(t);
  setTimeout(() => { t.style.opacity = '0'; t.style.transition = 'opacity .3s'; }, 3000);
  setTimeout(() => t.remove(), 3400);
}

// Konvensi proyek: SEMUA call API/AJAX menggunakan method POST.
// Server PrintBridge sudah disiapkan untuk menerima POST sebagai alias
// dari GET/PUT pada endpoint yang relevan. Untuk endpoint /api/config:
//   - POST tanpa body  → read (sama seperti GET)
//   - POST dengan body → update (sama seperti PUT)
async function apiCall(path, body) {
  const opts = {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
  };
  if (body !== undefined && body !== null) opts.body = body;
  const res = await fetch(API_BASE + path, opts);
  let data;
  try { data = await res.json(); } catch(e) { data = { _raw: await res.text() }; }
  return { ok: res.ok, status: res.status, data };
}

function fileToBase64(file) {
  return new Promise((resolve, reject) => {
    const reader = new FileReader();
    reader.onload = () => {
      const result = reader.result;
      const idx = result.indexOf(',');
      resolve(idx >= 0 ? result.substring(idx + 1) : result);
    };
    reader.onerror = reject;
    reader.readAsDataURL(file);
  });
}

function fmtTime(iso) {
  try {
    const d = new Date(iso);
    return d.toLocaleString();
  } catch(e) { return iso; }
}
function shortId(id) {
  if (!id) return '-';
  return String(id).replace(/-/g,'').substring(0,8);
}

// ============================
//  Tab switching
// ============================
$$('.tab').forEach(btn => {
  btn.addEventListener('click', () => {
    const target = btn.dataset.tab;
    $$('.tab').forEach(t => t.classList.toggle('active', t === btn));
    $$('.panel').forEach(p => p.classList.toggle('active', p.id === 'panel-' + target));
    if (target === 'logs') { refreshLogs(); }
  });
});

// ============================
//  Settings: load/save
// ============================
async function loadConfig() {
  const r = await apiCall('/api/config');
  if (!r.ok) { showToast('Gagal memuat config: ' + (r.data.message || r.status), 'error'); return; }
  currentConfig = r.data;
  applyConfigToForm();
  updateActiveBanner();
}

// applyModeToggle menetapkan tombol aktif pada sebuah .mode-toggle container
// dan menampilkan/menyembunyikan panel opsi yang sesuai.
function applyModeToggle(toggleId, mode) {
  const toggle = $('#' + toggleId);
  if (!toggle) return;
  toggle.querySelectorAll('.mode-btn').forEach(btn => {
    btn.classList.toggle('active', btn.dataset.mode === mode);
  });
  if (toggleId === 'textModeToggle') {
    textPrintMode = mode;
    $('#textThermalOpts').style.display     = mode === 'thermal'   ? '' : 'none';
    $('#textDotmatrixNotice').style.display = mode === 'dotmatrix' ? '' : 'none';
  } else if (toggleId === 'imgModeToggle') {
    imgPrintMode = mode;
    $('#imgThermalOpts').style.display      = mode === 'thermal'   ? '' : 'none';
    $('#imgDotmatrixNotice').style.display  = mode === 'dotmatrix' ? '' : 'none';
    $('#btnPrintImage').disabled            = mode === 'dotmatrix';
  }
}

// Pasang event listener untuk semua mode-toggle di halaman.
$$('.mode-toggle').forEach(toggle => {
  toggle.addEventListener('click', e => {
    const btn = e.target.closest('.mode-btn');
    if (!btn) return;
    applyModeToggle(toggle.id, btn.dataset.mode);
  });
});

function applyConfigToForm() {
  if (!currentConfig) return;
  $$('input[name="paperWidth"]').forEach(r => { r.checked = (parseInt(r.value, 10) === currentConfig.default_paper_width_mm); });
  // Set radio printer mode.
  const cfgMode = currentConfig.printer_mode || 'thermal';
  $$('input[name="printerMode"]').forEach(r => { r.checked = (r.value === cfgMode); });
  // Sinkronkan mode toggle di tab test agar default = mode dari config.
  applyModeToggle('textModeToggle', cfgMode);
  applyModeToggle('imgModeToggle', cfgMode);
  // Pilih printer di dropdown jika ada.
  const sel = $('#printerSelect');
  if (sel && currentConfig.default_printer) {
    let found = false;
    Array.from(sel.options).forEach(o => { if (o.value === currentConfig.default_printer) found = true; });
    if (!found && currentConfig.default_printer) {
      const opt = document.createElement('option');
      opt.value = currentConfig.default_printer;
      opt.textContent = currentConfig.default_printer + ' (tidak terdeteksi)';
      sel.appendChild(opt);
    }
    sel.value = currentConfig.default_printer;
  }
}

// getPrinterTypeForName mengembalikan printer_type yang akan disimpan ke
// config: diambil dari hasil deteksi (knownPrinters) agar selalu selaras
// dengan pilihan di dropdown. Jika printer tidak ada di daftar (mis. tidak
// terdeteksi), gunakan nilai config saat ini bila namanya sama, jika tidak
// default spooler.
function getPrinterTypeForName(name) {
  if (!name) return 'spooler';
  const p = knownPrinters.find(function(x) { return x.name === name; });
  if (p && p.type === 'bluetooth') return 'bluetooth';
  if (p && p.type === 'spooler') return 'spooler';
  if (currentConfig && currentConfig.default_printer === name && currentConfig.printer_type === 'bluetooth') {
    return 'bluetooth';
  }
  return 'spooler';
}

function updateActiveBanner() {
  const txt = $('#activePrinterText');
  if (!currentConfig || !currentConfig.default_printer) {
    txt.innerHTML = 'Belum dikonfigurasi <span class="status-pill failed" style="margin-left:8px;">N/A</span>';
    return;
  }
  const info = knownPrinters.find(p => p.name === currentConfig.default_printer);
  const type = info ? info.type : currentConfig.printer_type;
  const status = info ? info.status : 'unknown';
  const statusClass = status === 'ready' ? 'success' : (status === 'offline' ? 'failed' : 'pending');
  const pmode = currentConfig.printer_mode || 'thermal';
  const pmodeLabel = pmode === 'dotmatrix' ? 'Dot Matrix' : 'Thermal';
  const pmodeColor = pmode === 'dotmatrix' ? 'color:var(--success)' : 'color:var(--accent)';
  txt.innerHTML = currentConfig.default_printer
    + ' <span class="status-pill" style="margin-left:8px;">' + (type||'-') + '</span>'
    + ' <span class="status-pill" style="margin-left:6px;' + pmodeColor + '">' + pmodeLabel + '</span>'
    + ' <span class="status-pill" style="margin-left:6px;">' + currentConfig.default_paper_width_mm + 'mm</span>'
    + ' <span class="status-pill ' + statusClass + '" style="margin-left:6px;">' + status + '</span>';
}

// isInitialLoad dipakai untuk membedakan teks loading saat startup
// vs saat klik tombol Refresh manual.
let isInitialLoad = true;

async function loadPrinters(refresh) {
  const showLoading = !!refresh;
  const statusEl = $('#printerListStatus');
  const statusText = $('#printerListStatusText');
  const btnRef = $('#btnRefreshPrinters');
  const sel = $('#printerSelect');

  // Simpan nilai yang harus dipilih setelah select di-enable kembali.
  // Harus dideklarasikan DI LUAR try/finally agar bisa diakses keduanya.
  // Latar belakang: browser Chromium me-reset pilihan select ke option
  // pertama saat elemen di-enable dari kondisi disabled. Oleh karena itu
  // sel.value harus di-set SETELAH sel.disabled = false.
  let pendingSelection = '';

  if (showLoading) {
    statusEl.classList.add('visible');
    statusText.textContent = isInitialLoad
      ? 'Mendeteksi printer saat startup (spooler & Bluetooth)…'
      : 'Mendeteksi ulang printer spooler (OS) dan Bluetooth… mohon tunggu.';
    isInitialLoad = false;
    btnRef.disabled = true;
    sel.disabled = true;
    sel.setAttribute('aria-busy', 'true');
  }

  try {
    const path = refresh ? '/api/printers/refresh' : '/api/printers';
    const r = await apiCall(path);
    if (!r.ok) {
      showToast('Gagal memuat printer: ' + (r.data.message || r.status), 'error');
      return;
    }
    knownPrinters = r.data.printers || [];

    // Tangkap nilai yang sedang terpilih SEBELUM list dibangun ulang.
    // Prioritas: nilai saat ini di sel → config → kosong.
    const prev = sel.value || (currentConfig && currentConfig.default_printer) || '';

    sel.innerHTML = '';
    if (knownPrinters.length === 0) {
      const opt = document.createElement('option');
      opt.value = ''; opt.textContent = '— Tidak ada printer terdeteksi —';
      sel.appendChild(opt);
    } else {
      const placeholder = document.createElement('option');
      placeholder.value = ''; placeholder.textContent = '— Pilih printer —';
      sel.appendChild(placeholder);
      knownPrinters.forEach(p => {
        const opt = document.createElement('option');
        opt.value = p.name;
        opt.textContent = p.name + ' [' + p.type + '] · ' + p.status;
        sel.appendChild(opt);
      });
    }

    // Pastikan opsi untuk prev tersedia di list (tambahkan bila perlu).
    if (prev) {
      const found = Array.from(sel.options).some(o => o.value === prev);
      if (!found) {
        const opt = document.createElement('option');
        opt.value = prev;
        opt.textContent = prev + ' (tidak terdeteksi)';
        sel.appendChild(opt);
      }
      // Simpan dulu; sel.value di-assign di finally SETELAH disabled=false.
      pendingSelection = prev;
    }

    if (refresh) {
      showToast('Daftar printer di-refresh', 'success');
      if (r.data.warning) showToast(String(r.data.warning), 'warning');
    }
  } finally {
    if (showLoading) {
      statusEl.classList.remove('visible');
      btnRef.disabled = false;
      sel.disabled = false;            // enable dulu
      sel.removeAttribute('aria-busy');
    }
    // Set value SETELAH enabled supaya Chromium tidak reset ke option pertama.
    if (pendingSelection) {
      sel.value = pendingSelection;
    }
    // Update banner dengan pilihan yang sudah final.
    updateActiveBanner();
  }
}

$('#printerSelect').addEventListener('change', function() { updateActiveBanner(); });

$('#btnSaveSettings').addEventListener('click', async () => {
  const printer = $('#printerSelect').value;
  const widthRadio = $$('input[name="paperWidth"]').find(r => r.checked);
  const modeRadio  = $$('input[name="printerMode"]').find(r => r.checked);
  const width = widthRadio ? parseInt(widthRadio.value, 10) : 80;
  const pmode = modeRadio ? modeRadio.value : 'thermal';
  const ptype = getPrinterTypeForName(printer);

  if (!printer) { showToast('Silakan pilih printer terlebih dahulu', 'warning'); return; }

  const payload = {
    default_printer: printer,
    default_paper_width_mm: width,
    printer_type: ptype,
    printer_mode: pmode,
  };
  const r = await apiCall('/api/config', JSON.stringify(payload));
  if (r.ok && r.data.success !== false) {
    currentConfig = r.data.config || payload;
    showToast('Settings tersimpan ke config.json', 'success');
    updateActiveBanner();
  } else {
    showToast('Gagal menyimpan: ' + (r.data.message || r.status), 'error');
  }
});

$('#btnRefreshPrinters').addEventListener('click', () => loadPrinters(true));

// ============================
//  Logs
// ============================
async function refreshLogs() {
  const r = await apiCall('/api/logs');
  if (!r.ok) return;
  renderLogs(r.data.logs || []);
}
function renderLogs(logs) {
  $('#logCount').textContent = logs.length;
  const tb = $('#logsTable tbody');
  if (!logs.length) {
    tb.innerHTML = '<tr><td colspan="7" class="empty">Belum ada log…</td></tr>';
    return;
  }
  const rows = logs.map(l => {
    const cls = l.status || 'pending';
    const dur = (l.duration_ms || 0) + ' ms';
    return '<tr>'
      + '<td><span class="mono">' + fmtTime(l.timestamp) + '</span></td>'
      + '<td><span class="mono">' + shortId(l.job_id) + '</span></td>'
      + '<td>' + escapeHTML(l.printer || '-') + '</td>'
      + '<td>' + escapeHTML(l.type || '-') + '</td>'
      + '<td class="' + cls + '">' + (l.status || '-').toUpperCase() + '</td>'
      + '<td><span class="mono">' + dur + '</span></td>'
      + '<td>' + escapeHTML(l.message || '') + '</td>'
      + '</tr>';
  });
  tb.innerHTML = rows.join('');
}
function escapeHTML(s) {
  return String(s == null ? '' : s)
    .replace(/&/g,'&amp;').replace(/</g,'&lt;').replace(/>/g,'&gt;')
    .replace(/"/g,'&quot;').replace(/'/g,'&#39;');
}

$('#btnRefreshLogs').addEventListener('click', refreshLogs);
$('#btnClearDisplay').addEventListener('click', () => {
  $('#logsTable tbody').innerHTML = '<tr><td colspan="7" class="empty">Tampilan dibersihkan (server tetap menyimpan log).</td></tr>';
  $('#logCount').textContent = '0';
});

function startLogsAutoRefresh() {
  if (logsTimer) clearInterval(logsTimer);
  logsTimer = setInterval(() => {
    if ($('#panel-logs').classList.contains('active')) refreshLogs();
  }, 5000);
}

// ============================
//  Test Text
// ============================
$('#textLogoFile').addEventListener('change', async (e) => {
  const f = e.target.files[0];
  if (!f) { textLogoBase64 = ''; $('#textLogoPreview').style.display = 'none'; return; }
  textLogoBase64 = await fileToBase64(f);
  $('#textLogoImg').src = 'data:' + (f.type || 'image/png') + ';base64,' + textLogoBase64;
  $('#textLogoPreview').style.display = 'block';
});

$('#btnPrintText').addEventListener('click', async () => {
  const txt = $('#textContent').value;
  if (!txt && !(textLogoBase64 && textPrintMode === 'thermal')) {
    showToast('Isi konten teks terlebih dahulu', 'warning'); return;
  }
  const payload = {
    text: txt,
    logo_base64:   textPrintMode === 'thermal' ? (textLogoBase64 || '') : '',
    logo_position: textPrintMode === 'thermal' ? $('#textLogoPos').value : 'center',
    copies:   parseInt($('#textCopies').value, 10) || 1,
    cut_paper: textPrintMode === 'thermal' && $('#textCut').checked,
    encoding: 'utf-8',
    mode: textPrintMode,
  };
  const btn = $('#btnPrintText'); btn.disabled = true;
  try {
    const r = await apiCall('/api/print/text', JSON.stringify(payload));
    const box = $('#textResponse'); box.style.display = 'block';
    box.textContent = JSON.stringify(r.data, null, 2);
    if (r.ok && r.data.success) showToast('Job teks terkirim', 'success');
    else showToast('Gagal: ' + (r.data.message || r.status), 'error');
  } finally { btn.disabled = false; }
});

// ============================
//  Test Image
// ============================
$('#imgFile').addEventListener('change', async (e) => {
  const f = e.target.files[0];
  if (!f) { imageBase64 = ''; $('#imgPreview').style.display = 'none'; return; }
  imageBase64 = await fileToBase64(f);
  $('#imgPreviewImg').src = 'data:' + (f.type || 'image/png') + ';base64,' + imageBase64;
  $('#imgPreview').style.display = 'block';
});

$('#btnPrintImage').addEventListener('click', async () => {
  if (imgPrintMode === 'dotmatrix') {
    showToast('Cetak gambar tidak didukung pada mode Dot Matrix', 'warning'); return;
  }
  if (!imageBase64) { showToast('Pilih file gambar terlebih dahulu', 'warning'); return; }
  const payload = {
    image_base64: imageBase64,
    width_mm: currentConfig ? currentConfig.default_paper_width_mm : 80,
    dithering: $('#imgDither').value,
    copies: parseInt($('#imgCopies').value, 10) || 1,
    cut_paper: $('#imgCut').checked,
    mode: imgPrintMode,
  };
  const btn = $('#btnPrintImage'); btn.disabled = true;
  try {
    const r = await apiCall('/api/print/image', JSON.stringify(payload));
    const box = $('#imgResponse'); box.style.display = 'block';
    box.textContent = JSON.stringify(r.data, null, 2);
    if (r.ok && r.data.success) showToast('Job gambar terkirim', 'success');
    else showToast('Gagal: ' + (r.data.message || r.status), 'error');
  } finally { btn.disabled = false; }
});

// ============================
//  Test Receipt
// ============================
$('#receiptImgFile').addEventListener('change', async (e) => {
  const f = e.target.files[0];
  if (!f) return;
  const b64 = await fileToBase64(f);
  const dataUrl = 'data:' + (f.type || 'image/png') + ';base64,' + b64;
  let ta = $('#receiptPayload');
  let val = ta.value;
  const imageItem = '{ "type": "image", "align": "center", "data": "' + dataUrl + '" }';
  if (val.includes('"type": "image"')) {
    val = val.replace(/\{ "type": "image"[^}]+\}/, imageItem);
  } else {
    val = val.replace(/(\s*\]\s*,?\s*"cut_paper")/, ',\n    ' + imageItem + '\n  $1');
    if (!val.includes('"type": "image"')) {
      val = val.replace(/\n  \]/, ',\n    ' + imageItem + '\n  ]');
    }
  }
  ta.value = val;
});

$('#btnPrintReceipt').addEventListener('click', async () => {
  let payload;
  try {
    payload = JSON.parse($('#receiptPayload').value);
  } catch(e) {
    showToast('JSON tidak valid: ' + e.message, 'error'); return;
  }
  if (!payload.items || !payload.items.length) {
    showToast('Field items wajib diisi', 'warning'); return;
  }
  if (payload.cut_paper === undefined) {
    payload.cut_paper = $('#receiptCut').checked;
  }
  if (currentConfig) {
    payload.width_mm = currentConfig.default_paper_width_mm;
    payload.mode = currentConfig.default_printer_mode;
  }
  const btn = $('#btnPrintReceipt'); btn.disabled = true;
  try {
    const r = await apiCall('/api/print/receipt', JSON.stringify(payload));
    const box = $('#receiptResponse'); box.style.display = 'block';
    box.textContent = JSON.stringify(r.data, null, 2);
    if (r.ok && r.data.success) showToast('Receipt terkirim', 'success');
    else showToast('Gagal: ' + (r.data.message || r.status), 'error');
  } finally { btn.disabled = false; }
});

// ============================
//  Init
// ============================
(async function init() {
  try {
    const baseEl = $('#docBaseUrl');
    if (baseEl) baseEl.textContent = window.location.origin || 'http://localhost:8080';

    // 1. Muat config lebih dulu agar loadPrinters mengetahui printer mana
    //    yang harus di-pre-select setelah deteksi selesai.
    //    applyConfigToForm() di dalam loadConfig akan mengisi paper width,
    //    mode printer, dan menyiapkan currentConfig.default_printer sebagai
    //    target seleksi dropdown.
    await loadConfig();

    // 2. Jalankan refresh penuh (POST /api/printers/refresh) dengan
    //    loading indicator. Karena currentConfig sudah tersedia, variabel
    //    'prev' di dalam loadPrinters akan menangkap nama printer dari
    //    config dan otomatis memilihnya setelah daftar selesai dimuat.
    await loadPrinters(true);

    refreshLogs();
    startLogsAutoRefresh();
  } catch(e) {
    showToast('Gagal inisialisasi: ' + e.message, 'error');
  }
})();
</script>

</body>
</html>
`
