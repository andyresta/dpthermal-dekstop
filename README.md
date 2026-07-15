<div align="center">
  <img src="assets/banner.png" alt="DPThermal Banner" width="100%">
</div>

# DPThermal

DPThermal adalah service HTTP lokal ringan yang bertindak sebagai jembatan (bridge) antara aplikasi web dan printer kasir (Thermal atau Dot Matrix) yang terpasang di komputer (Windows/Linux) via Spooler (USB/LAN) atau Bluetooth.

Aplikasi web Anda cukup mengirim request HTTP POST (JSON) ke DPThermal, lalu DPThermal akan mengubahnya menjadi perintah ESC/POS atau plain text, dan meneruskannya ke printer.

## Fitur Utama

- **Cetak Receipt Terstruktur**: Mengirim daftar item (teks, garis, barcode/QR, gambar) dengan dukungan format yang kompatibel dengan MaxThermal Android.
- **Cetak Teks & Gambar (ESC/POS)**: Mendukung alignment, ukuran font, style (bold/underline), dan cetak gambar (dithering).
- **Dot Matrix / Plain Text**: Mode fallback untuk printer non-thermal (seperti EPSON LX-310).
- **Auto-Detect Printer**: Mendeteksi printer yang terpasang di OS (Spooler) dan Bluetooth (RFCOMM).
- **Web UI**: Dilengkapi dengan dashboard web lokal untuk testing, melihat log, dan dokumentasi API.

## Cara Penggunaan (Instalasi & Menjalankan)

### 1. Download Executable
Silakan download file _binary_ (executable) DPThermal yang sesuai dengan OS Anda:
- **Windows 64-bit**: `dpthermal-windows-amd64.exe`
- **Windows 32-bit**: `dpthermal-windows-386.exe`
- **Linux 64-bit**: `dpthermal-linux-amd64`
- **Linux ARM (Raspberry Pi, dll)**: `dpthermal-linux-arm` atau `dpthermal-linux-arm64`

### 2. Menjalankan Aplikasi
- **Windows**: Cukup klik dua kali (double-click) file `.exe` yang sudah di-download.
- **Linux**: Berikan izin eksekusi (*executable permission*) terlebih dahulu melalui terminal, lalu jalankan aplikasinya:
  ```bash
  chmod +x dpthermal-linux-amd64
  ./dpthermal-linux-amd64
  ```

Setelah dijalankan, service akan secara otomatis berjalan di background dan membuka dashboard pengaturan DPThermal di browser default Anda pada URL: `http://localhost:8080`.

### 3. Konfigurasi Printer
   Buka UI (`http://localhost:8080`), masuk ke tab **Pengaturan**.
   Pilih printer aktif Anda, pilih **Mode Printer** (Thermal atau Dot Matrix), dan **Lebar Kertas** (58mm atau 80mm).
   Klik **Simpan Pengaturan**.

## Integrasi API (Khusus Receipt)

DPThermal mendukung endpoint khusus `/api/print/receipt` (alias: `/print/receipt`) untuk mencetak receipt terstruktur. Endpoint ini sangat berguna untuk mencetak nota penjualan dengan layout yang rapi.

**Endpoint:** `POST http://localhost:8080/api/print/receipt`
**Content-Type:** `application/json`

### Format Payload

Payload menggunakan array `items` yang berisi urutan perintah cetak. Berikut contoh payload JSON:

```json
{
  "items": [
    { "type": "text", "align": "center", "size": 2, "style": "bold", "data": "NAMA TOKO" },
    { "type": "text", "align": "center", "data": "Jl. Contoh No. 123" },
    { "type": "line", "style": "double" },
    { "type": "text", "data": "Nasi Goreng      x2   30.000" },
    { "type": "text", "data": "Es Teh           x2   10.000" },
    { "type": "line" },
    { "type": "text", "align": "right", "size": 2, "style": "bold", "data": "TOTAL: Rp 40.000" },
    { "type": "feed", "data": "1" },
    { "type": "qr", "align": "center", "size": 200, "data": "https://toko.example.com/inv/12345" },
    { "type": "text", "align": "center", "style": "bold", "data": "Terima kasih!" },
    { "type": "feed", "data": "3" }
  ],
  "cut_paper": true,
  "width_mm": 80,
  "copies": 1
}
```

### Penjelasan Tipe Item (`type`)

- **`text`**: Mencetak baris teks.
  - `align`: `left` | `center` | `right` (default: `left`)
  - `size`: `1` - `8` (ukuran karakter font, default: `1`)
  - `style`: `bold` | `underline` | `bold,underline`
  - `data`: Teks yang akan dicetak.
- **`line`**: Mencetak garis pemisah horizontal.
  - `style`: `single` (`-`) | `double` (`=`) | `dotted` (`.`)
- **`qr`**: Generate & cetak QR Code.
  - `align`: `left` | `center` | `right` (default: `center`)
  - `size`: Resolusi QR dalam px (default: `200`)
  - `data`: Konten/URL QR Code.
- **`image`**: Cetak gambar (hanya mode Thermal).
  - `align`: `left` | `center` | `right` (default: `center`)
  - `data`: Base64 string dari gambar PNG/JPEG (misal: `data:image/png;base64,...`).
- **`feed`**: Menambahkan baris kosong (feed kertas).
  - `data`: Jumlah baris (misal: `"3"`).

*Catatan: Pada mode **Dot Matrix**, item tipe `qr` dan `image` tidak akan dicetak karena tidak didukung oleh printer.*

### Contoh Kode Integrasi (JavaScript/Frontend)

Anda dapat menggunakan fungsi `fetch` di JavaScript aplikasi web Anda untuk mengirim perintah cetak langsung ke DPThermal lokal:

```javascript
async function printReceipt(items) {
  try {
    const res = await fetch('http://localhost:8080/api/print/receipt', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        items: items,
        cut_paper: true
      })
    });
    
    const data = await res.json();
    if (data.success) {
      console.log("Cetak berhasil! Job ID:", data.job_id);
    } else {
      console.error("Gagal mencetak:", data.message);
    }
  } catch (err) {
    console.error("Service DPThermal tidak terdeteksi atau mati.", err);
  }
}
```

Untuk detail dokumentasi API (Cetak Teks / Cetak Gambar terpisah), Anda dapat membuka dashboard DPThermal di browser dan menuju tab **Dokumentasi API**.
