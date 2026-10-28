# RELEASE NOTES: RaHaMail

This document tracks all official releases of **RaHaMail**, highlighting new features, performance enhancements, and system updates across each version.

---

## 📦 Version 1.0.0 (October 28, 2026) — Initial Stable Release

Official initial general availability release of **RaHaMail v1.0.0**, an ultra-lightweight, modern, and standalone mail server written entirely in pure **Go** using the **Gin** framework.

### 🌟 Release Highlights
- **100% Pure Go Single Static Binary (Zero Node.js)**: The entire web interface (Go `.tmpl` templates), CSS, JavaScript, and Bootstrap 5.3 font icons are directly embedded (`//go:embed`) into the binary. This produces a container image of just **~51 MB** with an idle RAM footprint of **under 50 MB**.
- **Embedded Base64 Font Icons**: Bootstrap Icons are inlined directly into the CSS via Base64 Data URI, guaranteeing 100% icon rendering reliability without CORS blocks, missing font files, or MIME-type issues.
- **Consistent Light Theme**: The entire interface (Webmail, Registration, Login, Dashboard, and Setup Wizard) has been unified under a clean, modern, and professional Light Mode.

---

### ✨ Features Available in Version 1.0.0

#### 1. Core Mail Protocols & Server Engine
- **SMTP Inbound (Port 25)**: Receives incoming emails directly from the internet (Gmail, Yahoo, Outlook, etc.) with built-in Anti-Open-Relay enforcement, automatic RFC 5322 MIME parsing, and physical storage in standard Maildir (`.eml`) format on disk.
- **SMTP Submission (Port 587 / 2525)**: Supports SASL `PLAIN` / `LOGIN` authentication powered by `bcrypt` password hashing for third-party mail clients (Thunderbird, Apple Mail, Outlook).
- **IMAP Server (Port 143 / 993 / 1143)**: Bidirectional mailbox synchronization conforming to the IMAP v4rev1 specification for `INBOX`, `Sent`, `Trash`, and `Spam` folders.

#### 2. Outbound MTA & DKIM Cryptography
- **Direct MTA Delivery Engine**: Automatic Mail Exchanger (DNS MX Lookup) resolver sorting target servers by preference priority, with automatic fallback to DNS A-records.
- **DKIM RSA 2048-bit Signer**: Automatic generation of 2048-bit RSA key pairs upon domain creation, instant DNS TXT record export (`v=DKIM1; k=rsa; p=...`), and automatic digital signing on all outbound email headers to preserve domain reputation.

#### 3. Optimized Multi-Database Engine
- **SQLite WAL Mode (Default - Pure Go)**: Powered by a pure Go driver (`glebarez/sqlite`) with zero CGO dependencies. Configured with Write-Ahead Logging (WAL) mode and a 5000ms busy timeout for high concurrency without database lock issues.
- **PostgreSQL Ready**: Native support for enterprise-grade PostgreSQL databases with optimized connection pooling.
- **GORM Auto-Migration**: Automatic table schema migration upon server startup.

#### 4. Webmail Portal & Split-View Reader
- **Modern Split-Pane Layout (`/mail`)**: Multi-folder navigation (`INBOX`, `Sent`, `Scheduled`, `Drafts`, `Spam`, `Trash`), real-time search filter, star flagging, and dynamic storage quota progress bar.
- **Anti-XSS Security Isolation**: HTML email bodies are rendered inside an isolated iframe (`sandbox="allow-same-origin"`), preventing malicious scripts and tracking pixels from accessing user sessions.
- **Built-in Security Badges**: Automatic verification badges for `SPF: PASS`, `DKIM: PASS`, and `DMARC: PASS`.

#### 5. Rich-Text WYSIWYG Composer
- **Interactive Composer Modal**: Dedicated fields for `To`, toggleable `Cc` and `Bcc`, and Subject.
- **Lightweight Formatting Toolbar (Pure Vanilla JS)**: Bold, Italic, Underline, Bulleted List, Numbered List, Link Insertion, and Format Eraser without third-party npm libraries.
- **Multi-File Attachment Support**: Attach multiple files with visual file size chips and individual delete triggers.
- **Reply & Forward Integration**: Automatically fills recipient addresses, formats subjects (`Re:` or `Fwd:`), and formats original message quotes.
- **RFC 5322 MIME Builder**: Generates dynamic `multipart/alternative` and `multipart/mixed` payloads and stores sent copies in the sender's `Sent` mailbox.

#### 6. Email Scheduler Engine
- **Flexible Scheduled Delivery**: Date and time selector (`datetime-local`) embedded directly in the composer modal.
- **Background Periodic Worker**: An internal ticker worker scans the queue every 15 seconds to dispatch due emails, sign them with DKIM, and deliver them automatically.
- **Scheduled Mailbox Tab**: Dedicated **⏰ Scheduled** folder in the webmail sidebar to inspect pending outgoing emails with the ability to cancel scheduled deliveries at any time.

#### 7. Self-Service Mailbox Registration & Anti-Bot Captcha
- **Registration Portal (`/register`)**: Clean registration form with dynamic active domain selection.
- **Real-Time Username Availability**: Asynchronous AJAX email availability validation (`/api/v1/auth/check-username`) as users type.
- **Pure-Go Math Captcha**: Server-side mathematical anti-bot challenge secured by HMAC-SHA256 signatures with zero external Google reCAPTCHA trackers.

#### 8. Admin Dashboard & Real-Time SSE Stream
- **Interactive 4-Step Setup Wizard (`/setup`)**: Guided onboarding for database configuration, initial domain registration, DKIM key generation, and Super Admin provisioning.
- **Server-Sent Events (SSE) Stream (`/api/v1/sse`)**: Real-time push channel providing instant browser updates for server metrics and new incoming email notifications without page reloads.

#### 9. Production-Ready Containerization
- **Minimal ~51 MB Container**: Multi-stage `Dockerfile` based on `alpine:3.20`.
- **Traefik Reverse Proxy Integration**: Ready-to-deploy `docker-compose.traefik.yml` and automated deployment script `deploy.sh` with automatic Let's Encrypt SSL and disabled buffering (`X-Accel-Buffering: no`).

---

## 📜 Licensing & Donations
- **Open-Source License**: Licensed under the **GNU General Public License v3.0 (GPLv3)** for personal and non-commercial open-source usage.
- **Commercial License**: For commercial, enterprise, or closed-source deployments, please donate via PayPal to:  
  👉 **`ridwan.haqi@gmail.com`** (Refer to [COMMERCIAL_LICENSE.md](file:///home/coderhaqi/Projects/mail-server/COMMERCIAL_LICENSE.md) for details).
