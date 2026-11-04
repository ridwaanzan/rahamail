---
name: mail-server-development
description: >-
  Comprehensive guide and architectural reference for developing, maintaining, testing, 
  and operating the RaHaMail pure-Go mail server and webmail ecosystem. Covers SMTP/IMAP protocols, 
  SASL authentication, automated TLS engines, anti-spam/antivirus pipelines, DKIM/SPF/DMARC verification, 
  employee phishing awareness simulation, RHUI v1.0.4 SSR frontend, and single-binary deployment.
---

# RaHaMail: Pure-Go Mail Server & Webmail Development Skill

This skill guides AI agents and engineers in developing, auditing, maintaining, and extending **RaHaMail**, an enterprise-grade, single-binary, pure-Go mail server and interactive Webmail application (`github.com/ridwaanzan/rahamail`).

---

## 1. Architectural Philosophy & Mental Model

RaHaMail is built with strict architectural boundaries:
1. **Single Binary Executable**: Zero external runtime requirements (no Node.js in production, no Python, no separate C web server). Compiled with `CGO_ENABLED=0` where possible or standard Go toolchain.
2. **Concurrent Multi-Port Listeners**: Concurrently runs SMTP Inbound (`:25`), SMTP Submission (`:587`), IMAP (`:143`), IMAPS (`:993`), and Web UI/API (`:9000`).
3. **Defense-in-Depth Security**: Built-in mitigations for SMTP Smuggling (RFC 5321), stored XSS (backend DOM sanitization), brute-force throttling, and fail-open ClamAV/Rspamd integration.
4. **RHUI SSR Component Architecture**: All frontend interfaces are rendered server-side using **RHUI v1.0.4** (`github.com/suserc0de/rhui`) with pre-compiled AST nodes, zero-allocation fast paths, and atomic concurrency snapshotting.
5. **Storage Isolation**: Raw `.eml` Maildir files are addressed cryptographically using `SHA-256(content)` under `var/mail/{folder}/{userID}/{hash}.eml` to defeat path traversal and directory tampering.

---

## 2. Directory Structure & Key Subsystems

```
mail-server/
├── main.go                     # Concurrent engine runner (SMTP, IMAP, Web, Schedulers)
├── internal/
│   ├── smtpserver/             # SMTP Inbound (:25) & Submission (:587) with SASL PLAIN/LOGIN & smuggling protection
│   ├── imapserver/             # IMAP4rev1 (:143) & IMAPS (:993) mailbox server
│   ├── tlsconfig/              # Auto-TLS engine (loads external certs or provisions RSA 2048-bit fallbacks)
│   ├── delivery/               # Outbound MTA with DNS MX priority resolution & direct delivery
│   ├── antivirus/              # ClamAV INSTREAM socket scanning & Rspamd HTTP API (/checkv2, /learnspam, /learnham)
│   ├── dmarc/                  # DMARC policy enforcement, SPF, and DKIM verification
│   ├── trust/                  # Inbound sender reputation & suspicious link/domain detection
│   ├── simulation/             # Employee Phishing & Spam Awareness engine (tracking tokens, metrics, presets)
│   ├── storage/                # SHA-256 hashed Maildir storage manager and quota reconciler
│   ├── mailbuilder/            # RFC 5322 MIME compilation (multipart/alternative, attachments)
│   ├── mailreader/             # RFC 5322 MIME parser and body extractor
│   ├── backup/                 # AES-256 GCM encrypted SQLite & maildir snapshot uploader (S3, R2, Wasabi)
│   ├── totp/                   # RFC 6238 TOTP 2FA engine & backup emergency recovery codes
│   ├── apikey/                 # Developer transactional mail REST API token management (rhm_live_...)
│   ├── sanitizer/              # Strict HTML DOM tag & attribute sanitizer (Anti-Stored XSS)
│   ├── scheduler/              # Outbound retry & delayed message worker goroutines
│   ├── sse/                    # Server-Sent Events hub for live real-time admin monitoring
│   └── api/
│       ├── controllers/        # Gin controllers (mail, auth, admin, simulations, backup)
│       └── middleware/         # Session HMAC-SHA256, Super Admin RBAC, and Token Bucket rate limiters
├── web/
│   ├── web.go                  # RHUI v1.0.4 engine initialization & template renderer
│   ├── render_helper.go        # Gin layout rendering helpers
│   └── views/                  # Component-based .rhui templates (webmail, admin, auth, components)
└── .agents/skills/             # Agent skills (rhui, mail-server)
```

---

## 3. Protocol Engines & Network Ports

| Service | Port Env Var | Default | Protocol / Feature | Security Controls |
|---|---|---|---|---|
| **SMTP Inbound** | `SMTP_PORT` | `:25` | Server-to-Server MX ingestion | Anti-Open Relay, Smuggling Reader, ClamAV, Rspamd |
| **SMTP Submission** | `SUBMISSION_PORT` | `:587` | Client mail sending | SASL `PLAIN` & `LOGIN` (`bcrypt`), STARTTLS mandatory |
| **IMAP** | `IMAP_PORT` | `:143` | Mailbox synchronization | IMAP4rev1, STARTTLS |
| **IMAPS** | `IMAPS_PORT` | `:993` | Direct SSL/TLS IMAP | Direct TLS wrapper (`StartIMAPServerTLS`) |
| **Web UI & REST API** | `PORT` | `:9000` | Webmail, Admin, REST API, SSE | Session HMAC-SHA256, Super Admin RBAC, Rate Limiting |

### Automated TLS Engine (`internal/tlsconfig/tls.go`)
- Check order:
  1. Paths specified by `TLS_CERT_FILE` & `TLS_KEY_FILE`.
  2. Local persistent fallback at `data/server.crt` & `data/server.key`.
  3. Automatic generation of self-signed RSA 2048-bit certificate with SAN `localhost`, `127.0.0.1`, and hostname.

---

## 4. Webmail & Admin UI Development (RHUI v1.0.4)

When modifying or adding UI views in `web/views/`:

1. **Use Component Syntax**:
   - Master layout: `<layout:admin>` or `RenderWithLayout(c, code, "layouts/admin", "view_name", data)`.
   - Iterations: Always use `<each items="list" as="item">` with loop metadata `@index`, `@first`, `@last`.
   - Conditional logic: Use `<if test="len(list) == 0">` and `<else>`.
2. **Strict Attribute Binding**:
   - Dynamic values must use `:attr="expression"` (e.g. `:value="next"` or `:data-id="msg.ID"`).
   - Never inject unescaped template expressions inside native inline event handlers like `onclick="{{ ... }}"`. Use `:data-*` attributes and read with `this.getAttribute(...)`.
3. **Design System & Dark Mode**:
   - Standard Tailwind CSS palette with `light` and `dark` mode variants.
   - Modals must use the custom modal system (`showModalAlert`, `showModalConfirm`, `showToast`) instead of native browser `alert()` or `confirm()`.

---

## 5. Security Awareness & Phishing Simulator Workflows

### Campaign Execution Flow
1. **Creation**: Admin creates campaign via `admin_simulations.rhui` or selects built-in presets (`internal/simulation/simulation.go`).
2. **Dispatch (`LaunchCampaign`)**: Injects encrypted tracking code into `{{.TrackingLink}}` (`/sim/c/:code`) and delivers directly into employee `INBOX`.
3. **Interactions**:
   - **Clicking link**: Hit `/sim/c/:code`, status becomes `clicked` (**NOT PASSED**), redirects to `/views/simulation_teachable.rhui`.
   - **Reporting phishing**: User clicks **"Laporkan Phishing"** in Webmail reader toolbar, status becomes `reported` (**PASSED**), displays positive reinforcement modal.
4. **Bayesian Training**: Non-simulation spam actions trigger asynchronous background training with Rspamd (`/learnspam` or `/learnham`).

---

## 6. Testing, Verification & CI/CD Guidelines

Always run tests with the Go race detector enabled before committing any architectural or protocol changes:

```bash
# 1. Run full test suite with race condition detection
go test -race ./...

# 2. Run targeted subsystem tests
go test -v ./internal/smtpserver -run TestSASLLogin
go test -v ./internal/imapserver -run TestIMAP
go test -v ./internal/api/controllers -run TestPhishing
go test -v ./web -run TestRHUI

# 3. Verify single binary compilation
go build -v -o /tmp/mail-server-test main.go && rm -f /tmp/mail-server-test
```

### Git Branching Rules for RaHaMail
- **Branch Naming**: Always branch directly off `master` with standard semantic naming (`v.1.X.X` or `feat/...`).
- **Clean Diffs**: Never commit SQLite binaries (`gomail.db`, `gomail.db-wal`) or runtime storage (`data/`).
