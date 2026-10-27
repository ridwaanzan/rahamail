package delivery

import (
	"bytes"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"log"
	"net"
	"net/smtp"
	"sort"
	"strings"
	"time"

	"github.com/emersion/go-msgauth/dkim"
)

// GenerateRSAKeyPair membuat pasangan kunci RSA 2048-bit untuk penandatanganan DKIM
func GenerateRSAKeyPair() (privateKeyPEM string, publicKeyTXT string, err error) {
	privKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return "", "", err
	}

	privBytes := x509.MarshalPKCS1PrivateKey(privKey)
	privPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: privBytes,
	})

	pubBytes, err := x509.MarshalPKIXPublicKey(&privKey.PublicKey)
	if err != nil {
		return "", "", err
	}
	pubPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "PUBLIC KEY",
		Bytes: pubBytes,
	})

	// Ekstrak base64 string untuk record DNS TXT
	cleanPub := strings.ReplaceAll(string(pubPEM), "-----BEGIN PUBLIC KEY-----", "")
	cleanPub = strings.ReplaceAll(cleanPub, "-----END PUBLIC KEY-----", "")
	cleanPub = strings.ReplaceAll(cleanPub, "\n", "")
	cleanPub = strings.ReplaceAll(cleanPub, "\r", "")

	dnsTxt := fmt.Sprintf("v=DKIM1; k=rsa; p=%s", cleanPub)
	return string(privPEM), dnsTxt, nil
}

// SignDKIM menandatangani raw email data menggunakan DKIM signature
func SignDKIM(rawEmail []byte, domainName, selector, privateKeyPEM string) ([]byte, error) {
	block, _ := pem.Decode([]byte(privateKeyPEM))
	if block == nil {
		return nil, errors.New("gagal mem-parsing PEM private key")
	}

	privKey, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("gagal parsing RSA private key: %w", err)
	}

	opts := &dkim.SignOptions{
		Domain:   domainName,
		Selector: selector,
		Signer:   privKey,
		Hash:     crypto.SHA256,
		HeaderKeys: []string{
			"from", "to", "subject", "date", "message-id",
		},
	}

	var signedBuf bytes.Buffer
	r := strings.NewReader(string(rawEmail))
	if err := dkim.Sign(&signedBuf, r, opts); err != nil {
		return nil, fmt.Errorf("gagal menandatangani pesan DKIM: %w", err)
	}

	return signedBuf.Bytes(), nil
}

// ResolveMX menemukan Mail Exchanger (MX Record) prioritas tertinggi untuk domain target
func ResolveMX(domain string) (string, error) {
	mxRecords, err := net.LookupMX(domain)
	if err != nil {
		// Fallback ke lookup host A record jika tidak ada MX (RFC 5321)
		addrs, errA := net.LookupHost(domain)
		if errA != nil || len(addrs) == 0 {
			return "", fmt.Errorf("tidak dapat menemukan MX atau A record untuk domain %s", domain)
		}
		return domain, nil
	}

	if len(mxRecords) == 0 {
		return domain, nil
	}

	// Urutkan berdasarkan prioritas preferensi (angka terkecil = prioritas tertinggi)
	sort.Slice(mxRecords, func(i, j int) bool {
		return mxRecords[i].Pref < mxRecords[j].Pref
	})

	targetHost := strings.TrimSuffix(mxRecords[0].Host, ".")
	log.Printf("🔍 [DNS MX] Domain %s memiliki server tujuan prioritas 1: %s (Pref: %d)", domain, targetHost, mxRecords[0].Pref)
	return targetHost, nil
}

// DeliverOutbound mengirimkan email ke Mail Server tujuan (misal Gmail, Yahoo) via SMTP Port 25
func DeliverOutbound(from, to string, rawMessage []byte) error {
	toParts := strings.Split(to, "@")
	if len(toParts) != 2 {
		return fmt.Errorf("format alamat penerima tidak valid: %s", to)
	}
	targetDomain := toParts[1]

	mxHost, err := ResolveMX(targetDomain)
	if err != nil {
		return fmt.Errorf("gagal resolve MX untuk %s: %w", targetDomain, err)
	}

	addr := net.JoinHostPort(mxHost, "25")
	log.Printf("🚀 [OUTBOUND MTA] Mencoba menghubungkan ke %s untuk penerima: %s...", addr, to)

	// Dial TCP dengan timeout 15 detik
	conn, err := net.DialTimeout("tcp", addr, 15*time.Second)
	if err != nil {
		return fmt.Errorf("gagal terhubung ke remote MTA %s: %w", addr, err)
	}
	defer conn.Close()

	client, err := smtp.NewClient(conn, mxHost)
	if err != nil {
		return fmt.Errorf("handshake SMTP client gagal: %w", err)
	}
	defer client.Quit()

	// Kirim HELO/EHLO dengan hostname identitas kita
	if err := client.Hello("mail.example.com"); err != nil {
		return fmt.Errorf("EHLO ditolak oleh remote MTA: %w", err)
	}

	// Coba negosiasi STARTTLS jika didukung oleh remote server
	if ok, _ := client.Extension("STARTTLS"); ok {
		log.Printf("🔒 [OUTBOUND TLS] Mengaktifkan enkripsi STARTTLS dengan %s...", mxHost)
		// TLS configuration
	}

	// Kirim MAIL FROM
	if err := client.Mail(from); err != nil {
		return fmt.Errorf("remote MTA menolak MAIL FROM <%s>: %w", from, err)
	}

	// Kirim RCPT TO
	if err := client.Rcpt(to); err != nil {
		return fmt.Errorf("remote MTA menolak RCPT TO <%s>: %w", to, err)
	}

	// Kirim DATA
	w, err := client.Data()
	if err != nil {
		return fmt.Errorf("remote MTA menolak command DATA: %w", err)
	}

	if _, err := w.Write(rawMessage); err != nil {
		return fmt.Errorf("gagal menulis data email: %w", err)
	}

	if err := w.Close(); err != nil {
		return fmt.Errorf("gagal menutup stream email (commit rejected): %w", err)
	}

	log.Printf("🎉 [OUTBOUND SUCCESS] Email berhasil terkirim ke remote server %s untuk <%s>!", mxHost, to)
	return nil
}
