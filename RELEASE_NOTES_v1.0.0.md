# RELEASE NOTES: RaHaMail v1.0.0
**Tanggal Rilis**: 28 Oktober 2026  
**Status Rilis**: Stable GA (General Availability)  
**Arsitektur**: 100% Pure Go Single Static Binary (Zero Node.js, Embedded Assets)

---

## 🚀 Ringkasan Peluncuran (Launch Highlights)

Kami dengan bangga mengumumkan rilis resmi **RaHaMail v1.0.0**, sebuah solusi mail server mandiri, modern, dan ultra-ringan yang ditulis sepenuhnya menggunakan bahasa pemrograman **Go**.

RaHaMail dirancang khusus untuk mengatasi kompleksitas dan konsumsi sumber daya tinggi dari mail server konvensional (seperti Postfix/Dovecot/SpamAssassin stack yang memakan banyak memori). Dengan arsitektur **Single Static Binary**, seluruh frontend webmail, template HTML, file CSS, JS, dan font ikon disematkan langsung (`//go:embed`) ke dalam binary eksekutabel, menghasilkan image container hanya berukuran **~51 MB** dengan konsumsi RAM di bawah **50 MB** saat idle.

---

## ✨ Fitur-Fitur Utama di Versi 1.0.0

### 1. Protokol Email Lengkap (Core Mail Engine)
- **SMTP Inbound (Port 25)**:
  - Menerima email masuk langsung dari server internet (Gmail, Yahoo, Outlook, dsb).
  - Dilengkapi sistem proteksi *Anti Open-Relay* (menolak penerima non-lokal yang tidak terdaftar).
  - Parsing otomatis MIME RFC 5322 untuk subjek, pengirim, penerima, dan tanggal.
  - Penyimpanan file mentah email berstandar Maildir (`.eml`) di direktori fisik disk.
- **SMTP Submission (Port 587 / 2525)**:
  - Dukungan otentikasi SASL `PLAIN` / `LOGIN` untuk aplikasi mail client pihak ketiga (Thunderbird, Apple Mail, Outlook).
  - Verifikasi keamanan kata sandi menggunakan enkripsi `bcrypt`.
- **IMAP Server (Port 143 / 993 / 1143)**:
  - Sinkronisasi kotak surat dua arah berstandar protokol IMAP v4rev1.
  - Akses folder pesan standar: `INBOX`, `Sent`, `Trash`, `Spam`.

### 2. Outbound MTA & Kriptografi DKIM
- **Mesin Pengiriman Outbound (Direct MTA Delivery)**:
  - Mesin pencari otomatis record Mail Exchanger (DNS MX Lookup) dengan pengurutan prioritas preferensi tertinggi.
  - Fallback ke DNS A-record jika remote domain tidak memiliki MX record.
- **DKIM RSA 2048-bit Signer**:
  - Pembuatan pasangan kunci privat & publik RSA 2048-bit otomatis untuk setiap domain baru.
  - Ekspor DNS TXT Record (`v=DKIM1; k=rsa; p=...`) siap pasang.
  - Penandatanganan digital otomatis pada setiap header email keluar untuk mencegah email masuk ke folder Spam penerima.

### 3. Arsitektur Multi-Database Teroptimasi
- **SQLite WAL Mode (Default - Zero CGO)**:
  - Menggunakan driver murni Go (`glebarez/sqlite`) tanpa membutuhkan compiler C / gcc.
  - Mode Write-Ahead Logging (WAL) dan `busy_timeout=5000` untuk konkurensi tinggi tanpa resiko *database locked*.
- **PostgreSQL Ready**:
  - Dukungan langsung untuk basis data PostgreSQL bagi kebutuhan beban kerja skala enterprise.
- **GORM Auto-Migration**:
  - Migrasi skema tabel otomatis saat pertama kali aplikasi dijalankan.

### 4. Zero-Node.js Frontend & Embedded Assets
- **HTML Go Templates (`.tmpl`)**:
  - Frontend murni menggunakan engine template bawaan Go tanpa ketergantungan framework Node.js (seperti React, Vue, atau Next.js).
- **Embedded Bootstrap 5.3 & Icons**:
  - Aset CSS, JavaScript, dan Font Icons tertanam langsung di dalam binary.
  - **Base64 Inline Font Icons**: Font `bootstrap-icons.woff2` disematkan langsung via data URI pada file CSS untuk menjamin ikon muncul 100% tanpa kendala CORS atau MIME-type di browser.
- **Konsistensi Tampilan Light Theme**:
  - Desain antarmuka profesional bertema terang (*Light Mode*) yang bersih, nyaman, dan konsisten di seluruh halaman.

### 5. Webmail Portal & Split-View Reader
- **Tampilan Split-Pane Modern**:
  - Sidebar navigasi folder: `Kotak Masuk (INBOX)`, `Terkirim (Sent)`, `Terjadwal (Scheduled)`, `Draf (Drafts)`, `Spam`, dan `Sampah (Trash)`.
  - Panel daftar pesan dengan indikator belum dibaca (*unread*), pencarian instan (*live search*), dan tombol bintang (*starred*).
  - Panel pembaca detail pesan dengan header lengkap dan badge verifikasi status keamanan (`SPF: PASS`, `DKIM: PASS`, `DMARC: PASS`).
- **Keamanan Anti-XSS (Sandboxed Iframe)**:
  - Isi email HTML dirender dalam frame terisolasi (`sandbox="allow-same-origin"`), menangkal eksekusi skrip jahat dari email phishing.

### 6. Rich-Text WYSIWYG Composer
- **Modal Tulis Email Interaktif**:
  - Kolom tujuan `To`, toggle cepat `Cc` dan `Bcc`, serta subjek email.
  - Formatting bar ringan (Pure Vanilla JS): Format **Tebal**, *Miring*, <u>Garis Bawah</u>, Bullet List, Numbered List, Sisipkan Tautan, dan Bersihkan Format.
  - Pengunggah multi-file lampiran (*attachment chips*) dengan deteksi ukuran file dan tombol hapus lampiran.
  - Tombol **Balas (Reply)** dan **Teruskan (Forward)** yang otomatis mengisi subjek (`Re:` / `Fwd:`) dan mengutip isi pesan lama.
- **MIME Builder RFC 5322**:
  - Menyusun struktur multipart MIME secara dinamis dan menyimpan salinannya otomatis ke folder `Sent`.

### 7. Email Scheduler Engine (Pengiriman Terjadwal)
- **Penjadwalan Pengiriman Fleksibel**:
  - Tombol `Jadwalkan` pada modal composer dengan pemilih waktu `datetime-local` masa depan.
- **Background Periodic Worker**:
  - Worker ticker internal mengecek antrean setiap 15 detik untuk memproses email yang telah jatuh tempo secara otomatis.
- **Manajemen Tab Folder "Scheduled"**:
  - Tab folder khusus di sidebar webmail untuk melihat antrean email dan membatalkan jadwal kapan saja sebelum terkirim.

### 8. Pendaftaran Akun Mandiri (FE Register) & Anti-Bot Captcha
- **Portal Registrasi Akun (`/register`)**:
  - Form pendaftaran mailbox mandiri dengan dropdown domain aktif.
  - **Real-Time Username Availability**: Validasi ketersediaan email via AJAX saat pengguna mengetik.
  - **Pure-Go Math Captcha**: Tantangan keamanan matematika server-side dengan tanda tangan kriptografis HMAC tanpa pelacak Google reCAPTCHA pihak ketiga.

### 9. Admin Dashboard & Real-Time SSE Stream
- **Interactive Setup Wizard (`/setup`)**:
  - Setup 4 langkah untuk konfigurasi awal database, domain perdana, DKIM, dan pembuatan akun Super Admin.
- **Server-Sent Events (SSE)**:
  - Kanal live stream data (`/api/v1/sse`) untuk memperbarui status metrik dan notifikasi email masuk baru tanpa reload browser.

### 10. Siap Produksi (Docker & Traefik Ready)
- **Ukuran Image Ultra Kecil**: Multi-stage build menghasilkan container runtime hanya **~51.3 MB**.
- **Traefik Reverse Proxy**: Template `docker-compose.traefik.yml` dan script deployment `deploy.sh` dengan konfigurasi otomatis Let's Encrypt dan penonaktifan response buffering (`X-Accel-Buffering: no`).

---

## 🛠️ Panduan Menjalankan Versi 1.0.0

### A. Menjalankan Langsung via Go Binary
```bash
# Clone repositori
git clone https://github.com/yourusername/rahamail.git
cd rahamail

# Build binary mandiri
go build -o rahamail main.go

# Jalankan server
./rahamail
```
Akses Web UI di: **http://localhost:8080**  
(Atau jalankan wizard konfigurasi perdana di **http://localhost:8080/setup**)

### B. Menjalankan via Container (Podman / Docker)
```bash
# Build image
podman build -t rahamail:1.0.0 .

# Jalankan container
podman run -d --name rahamail \
  -p 8080:8080 -p 25:25 -p 587:587 -p 143:143 \
  -v ./data:/app/data \
  rahamail:1.0.0
```

---

## 📜 Lisensi & Donasi
- **Lisensi**: Proyek ini dilisensikan di bawah **GNU General Public License v3.0 (GPLv3)** untuk penggunaan open-source dan non-komersial.
- **Lisensi Komersial**: Untuk penggunaan komersial / corporate deployment, silakan berdonasi melalui PayPal ke:
  👉 **`ridwan.haqi@gmail.com`** (Lihat [COMMERCIAL_LICENSE.md](file:///home/coderhaqi/Projects/mail-server/COMMERCIAL_LICENSE.md) untuk detailnya).

---

*Dibuat dengan ❤️ oleh Ridwan Haqi & RaHaMail Contributors.*
