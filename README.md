# RaHaMail (RaHa Mail Server)

> Modern, ultra-lightweight, high-performance, single-binary Mail Server written in pure Go with Webmail Portal, Admin Dashboard, SMTP Inbound & Submission, IMAP4rev1 Server, Email Scheduler, Server-Sent Events (SSE), and embedded SQLite WAL storage.

[![Release](https://img.shields.io/badge/release-v1.4.1-blue.svg)](RELEASE_NOTES.md)
[![License: GPL v3](https://img.shields.io/badge/License-GPLv3-blue.svg)](LICENSE)
[![Go Report Card](https://img.shields.io/badge/Go-1.26+-00ADD8.svg)](https://go.dev/)
[![Docker Image Size](https://img.shields.io/badge/docker%20image-~51MB-2496ED.svg)](Dockerfile)
[![PayPal Donate](https://img.shields.io/badge/Donate-PayPal-00457C.svg)](https://www.paypal.com/paypalme)

---

## 🌟 Key Features

### 🚀 Architecture & Core Engine
- **100% Pure Go Single Static Binary (Zero Node.js, Zero CGO)**: Webmail views (Go `.tmpl` templates), stylesheets, JavaScript, and font icons are embedded directly inside the binary via `//go:embed`. No Node.js runtime, npm, or external daemons required.
- **Embedded Base64 Font Icons**: Bootstrap Icons are inlined directly into the CSS stylesheet via Base64 Data URI, guaranteeing 100% icon rendering reliability without CORS blocks or missing font assets.
- **Unified Light Mode UI**: A clean, modern, and accessible Light Mode theme across all web interfaces (Webmail, Registration, Login, Dashboard, and Setup Wizard).
- **Multi-Database Support**: Pure-Go SQLite with Write-Ahead Logging (WAL mode) and enterprise PostgreSQL compatibility.
- **Ultra-Small Footprint**: Final container image is only **~51 MB** with an idle RAM footprint of **under 50 MB**.

### 📬 Complete Mail Protocols & Outbound MTA
- **SMTP Inbound (Port 25)**: Direct server-to-server mail ingestion with strict Anti-Open-Relay validation and automatic RFC 5322 MIME parsing.
- **SMTP Submission (Port 587 / 2525)**: SASL `PLAIN` / `LOGIN` authenticated transmission powered by `bcrypt` for desktop and mobile clients (Thunderbird, Apple Mail, Outlook).
- **IMAP Server (Port 143 / 993 / 1143)**: Bidirectional mailbox synchronization conforming to IMAP v4rev1 for `INBOX`, `Sent`, `Trash`, and `Spam` folders.
- **Direct Outbound MTA Delivery**: Automated DNS MX record resolver sorting target MTAs by preference priority with automatic fallback to DNS A-records.
- **DKIM RSA 2048-bit Signer**: Automatic generation of 2048-bit RSA key pairs upon domain creation, instant DNS TXT record export (`v=DKIM1; k=rsa; p=...`), and automated digital signing on outbound email headers.
- **Catch-All Mailboxes & Domain Aliases**: Route unmatched incoming emails (`*@domain.com`) directly to a designated administrative or support mailbox without triggering 550 bounces.

### 🛡️ Security & Authentication Hardening
- **Employee Phishing & Spam Awareness Simulation (v1.4.0)**: Built-in human risk management training platform. Enables security administrators to dispatch controlled phishing simulations, inject encrypted tracking tokens (`/sim/c/:code`), record non-compliance (*NOT PASSED* vs *PASSED*), present instant educational landing pages (*Teachable Moments*), and view interactive failure rate analytics dashboards.
- **Inbound Trust & Unknown Domain Warning Banner**: Automatic sender reputation analysis engine (`internal/trust`). Detects first-time external senders, scans payloads for suspicious URLs and attachments, and renders interactive warning banners in Webmail with one-click **"Mark as Trusted / Add to Contacts"** and **"Move to Spam"** actions.
- **Antivirus & Anti-Spam Filtering (ClamAV & Rspamd)**: Real-time scan of incoming emails via ClamAV INSTREAM socket (blocking malware with `554 5.7.1`) and Rspamd HTTP checks routing spam to `Spam` folders.
- **Inbound DMARC, SPF & DKIM Verification (RFC 7489 & RFC 8601)**: Strict policy enforcement (`p=reject` / `p=quarantine`) with automatic `Authentication-Results` header generation.
- **Automated S3 Cloud Disaster Recovery**: Non-blocking atomic SQLite backups (`VACUUM INTO`) and Maildir archives encrypted client-side with **AES-256 GCM** and uploaded to AWS S3, Cloudflare R2, Wasabi, or MinIO.
- **Two-Factor Authentication (2FA / TOTP RFC 6238)**: Built-in TOTP support with standard `otpauth://` URI exchange, 6-digit dynamic window validation, and 8 one-time emergency backup recovery codes.
- **Developer Transactional REST API**: Dedicated `POST /api/v1/send` endpoint with Bearer token authentication (`rhm_live_...`) to send transactional emails via JSON over HTTPS (an open-source alternative to SendGrid/Mailgun).
- **SMTP Smuggling Defense (RFC 5321)**: Mitigates global vulnerabilities **CVE-2023-51764 & CVE-2023-51765** by enforcing canonical CRLF sequence validation and outright rejecting streams with naked LF (`\n` without `\r`).
- **Backend HTML Sanitizer (Anti-Stored XSS)**: Strips malicious tags (`<script>`, `<iframe>`, `<object>`, `<embed>`, `<form>`, inline event handlers `on*`, and `javascript:` URIs) before delivering emails to the browser.
- **Token Bucket Rate Limiting**: In-memory rate limiting powered by `golang.org/x/time/rate` defending against brute-force logins (5 req/min per IP), registration floods (3 req/hr per IP), and outbound spam abuse (30 emails/min per user).
- **SHA-256 Hashed Storage Paths**: Eliminates directory traversal and Zip-Slip risks by persisting raw `.eml` Maildir files under isolated cryptographic paths: `var/mail/{folder}/{userID}/{sha256}.eml`.

### 💻 Webmail & Productivity
- **Address Book & Contact Auto-Complete**: Integrated contact management portal with auto-suggest dropdowns in composer `To`/`Cc`/`Bcc` fields and background auto-harvesting from sent messages.
- **Vacation Notice / Out-of-Office Responder**: Scheduled automated reply engine with anti-looping protection and a 24-hour rate limit per sender.
- **Canned Responses & Quick Templates**: Reusable email snippets that can be inserted into the composer with one click.
- **Split-View Mailbox Portal (`/mail`)**: Fastmail/Gmail-style split pane layout with real-time live search, unread badges, star flagging, and storage quota progress bars.
- **Anti-XSS Sandboxed Reader**: Isolated iframe (`sandbox="allow-same-origin"`) for rendering HTML email bodies safely.
- **Mailbox Pagination**: Dynamic query pagination (`?folder=...&page=1&limit=25`) with interactive Next/Previous navigation controls.
- **WYSIWYG Rich-Text Composer**: Dedicated fields for `To`, toggleable `Cc` and `Bcc`, and a lightweight formatting bar (Bold, Italic, Underline, Lists, Link, Format Eraser).
- **Composer Draft Auto-Save**: Real-time browser `localStorage` draft saving every second with automatic recovery and cleanup.
- **In-Browser Attachment Live Previews**: Native modal preview for images (`.png`, `.jpg`, `.webp`, `.gif`), PDF documents, and text files (`.txt`, `.log`, `.csv`, `.json`) without requiring file downloads.
- **Email Scheduler Engine**: Integrated date/time picker (`datetime-local`) in the composer with an internal 15-second background periodic worker and dedicated **⏰ Scheduled** tab with one-click cancellation.
- **Self-Service Registration (`/register`)**: Clean registration portal with dynamic domain selection, real-time AJAX username availability checks, and a server-side Pure-Go Math Captcha (no Google tracking).
- **Admin Setup Wizard (`/setup`)**: Guided 4-step wizard for database selection, initial domain registration, DKIM key generation, and Super Admin provisioning.
- **Live SSE Stream (`/api/v1/sse`)**: Real-time push channel providing instant browser updates for server metrics and new incoming email notifications without page reloads.

---

## 🚀 Quick Start

### 1. Run Locally from Source
```bash
# Clone repository
git clone https://github.com/ridwaanzan/rahamail.git
cd rahamail

# Build single static executable
go build -o rahamail main.go

# Start RaHaMail server
./rahamail
```
Open **`http://localhost:8080/setup`** in your browser to complete the initial setup wizard.

### 2. Deploy with Podman / Docker & Traefik
```bash
# Automated production deployment with Traefik reverse proxy
chmod +x deploy.sh
./deploy.sh
```

---

## 🌐 Network Ports Reference

| Port | Protocol | Purpose / Description |
|---|---|---|
| **8080** | HTTP | Webmail Portal, Admin Dashboard, REST API & Live SSE Stream |
| **25** | SMTP | Core Inbound Mail Transfer (Server-to-Server MX) |
| **587** | SMTP | Authenticated Mail Submission (Client-to-Server) |
| **143** | IMAP | Mailbox Synchronization for IMAP Clients |
| **993** | IMAPS | Direct SSL/TLS Encrypted IMAP |

---

## 📄 License & Commercial Terms

RaHaMail is licensed under a **Dual-Licensing / Commercial Donation Model**:

### 1. Non-Commercial & Open Source Use
Licensed under the **GNU General Public License version 3 (GNU GPLv3)**. Free for personal, non-commercial, hobby, academic, and non-profit usage.

### 2. Commercial Use & Donation Requirement
If you intend to use RaHaMail for **commercial purposes** (such as company email infrastructure, commercial web hosting, SaaS/managed email services, or integrating into proprietary commercial appliances), you are required to support the project by becoming an active sponsor or making an official commercial donation.

For details on commercial licensing exemptions and how to donate:
- Read full terms: [`COMMERCIAL_LICENSE.md`](COMMERCIAL_LICENSE.md)
- **PayPal Donation**: `ridwan.haqi@gmail.com`
- **Direct Inquiries**: `ridwan.haqi@gmail.com`

---

## 📚 Documentation & Changelog
- **Detailed Release Notes**: [`RELEASE_NOTES.md`](RELEASE_NOTES.md)
- **Changelog History**: [`CHANGELOG.md`](CHANGELOG.md)

---

*Crafted with ❤️ by Ridwan Haqi & RaHaMail Contributors.*
