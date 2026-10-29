package repository

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"mail-server/internal/domain"

	"github.com/glebarez/sqlite"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var DB *gorm.DB
var ActiveDriver string

// InitDB menginisialisasi database dengan dukungan multi-database (PostgreSQL atau SQLite dengan mode WAL teroptimasi)
func InitDB() *gorm.DB {
	driverType := strings.ToLower(os.Getenv("DB_DRIVER")) // "postgres" atau "sqlite"
	databaseURL := os.Getenv("DATABASE_URL")

	var dialector gorm.Dialector

	// Deteksi otomatis atau via DB_DRIVER
	if driverType == "postgres" || (driverType == "" && strings.HasPrefix(databaseURL, "postgres://")) {
		ActiveDriver = "PostgreSQL"
		log.Printf("🐘 Menggunakan database PostgreSQL (%s)...", maskDSN(databaseURL))
		dialector = postgres.Open(databaseURL)
	} else {
		ActiveDriver = "SQLite (WAL Mode)"
		dbPath := os.Getenv("SQLITE_PATH")
		if dbPath == "" {
			dbPath = "gomail.db"
		}

		// Optimasi PRAGMA: Write-Ahead Logging (WAL), Busy Timeout 5000ms, synchronous=NORMAL, cache 64MB
		sqliteDSN := fmt.Sprintf("%s?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)&_pragma=synchronous(NORMAL)&_pragma=cache_size(-64000)&_pragma=foreign_keys(ON)", dbPath)
		log.Printf("🗄️ Menggunakan database SQLite Teroptimasi: %s (Mode: WAL)...", dbPath)
		dialector = sqlite.Open(sqliteDSN)
	}

	var err error
	DB, err = gorm.Open(dialector, &gorm.Config{
		Logger: logger.Default.LogMode(logger.Warn),
	})
	if err != nil {
		log.Fatalf("❌ Gagal terhubung ke database (%s): %v", ActiveDriver, err)
	}

	// Konfigurasi connection pool pada underlying sql.DB
	sqlDB, err := DB.DB()
	if err == nil {
		if ActiveDriver == "SQLite (WAL Mode)" {
			// SQLite bekerja optimal dengan connection pool yang terkontrol
			sqlDB.SetMaxOpenConns(50)
			sqlDB.SetMaxIdleConns(10)
			sqlDB.SetConnMaxLifetime(1 * time.Hour)
		} else {
			sqlDB.SetMaxOpenConns(100)
			sqlDB.SetMaxIdleConns(20)
			sqlDB.SetConnMaxLifetime(30 * time.Minute)
		}
	}

	// Auto Migrate Tabel
	err = DB.AutoMigrate(
		&domain.Domain{},
		&domain.User{},
		&domain.BackupCode{},
		&domain.APIKey{},
		&domain.Alias{},
		&domain.Message{},
		&domain.ScheduledMail{},
		&domain.Attachment{},
		&domain.Contact{},
		&domain.AutoResponder{},
		&domain.AutoResponderLog{},
		&domain.CannedResponse{},
		&domain.SystemConfig{},
	)
	if err != nil {
		log.Fatalf("❌ Gagal migrasi database: %v", err)
	}

	log.Printf("✅ Database %s terhubung dan migrasi skema berhasil dijalankan.", ActiveDriver)
	seedInitialData(DB)
	return DB
}

func maskDSN(dsn string) string {
	if len(dsn) < 15 {
		return dsn
	}
	return dsn[:12] + "..."
}

// seedInitialData mengisi domain & user admin default jika database masih kosong
func seedInitialData(db *gorm.DB) {
	var domainCount int64
	db.Model(&domain.Domain{}).Count(&domainCount)
	if domainCount == 0 {
		// Generate real RSA 2048-bit DKIM Key Pair
		privPEM, pubTXT, err := generateRSAKeyPair()
		if err != nil {
			privPEM = ""
			pubTXT = "v=DKIM1; k=rsa; p=MIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIBCgKCAQEA..."
		}

		defaultDomain := domain.Domain{
			Name:           "example.com",
			DKIMSelector:   "mail",
			DKIMPrivateKey: privPEM,
			DKIMPublicKey:  pubTXT,
			IsActive:       true,
		}
		db.Create(&defaultDomain)

		passHash, _ := bcrypt.GenerateFromPassword([]byte("admin123"), bcrypt.DefaultCost)
		adminUser := domain.User{
			DomainID:     defaultDomain.ID,
			Email:        "admin@example.com",
			PasswordHash: string(passHash),
			IsAdmin:      true,
			IsActive:     true,
			QuotaBytes:   5 * 1024 * 1024 * 1024, // 5 GB
		}
		db.Create(&adminUser)

		// Seed pesan inbox contoh
		db.Create(&domain.Message{
			UserID:      adminUser.ID,
			MessageID:   "<welcome.2026@rahamail.local>",
			Mailbox:     "INBOX",
			From:        "system@rahamail.local",
			To:          "admin@example.com",
			Subject:     "Selamat Datang di RaHaMail Server v1.0",
			Snippet:     "Server email RaHaMail Anda dengan Gin Engine & Bootstrap 5 telah siap digunakan...",
			SPFStatus:   "pass",
			DKIMStatus:  "pass",
			DMARCStatus: "pass",
			StoragePath: "var/mail/inbox/welcome.eml",
			IsSeen:      false,
			ReceivedAt:  time.Now(),
		})

		log.Println("🌱 Data awal (seed) berhasil dibuat: admin@example.com / admin123")
	}
}

func generateRSAKeyPair() (string, string, error) {
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
	cleanPub := strings.ReplaceAll(string(pubPEM), "-----BEGIN PUBLIC KEY-----", "")
	cleanPub = strings.ReplaceAll(cleanPub, "-----END PUBLIC KEY-----", "")
	cleanPub = strings.ReplaceAll(cleanPub, "\n", "")
	cleanPub = strings.ReplaceAll(cleanPub, "\r", "")
	return string(privPEM), fmt.Sprintf("v=DKIM1; k=rsa; p=%s", cleanPub), nil
}
