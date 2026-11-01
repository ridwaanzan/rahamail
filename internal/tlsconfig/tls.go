package tlsconfig

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"log"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// LoadOrGenerateTLSConfig memuat sertifikat TLS dari file (jika disediakan via env TLS_CERT_FILE & TLS_KEY_FILE
// atau file sertifikat default), atau secara otomatis membuat self-signed cert jika belum ada file cert.
func LoadOrGenerateTLSConfig(domainName string) (*tls.Config, error) {
	certFile := strings.TrimSpace(os.Getenv("TLS_CERT_FILE"))
	keyFile := strings.TrimSpace(os.Getenv("TLS_KEY_FILE"))

	// 1. Coba baca dari file yang didefinisikan pengguna
	if certFile != "" && keyFile != "" {
		if cert, err := tls.LoadX509KeyPair(certFile, keyFile); err == nil {
			log.Printf("🔒 [TLS-ENGINE] Berhasil memuat sertifikat TLS resmi dari: %s", certFile)
			return &tls.Config{
				Certificates: []tls.Certificate{cert},
				MinVersion:   tls.VersionTLS12,
			}, nil
		} else {
			log.Printf("⚠️ [TLS-ENGINE] Gagal membaca sertifikat TLS dari %s: %v. Mencoba fallback...", certFile, err)
		}
	}

	// 2. Cek lokasi default cert di direktori data / app
	defaultCertPaths := []struct {
		cert string
		key  string
	}{
		{"/app/data/certs/fullchain.pem", "/app/data/certs/privkey.pem"},
		{"./data/certs/fullchain.pem", "./data/certs/privkey.pem"},
		{"./certs/fullchain.pem", "./certs/privkey.pem"},
	}

	for _, p := range defaultCertPaths {
		if _, err := os.Stat(p.cert); err == nil {
			if _, err := os.Stat(p.key); err == nil {
				if cert, err := tls.LoadX509KeyPair(p.cert, p.key); err == nil {
					log.Printf("🔒 [TLS-ENGINE] Berhasil memuat sertifikat TLS dari default path: %s", p.cert)
					return &tls.Config{
						Certificates: []tls.Certificate{cert},
						MinVersion:   tls.VersionTLS12,
					}, nil
				}
			}
		}
	}

	// 3. Cek apakah ada file cert self-signed yang sudah disimpan di disk sebelumnya
	autoDir := os.Getenv("DATA_DIR")
	if autoDir == "" {
		autoDir = "./data"
	}
	cachedCert := filepath.Join(autoDir, "server.crt")
	cachedKey := filepath.Join(autoDir, "server.key")

	if _, err := os.Stat(cachedCert); err == nil {
		if _, err := os.Stat(cachedKey); err == nil {
			if cert, err := tls.LoadX509KeyPair(cachedCert, cachedKey); err == nil {
				log.Printf("🔒 [TLS-ENGINE] Memuat sertifikat TLS self-signed tersimpan: %s", cachedCert)
				return &tls.Config{
					Certificates: []tls.Certificate{cert},
					MinVersion:   tls.VersionTLS12,
				}, nil
			}
		}
	}

	// 4. Generate sertifikat RSA 2048-bit self-signed baru secara otomatis
	log.Printf("🛡️ [TLS-ENGINE] Tidak ada sertifikat eksternal. Membuat Self-Signed TLS Certificate untuk domain: %s...", domainName)
	cert, err := generateSelfSignedCert(domainName, cachedCert, cachedKey)
	if err != nil {
		log.Printf("❌ [TLS-ENGINE] Gagal membuat self-signed TLS cert: %v", err)
		return nil, err
	}

	return &tls.Config{
		Certificates: []tls.Certificate{cert},
		MinVersion:   tls.VersionTLS12,
	}, nil
}

func generateSelfSignedCert(domainName, saveCertPath, saveKeyPath string) (tls.Certificate, error) {
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return tls.Certificate{}, err
	}

	notBefore := time.Now().Add(-1 * time.Hour)
	notAfter := notBefore.Add(365 * 24 * time.Hour) // Berlaku 1 tahun

	serialNumberLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	serialNumber, err := rand.Int(rand.Reader, serialNumberLimit)
	if err != nil {
		return tls.Certificate{}, err
	}

	cleanDomain := strings.TrimSpace(domainName)
	if cleanDomain == "" {
		cleanDomain = "localhost"
	}

	template := x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			Organization: []string{"RaHaMail Secure Mail Server"},
			CommonName:   cleanDomain,
		},
		NotBefore:             notBefore,
		NotAfter:              notAfter,
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
		BasicConstraintsValid: true,
	}

	// Tambahkan SAN (Subject Alternative Names) untuk domain dan IP
	dnsNames := []string{cleanDomain, "localhost", "mail." + cleanDomain}
	for _, d := range dnsNames {
		template.DNSNames = append(template.DNSNames, d)
	}
	template.IPAddresses = []net.IP{net.ParseIP("127.0.0.1"), net.ParseIP("::1")}

	derBytes, err := x509.CreateCertificate(rand.Reader, &template, &template, &priv.PublicKey, priv)
	if err != nil {
		return tls.Certificate{}, err
	}

	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: derBytes})
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(priv)})

	// Simpan ke disk jika direktori dapat ditulis
	if err := os.MkdirAll(filepath.Dir(saveCertPath), 0755); err == nil {
		_ = os.WriteFile(saveCertPath, certPEM, 0644)
		_ = os.WriteFile(saveKeyPath, keyPEM, 0600)
	}

	cert, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		return tls.Certificate{}, err
	}

	log.Printf("✅ [TLS-ENGINE] Self-Signed TLS Certificate berhasil dibuat (RSA 2048-bit, SAN: %s)", strings.Join(dnsNames, ", "))
	return cert, nil
}
