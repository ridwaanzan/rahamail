# CHANGELOG: RaHaMail

Semua perubahan penting pada proyek **RaHaMail** akan dicatat dalam berkas ini. Format penulisan mengacu pada [Keep a Changelog](https://keepachangelog.com/id/1.0.0/) dan proyek ini mematuhi standar [Semantic Versioning](https://semver.org/).

---

## [1.0.0] - 2026-10-28

### Ditambahkan (Added)
- **Core Mail Protocols**:
  - Implementasi SMTP Inbound (Port 25) dengan proteksi anti open-relay.
  - Implementasi SMTP Submission (Port 587/2525) dengan otentikasi SASL PLAIN/LOGIN via `bcrypt`.
  - Implementasi server IMAP v4rev1 (Port 143/993) untuk mail client pihak ketiga.
- **Outbound MTA & Kriptografi**:
  - Mesin pengiriman langsung (outbound delivery) berbasis DNS MX Resolver otomatis.
  - Pembuatan pasangan kunci RSA 2048-bit dan penandatanganan digital DKIM otomatis pada header email keluar.
- **Frontend Murni Zero-Node.js**:
  - Migrasi seluruh view antarmuka ke engine template bawaan Go (`.tmpl`) dengan sistem `//go:embed`.
  - Integrasi aset CSS & JavaScript Bootstrap 5.3 langsung di dalam memori binary tanpa membutuhkan runtime Node.js.
  - Embedding font Bootstrap Icons secara langsung ke dalam CSS via Base64 Data URI untuk memastikan ikon tampil 100% tanpa kendala CORS / MIME-type.
  - Standarisasi antarmuka pengguna ke tema terang (*Light Mode*) yang bersih dan modern.
- **Portal Webmail & Reader**:
  - Tampilan split-view modern (daftar pesan di kiri, detail isi email di kanan).
  - Navigasi folder lengkap: `Kotak Masuk`, `Terkirim`, `Terjadwal`, `Draf`, `Spam`, dan `Sampah`.
  - Pembaca email aman anti-XSS dengan sandboxed iframe.
  - Indikator keamanan bawaan: badge verifikasi `SPF: PASS`, `DKIM: PASS`, dan `DMARC: PASS`.
- **WYSIWYG Rich-Text Composer**:
  - Modal tulis pesan baru dengan toolbar formatting teks ringan (Pure Vanilla JS).
  - Kolom penerima `To`, serta toggle instan `Cc` dan `Bcc`.
  - Sistem lampiran multi-file (*attachments*) dengan penampil chip ukuran file.
  - Fitur Balas (*Reply*) dan Teruskan (*Forward*) dengan pengisian subjek dan kutipan pesan otomatis.
- **Email Scheduler Engine**:
  - Fitur penjadwalan pengiriman email masa depan langsung dari composer modal.
  - Background ticker periodic worker (interval 15 detik) untuk mengeksekusi antrean email terjadwal secara otomatis.
  - Tab folder khusus `⏰ Terjadwal` dengan opsi pembatalan jadwal kapan saja.
- **Autentikasi & Registrasi Mandiri**:
  - Halaman pendaftaran mailbox mandiri (`/register`) dengan dropdown domain aktif.
  - Pengecekan ketersediaan username realtime via AJAX (`/api/v1/auth/check-username`).
  - Tantangan keamanan anti-bot matematika (*Pure-Go Math Captcha*) tanpa Google reCAPTCHA.
  - Sistem sesi berbasis cookie HMAC-SHA256 murni di Go.
- **Admin & Monitoring**:
  - Setup Wizard interaktif 4-tahap (`/setup`).
  - Dashboard statistik real-time bertenaga Server-Sent Events (SSE).
- **Deployment & Kontainerisasi**:
  - Multi-stage `Dockerfile` dengan hasil image akhir berukuran ~51.3 MB.
  - Konfigurasi produksi Traefik reverse proxy otomatis (`docker-compose.traefik.yml` dan `deploy.sh`).
- **Skema Multi-Database**:
  - Dukungan basis data SQLite WAL Mode murni Go (`glebarez/sqlite`) tanpa CGO.
  - Dukungan basis data PostgreSQL untuk beban kerja tinggi.

### Diubah (Changed)
- Mengganti seluruh referensi nama brand lama menjadi **RaHaMail**.
- Menghilangkan switch tema ganda (dark/light toggle) dan menyelaraskan ke tema Light yang konsisten.
- Mengoptimalkan pembacaan query daftar pesan di database dengan penambahan `.Limit(100)` untuk mencegah memory leak.
- Menetapkan `GIN_MODE=release` secara default saat startup.

### Dihapus (Removed)
- Menghapus seluruh ketergantungan pada runtime Node.js, npm, webpack, dan node_modules dari codebase.
- Menghapus seluruh referensi URL dan nama pihak ketiga yang tidak relevan agar repositori murni bersifat open-source.

---

### Lisensi & Dukungan
- Lisensi: **GPLv3** (Penggunaan Open Source / Non-Komersial).
- Lisensi Komersial: Donasi PayPal ke **`ridwan.haqi@gmail.com`**.
