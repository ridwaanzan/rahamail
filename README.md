# RaHaMail (RaHa Mail Server)

> Modern, ultra-lightweight, high-performance, single-binary Mail Server written in Go with Web GUI Dashboard, SMTP Inbound/Submission, IMAP Engine, Server-Sent Events (SSE), and embedded SQLite WAL storage.

---

## 🌟 Key Features

- **True Single Static Binary**: Zero external daemon dependencies.
- **Embedded Web GUI & Setup Wizard**: Step-by-step 4-step onboarding wizard at `/setup`.
- **Pure-Go Architecture (Zero Node.js / Zero CGO)**: Web templates and Bootstrap assets embedded directly inside the binary via `go:embed`.
- **Multi-Database Support**: High-performance SQLite with Write-Ahead Logging (WAL mode) and PostgreSQL compatibility.
- **Protocols Supported**:
  - SMTP Inbound (Port 25) with anti-open relay enforcement.
  - SMTP Authenticated Submission (Port 587/465).
  - IMAP4rev1 Server (Port 143/993) for Mozilla Thunderbird, Outlook, and Apple Mail.
- **Automated DKIM & Security**: Dynamic 2048-bit RSA key generation and DKIM signing.
- **Docker & Podman Ready**: Ultra-small image (~50 MB) optimized for Traefik reverse proxy.

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

## 🚀 Quick Start

### Run locally:
```bash
go run main.go
```
Open **`http://localhost:8080/setup`** in your browser.

### Deploy with Podman & Traefik:
```bash
./deploy.sh
```
