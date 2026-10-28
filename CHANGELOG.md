# CHANGELOG: RaHaMail

All notable changes to the **RaHaMail** project will be documented in this file. The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/), and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

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
