package backup

import (
	"os"
	"testing"
)

func TestCreateEncryptedArchive(t *testing.T) {
	// Buat file dummy
	tmpDB, _ := os.CreateTemp("", "testdb-*.db")
	defer os.Remove(tmpDB.Name())
	tmpDB.WriteString("SQLITE DUMMY DATA")
	tmpDB.Close()

	data, err := CreateEncryptedArchive(tmpDB.Name(), "", "my-secret-passphrase")
	if err != nil {
		t.Fatalf("CreateEncryptedArchive failed: %v", err)
	}

	if len(data) == 0 {
		t.Fatalf("Expected non-empty archive, got 0 bytes")
	}
}
