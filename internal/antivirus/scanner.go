package antivirus

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"io"
	"net"
	"net/http"
	"os"
	"strings"
	"time"
)

// ScanResult merepresentasikan hasil scanning email dari ClamAV dan Rspamd
type ScanResult struct {
	IsInfected bool
	VirusName  string
	IsSpam     bool
	SpamScore  float64
	Action     string // "no_action", "add_header", "rewrite_subject", "reject"
}

// GetClamAVAddress mengambil alamat ClamAV daemon dari environment (TCP host:port atau unix socket path)
func GetClamAVAddress() string {
	addr := os.Getenv("CLAMAV_ADDR")
	if addr == "" {
		return "127.0.0.1:3310" // Default ClamAV clamd TCP port
	}
	return addr
}

// GetRspamdURL mengambil URL HTTP daemon Rspamd dari environment
func GetRspamdURL() string {
	url := os.Getenv("RSPAMD_URL")
	if url == "" {
		return "http://127.0.0.1:11333" // Default Rspamd HTTP port
	}
	return strings.TrimRight(url, "/")
}

// ScanClamAV melakukan pemindaian stream email menggunakan protokol INSTREAM ClamAV clamd (RFC/ClamAV specification)
// Jika ClamAV tidak aktif atau tidak dapat dijangkau, scanning di-bypass secara graceful (fail-open)
func ScanClamAV(data []byte) (isInfected bool, virusName string, err error) {
	addr := GetClamAVAddress()

	network := "tcp"
	if strings.HasPrefix(addr, "/") {
		network = "unix"
	}

	conn, err := net.DialTimeout(network, addr, 3*time.Second)
	if err != nil {
		// ClamAV tidak terpasang/aktif, graceful bypass
		return false, "", nil
	}
	defer conn.Close()

	_ = conn.SetDeadline(time.Now().Add(10 * time.Second))

	// Kirim perintah nINSTREAM\n
	if _, err := conn.Write([]byte("zINSTREAM\x00")); err != nil {
		return false, "", err
	}

	// Kirim potongan data dengan prefix panjang 4-byte big-endian
	chunkSize := 2048
	for i := 0; i < len(data); i += chunkSize {
		end := i + chunkSize
		if end > len(data) {
			end = len(data)
		}
		chunk := data[i:end]
		lenBuf := make([]byte, 4)
		binary.BigEndian.PutUint32(lenBuf, uint32(len(chunk)))

		if _, err := conn.Write(lenBuf); err != nil {
			return false, "", err
		}
		if _, err := conn.Write(chunk); err != nil {
			return false, "", err
		}
	}

	// Kirim chunk kosong (panjang 0) sebagai tanda EOF
	zeroBuf := make([]byte, 4)
	binary.BigEndian.PutUint32(zeroBuf, 0)
	if _, err := conn.Write(zeroBuf); err != nil {
		return false, "", err
	}

	// Baca respons dari clamd
	reader := bufio.NewReader(conn)
	resp, err := reader.ReadString('\x00')
	if err != nil && !strings.Contains(err.Error(), "EOF") {
		return false, "", err
	}

	resp = strings.TrimSpace(strings.ReplaceAll(resp, "\x00", ""))
	// Format respons: "stream: OK" atau "stream: Eicar-Signature FOUND"
	if strings.Contains(resp, "FOUND") {
		parts := strings.Split(resp, " ")
		virus := "Malware/Virus"
		if len(parts) >= 2 {
			virus = parts[1]
		}
		return true, virus, nil
	}

	return false, "", nil
}

// CheckRspamd memeriksa email terhadap daemon Rspamd melalui HTTP API (/checkv2)
// Jika Rspamd tidak aktif, scanning di-bypass secara graceful (fail-open)
func CheckRspamd(data []byte) (isSpam bool, spamScore float64, err error) {
	rspamdURL := GetRspamdURL() + "/checkv2"

	req, err := http.NewRequest("POST", rspamdURL, bytes.NewReader(data))
	if err != nil {
		return false, 0, err
	}

	req.Header.Set("Content-Type", "message/rfc822")
	client := &http.Client{Timeout: 3 * time.Second}

	resp, err := client.Do(req)
	if err != nil {
		// Rspamd tidak aktif, graceful bypass
		return false, 0, nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return false, 0, nil
	}

	// Parsing respons minimalis Rspamd
	body, _ := io.ReadAll(resp.Body)
	bodyStr := string(body)

	// Jika Rspamd menandai action reject atau add_header
	if strings.Contains(bodyStr, `"action":"reject"`) || strings.Contains(bodyStr, `"action":"add header"`) || strings.Contains(bodyStr, `"is_spam":true`) {
		return true, 15.0, nil
	}

	return false, 0.0, nil
}

// ScanIncomingMessage melakukan pemindaian gabungan ClamAV dan Rspamd
func ScanIncomingMessage(rawBytes []byte) ScanResult {
	result := ScanResult{
		Action: "no_action",
	}

	// 1. Antivirus Scan
	if infected, virus, err := ScanClamAV(rawBytes); err == nil && infected {
		result.IsInfected = true
		result.VirusName = virus
		result.Action = "reject"
		return result
	}

	// 2. Anti-Spam Check
	if isSpam, score, err := CheckRspamd(rawBytes); err == nil && isSpam {
		result.IsSpam = true
		result.SpamScore = score
		result.Action = "rewrite_subject"
	}

	return result
}
