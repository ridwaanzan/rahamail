# RELEASE NOTES: RaHaMail

This document tracks all official releases of **RaHaMail**, highlighting new features, performance enhancements, and system updates across each version.

---

## 📦 Version 1.4.1 (October 29, 2026) — Database Migration Stability Patch

Patch release resolving SQLite table recreation foreign key constraint failures during server startup.

### 🌟 Release Highlights
- **SQLite WAL Mode Auto-Migration Stability**:
  - Configured `_pragma=foreign_keys(OFF)` during initial connection setup for non-blocking GORM table recreation and temporary table drops.
  - Automatically re-enables `PRAGMA foreign_keys = ON;` immediately after schema synchronization.
  - Fixes `constraint failed: FOREIGN KEY constraint failed (787)` on `DROP TABLE domains` when existing relations are present.

---

## 📦 Version 1.4.0 (October 29, 2026) — Security Awareness Simulation & Enterprise Threat Defenses

Release 1.4.0 introduces an internal Phishing & Spam Awareness Simulation engine for employee training, alongside ClamAV/Rspamd scanner milters, Inbound DMARC verification, and S3-compatible cloud disaster recovery.

### 🌟 Release Highlights
- **Employee Phishing & Spam Awareness Simulation Platform**:
  - Full-featured human risk management training platform integrated directly into the mail server.
  - Skenario templates out-of-the-box: IT Password Expiration alerts, HR Bonus notifications, and Courier Delivery notices.
  - Safe local distribution via internal queue without outbound internet footprint.
  - Tracking URL dispatcher (`GET /sim/c/:code`) capturing click interactions, timestamps, IP addresses, and user-agents (**Status: NOT PASSED**).
  - Teachable Moment educational landing page providing instant constructive feedback on overlooked red flags.
  - Webmail Spam Report detection marking employees who report simulated emails as **PASSED**.
  - Admin Analytics Dashboard with interactive SVG Donut charts, failure rate percentages, and individual employee compliance breakdown.
- **Antivirus & Anti-Spam Filtering (ClamAV & Rspamd)**:
  - Real-time scanning of inbound email streams and attachments over ClamAV `INSTREAM` protocol (`127.0.0.1:3310` or Unix socket). Rejects infected messages with `554 5.7.1 Message rejected: Infected with [VirusName]`.
  - HTTP-based Rspamd milter scanning (`/checkv2`) with automatic classification, quarantine routing to the `Spam` folder, and subject tagging (`[SPAM]`).
  - Graceful fail-open bypass: Incoming email flow remains completely uninterrupted if scanner daemons are temporarily offline.
- **Inbound DMARC, SPF & DKIM Protocol Verification (RFC 7489 & RFC 8601)**:
  - Automated DNS TXT record lookups verifying SPF sender identity and DKIM cryptographic signatures on Port 25.
  - Strict enforcement of sender DMARC policies (`p=reject` drops unauthenticated mail with `550 5.7.1`; `p=quarantine` routes to the user's `Spam` folder).
  - Prepends standard RFC 8601 `Authentication-Results: rahamail; spf=...; dkim=...; dmarc=...` headers to stored messages.
- **Automated S3-Compatible Cloud Backups**:
  - Pure-Go backup engine creating atomic SQLite snapshots via `VACUUM INTO` and tar.gz compressed Maildir archives.
  - End-to-end client-side encryption using **AES-256 GCM** before cloud dispatch.
  - S3 object storage upload pipeline supporting AWS S3, Cloudflare R2, Wasabi, and MinIO.
  - Admin management API endpoints (`/api/v1/admin/backup/config` and `/api/v1/admin/backup/trigger`).

---

## 📦 Version 1.3.0 (October 29, 2026) — Trust & Safety Warning Banner & Webmail Productivity

Release 1.3.0 brings intelligent inbound trust and reputation evaluation, warning banners for unknown external senders, Address Book management, Contact Auto-Complete, Vacation Notice Auto-Responder, and Canned Email Templates.

### 🌟 Release Highlights
- **Inbound Trust & Unknown Domain Warning Banner**:
  - Pure-Go sender reputation evaluation engine (`internal/trust`) with 5-stage trust analysis:
    1. **Authentication Verification**: Identifies spoofing via SPF, DKIM, and DMARC failures (`danger` alert level).
    2. **Local Domain Whitelist**: Checks active domains configured on the server.
    3. **Address Book Matching**: Verifies whether sender is already listed in user contacts.
    4. **Interaction History Analysis**: Detects prior message exchanges or past outgoing replies.
    5. **Threat & Risk Payload Scan**: Evaluates presence of external URLs (`http://`, `https://`) and attachments for first-time external senders (`warning` alert level).
  - Webmail Interactive Banner: Prominently displayed above the email content with dynamic color coding (Danger/Warning/Info).
  - Quick Remediation Actions: One-click **"Mark as Trusted / Add to Contacts"** to whitelist the sender immediately, and **"Move to Spam"** to isolate malicious emails.
- **Address Book & Contact Auto-Complete**:
  - Full-featured contact management portal (`/mail` sidebar modal) supporting contact name, email address, company name, and notes.
  - Interactive auto-suggest dropdown in the composer for `To`, `Cc`, and `Bcc` inputs with 150ms debounce and keyboard navigation.
  - Background auto-harvesting engine: Automatically saves recipient email addresses from sent messages to the user's personal address book if not already present.
- **Vacation Notice / Out-of-Office Auto-Responder**:
  - Configurable automated replies with start/end scheduling, custom subject, and message body.
  - Anti-looping defense: Automatic suppression for bounces, system daemons (`mailer-daemon`, `<>`), and `noreply` addresses.
  - Frequency throttling: Enforces a strict 1 auto-reply per sender per 24 hours limit backed by `auto_responder_logs`.
- **Canned Responses & Quick Templates**:
  - Pre-written email snippet repository enabling one-click insertion directly into the WYSIWYG composer editor.
  - Full CRUD management interface directly accessible from the composer toolbar dropdown.

---

## 📦 Version 1.2.0 (October 29, 2026) — Security, Identity & Developer APIs

Phase 1 release of the RaHaMail Enterprise Roadmap, adding Two-Factor Authentication (2FA), a Developer Transactional REST API, and domain-level Catch-All routing.

### 🌟 Release Highlights
- **Two-Factor Authentication (2FA / TOTP — RFC 6238)**:
  - Time-based One-Time Password verification for webmail and admin accounts with zero external dependencies.
  - Interactive setup modal with standard `otpauth://` key generation and Base32 secret exchange compatible with Google Authenticator, Microsoft Authenticator, and Bitwarden.
  - 8 emergency one-time recovery backup codes hashed with `bcrypt`.
  - Two-step login flow with dedicated modal prompting for OTP or emergency recovery code.
- **Developer Transactional REST API**:
  - Outbound email dispatch endpoint `POST /api/v1/send` authenticated via Bearer token (`rhm_live_...`).
  - Send transactional emails directly via JSON payloads over HTTPS without needing local SMTP client configurations.
  - Full support for `to`, `cc`, `bcc`, `html`, and `text` with automatic RFC 5322 MIME compilation, DKIM digital signing, and dual local/MTA outbound delivery.
  - Dedicated API Keys management panel in Webmail settings to generate, label, track usage, and revoke tokens.
- **Catch-All Mailboxes & Domain Aliases**:
  - Configurable `catch_all_user_id` per domain to route all unassigned inbound emails (`*@domain.com`) to a designated mailbox without generating SMTP 550 Mailbox Not Found bounces.
  - Management API endpoint `POST /api/v1/domains/:id/catch-all`.

---

## 📦 Version 1.1.0 (October 28, 2026) — Security Hardening & Usability Release

A comprehensive security, reliability, and usability upgrade for **RaHaMail**, delivering all enhancement opportunities identified in the v1.0.0 roadmap.

### 🌟 Release Highlights
- **SMTP Smuggling Defense (RFC 5321)**: Mitigated global high-profile vulnerabilities **CVE-2023-51764 & CVE-2023-51765** by enforcing canonical CRLF sequence validation and outright rejecting naked LF characters in SMTP DATA streams.
- **Backend HTML Sanitization Engine**: Direct server-side parsing to eliminate Stored XSS vectors (stripping `<script>`, `<iframe>`, `<object>`, `<embed>`, inline `on*` event handlers, and `javascript:` URIs) before delivering emails to the webmail client.
- **Multi-Tier Token Bucket Rate Limiting**: In-memory rate limiting powered by `golang.org/x/time/rate` defending against brute-force logins (5 req/min per IP), registration floods (3 req/hr per IP), and outbound spam abuse (30 emails/min per user).
- **SHA-256 Hashed Storage Architecture**: Hardened physical Maildir storage using randomized SHA-256 directories (`var/mail/{folder}/{userID}/{hash}.eml`) with strict path traversal isolation.
- **Mailbox Pagination**: Dynamic cursor and limit pagination (`?page=1&limit=25`) with interactive page navigation controls in the webmail interface.
- **Composer Draft Auto-Save**: Real-time browser `localStorage` draft saving every second with automatic recovery upon reopening the composer and automated cleanup upon sending.
- **In-Browser Attachment Live Previews**: Native modal preview for images (`.png`, `.jpg`, `.webp`, `.gif`), PDF documents, and text files (`.txt`, `.log`, `.csv`, `.json`) without requiring file downloads.

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
