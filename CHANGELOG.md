# CHANGELOG: RaHaMail

All notable changes to the **RaHaMail** project will be documented in this file. The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/), and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

---

## [1.4.0] - 2026-10-29

### Added
- **Employee Phishing & Spam Awareness Simulation Platform**:
  - Pure-Go human risk training and security awareness simulation engine in `internal/simulation`.
  - Built-in phishing scenario presets (IT Password Reset Expiry, HR Payroll/Bonus, and Courier Package Delivery).
  - Encrypted individual tracking link generator (`GET /sim/c/:code`) tracking employee interactions with IP and User-Agent capture.
  - Interactive **Teachable Moment** educational landing page in `web/views/simulation_teachable.tmpl` (Status: **NOT PASSED** vs **PASSED** on Spam Report).
  - Admin REST APIs for campaign creation, safe local mailbox delivery (`LaunchCampaign`), and live failure metrics (`/api/v1/admin/simulations`).
  - Interactive SVG Donut & metrics analytics dashboard tab in the Admin Web Portal.
- **Antivirus & Anti-Spam Filtering (ClamAV & Rspamd)**:
  - New pure-Go scanner package in `internal/antivirus` supporting ClamAV `INSTREAM` protocol (Unix/TCP socket) and HTTP-based Rspamd `/checkv2` API.
  - Inbound SMTP pipeline defense automatically rejecting virus-infected emails (`554 5.7.1`) and routing spam messages to the `Spam` mailbox with `[SPAM]` subject tags.
  - Graceful fail-open capability ensuring zero email loss if scanning services are temporarily unreachable.
- **Inbound DMARC, SPF & DKIM Protocol Verification**:
  - New verification engine in `internal/dmarc` checking SPF DNS records, cryptographic DKIM signatures, and evaluating domain DMARC policies (`p=none`, `p=quarantine`, `p=reject`).
  - Automatic injection of RFC 8601 compliant `Authentication-Results` headers into stored `.eml` files.
- **Automated S3-Compatible Cloud Backups**:
  - New cloud disaster recovery package in `internal/backup` creating non-blocking SQLite database snapshots (`VACUUM INTO`) and tar.gz compressed Maildir archives.
  - End-to-end client-side encryption using **AES-256 GCM**.
  - Object storage upload integration supporting AWS S3, Cloudflare R2, MinIO, and Wasabi.
  - Admin management API endpoints (`/api/v1/admin/backup/config` and `/api/v1/admin/backup/trigger`).

---

## [1.3.0] - 2026-10-29

### Added
- **Inbound Trust & Unknown Domain Warning Banner**:
  - Pure-Go sender reputation evaluation engine (`internal/trust`) with 5-stage trust assessment (Authentication status, local domain whitelist, contact address book check, prior message interaction history, and payload scanning).
  - First-time external sender detection with risk level classification:
    - 🚨 `danger`: SPF/DKIM/DMARC failure or sender spoofing.
    - ⚠️ `warning`: Unknown external sender with file attachments or external URLs.
    - ℹ️ `info`: Unknown external sender with plain text message.
  - Interactive Webmail safety banner placed above the email body reader with visual warning badges.
  - One-click **"Mark as Trusted / Add to Contacts"** remediation: Adds sender to personal address book and dismisses banner.
  - One-click **"Move to Spam"** action: Relocates untrusted emails directly to the spam folder.
- **Address Book & Contact Auto-Complete**:
  - Full CRUD contact management API (`/api/v1/webmail/contacts`) supporting name, email, company, and notes.
  - Interactive Address Book modal in the Webmail sidebar with real-time contact filtering.
  - Composer recipient auto-complete dropdown for `To`, `Cc`, and `Bcc` inputs with 150ms debounce.
  - Background auto-harvesting function automatically adding unique recipient addresses from sent emails into the user's personal address book.
- **Vacation Notice / Out-of-Office Auto-Responder**:
  - Out-of-office scheduling engine (`auto_responders` table) with customizable start/end dates, subject, and text body.
  - Dedicated configuration modal in the Webmail client.
  - Anti-looping defense and frequency throttling ensuring senders receive at most 1 automated reply per 24 hours (`auto_responder_logs` table).
- **Canned Responses & Quick Templates**:
  - Pre-written email snippet repository (`canned_responses` table) with management modal and one-click insertion into the WYSIWYG composer editor.
  - Quick-insert dropdown in the composer toolbar.

---

## [1.2.0] - 2026-10-29

### Added
- **Two-Factor Authentication (2FA / TOTP RFC 6238)**:
  - New pure-Go TOTP implementation in `internal/totp` with Base32 secret generation, HMAC-SHA1 algorithm, dynamic truncation, and time-skew window validation.
  - Recovery backup code generator (`internal/totp/GenerateBackupCodes`) producing 8 one-time emergency codes stored with `bcrypt` hashes in `backup_codes` table.
  - Interactive Security & 2FA modal in Webmail UI with secret key display and standard `otpauth://` URI export.
  - Two-step authentication flow on `/login`: step 1 (Email + Password) $\rightarrow$ step 2 (TOTP verification prompt via `/api/v1/auth/verify-2fa`).
  - Account security settings in Webmail sidebar allowing self-service 2FA setup, verification, and password-protected deactivation.
- **Developer Transactional REST API**:
  - New API key generator and verifier in `internal/apikey` creating cryptographically random tokens prefixed with `rhm_live_` and hashed with SHA-256 for secure storage.
  - `APIKeyAuthMiddleware` enforcing `Authorization: Bearer rhm_live_...` authentication and asynchronous `last_used_at` telemetry tracking.
  - REST API endpoint `POST /api/v1/send` enabling programmatic email delivery via JSON over HTTPS with DKIM signing and automatic local/outbound routing.
  - Management API endpoints for listing, creating, and revoking API keys (`/api/v1/api-keys`).
  - Interactive "Developer API Keys" dashboard tab in the Webmail Security modal.
- **Catch-All Mailboxes & Domain Aliases**:
  - Domain-level `catch_all_user_id` field enabling automatic redirection of unmatched recipient addresses (`*@domain.com`) to a designated mailbox.
  - Inbound SMTP pipeline enhancement in `internal/smtpserver/server.go` delivering unmatched recipient emails to the active Catch-All user rather than rejecting with `550 Mailbox Not Found`.
  - Admin API endpoint `POST /api/v1/domains/:id/catch-all` to configure or remove Catch-All mailboxes.

---

## [1.1.0] - 2026-10-28

### Added
- **Security Hardening**:
  - Implementation of `SmugglingReader` stream wrapper rejecting naked LF characters to mitigate SMTP Smuggling vulnerabilities (**CVE-2023-51764 & CVE-2023-51765**).
  - Implementation of pure-Go HTML sanitizer engine in `internal/sanitizer` to eliminate Stored XSS vectors (script tags, iframe embeds, object tags, on* event handlers, and javascript: protocols).
  - Introduction of thread-safe Token Bucket Rate Limiting middleware (`internal/api/middleware/ratelimit.go`) for `/login`, `/register`, and `/send` endpoints with automatic IP cleanup.
  - Hardened physical storage architecture (`internal/storage`) using randomized SHA-256 filenames and strict base-directory traversal validation.
- **Webmail Usability**:
  - Mailbox pagination engine with dynamic query parameters (`?folder=...&page=1&limit=25`) and responsive Prev/Next navigation controls.
  - Composer draft auto-save engine synchronizing unsaved messages to `localStorage` every second with draft restoration and cleanup prompts.
  - In-browser live preview modal for image attachments (`.png`, `.jpg`, `.jpeg`, `.gif`, `.webp`), PDFs, and text files (`.txt`, `.log`, `.csv`, `.json`).

---

## [1.0.0] - 2026-10-28

### Added
- **Core Mail Protocols**:
  - Implementation of SMTP Inbound (Port 25) with strict anti-open-relay validation.
  - Implementation of SMTP Submission (Port 587/2525) with SASL PLAIN/LOGIN authentication via `bcrypt`.
  - Implementation of IMAP v4rev1 server (Port 143/993) for desktop and mobile mail clients.
- **Outbound MTA & Cryptography**:
  - Direct MTA outbound delivery engine backed by an automated DNS MX record resolver.
  - Dynamic 2048-bit RSA key pair generation and automated DKIM digital header signing.
- **Zero-Node.js Frontend Architecture**:
  - Full migration of user interface views to native Go templates (`.tmpl`) with `//go:embed` memory compilation.
  - Direct embedding of Bootstrap 5.3 CSS & JavaScript without requiring Node.js or npm runtimes.
  - Inlined Bootstrap Icons WOFF2 font as Base64 Data URI inside the CSS stylesheet to ensure 100% icon rendering reliability regardless of CORS or MIME-type configurations.
  - Standardized modern Light Mode UI across all web surfaces.
- **Webmail Portal & Reader**:
  - Split-view modern mailbox layout (message list pane on the left, reader detail pane on the right).
  - Complete folder structure: `INBOX`, `Sent`, `Scheduled`, `Drafts`, `Spam`, and `Trash`.
  - Anti-XSS sandboxed iframe email body renderer.
  - Integrated authentication badges: `SPF: PASS`, `DKIM: PASS`, and `DMARC: PASS`.
- **WYSIWYG Rich-Text Composer**:
  - Interactive modal composer with lightweight vanilla JavaScript text formatting toolbar.
  - Multi-recipient inputs with toggleable `Cc` and `Bcc` fields.
  - Multi-file attachment uploader with chip-based file size displays and item deletion triggers.
  - One-click `Reply` and `Forward` actions with automatic subject formatting and original message quoting.
- **Email Scheduler Engine**:
  - Native scheduled delivery feature integrated directly into the composer modal.
  - Background periodic worker (15-second interval) automatically dispatching due scheduled emails.
  - Dedicated `⏰ Scheduled` folder with the ability to cancel pending deliveries at any time.
- **Authentication & Self-Service Registration**:
  - Self-service mailbox registration portal (`/register`) with active domain selector dropdown.
  - Real-time username availability validation via AJAX (`/api/v1/auth/check-username`).
  - Pure-Go mathematical anti-bot challenge (Math Captcha) secured via HMAC-SHA256 signatures with zero third-party tracking.
  - Secure session management based on HMAC-SHA256 HTTP-only cookies.
- **Administration & Monitoring**:
  - 4-step onboarding Setup Wizard (`/setup`).
  - Live statistics dashboard powered by Server-Sent Events (SSE).
- **Deployment & Containerization**:
  - Ultra-small multi-stage `Dockerfile` with a final runtime footprint of ~51.3 MB.
  - Production-ready Traefik reverse proxy configuration (`docker-compose.traefik.yml` and `deploy.sh`).
- **Multi-Database Support**:
  - Pure-Go SQLite WAL mode via `glebarez/sqlite` with zero CGO dependencies.
  - Native PostgreSQL support for high-throughput enterprise workloads.

### Changed
- Rebranded the entire codebase, templates, and documentation to **RaHaMail**.
- Removed the dark/light mode toggle in favor of a unified, clean, and accessible Light Theme.
- Enhanced database query performance by enforcing a `.Limit(100)` clause on mailbox listings to prevent memory exhaustion.
- Configured `GIN_MODE=release` as the default server runtime mode.

### Removed
- Removed all dependencies on Node.js, npm, webpack, and `node_modules`.
- Removed all external third-party domain references to maintain an open-source codebase.

---

### Licensing & Support
- **License**: **GPLv3** (Open-Source / Non-Commercial Use).
- **Commercial Inquiries & Donations**: PayPal to **`ridwan.haqi@gmail.com`**.
