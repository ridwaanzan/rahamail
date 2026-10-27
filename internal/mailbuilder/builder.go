package mailbuilder

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"mime"
	"net/mail"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// OutgoingEmail merepresentasikan struktur email yang akan dibuat dan dikirim
type OutgoingEmail struct {
	From        string
	To          []string
	Cc          []string
	Bcc         []string
	Subject     string
	BodyHTML    string
	BodyText    string
	Attachments []AttachmentInput
}

type AttachmentInput struct {
	FileName    string
	ContentType string
	Data        []byte
}

// BuildRFC5322Message menyusun string MIME email lengkap (MIME multipart/mixed dan multipart/alternative)
func BuildRFC5322Message(mailData OutgoingEmail) ([]byte, string, error) {
	messageID := fmt.Sprintf("<%d.%d@rahamail.local>", time.Now().UnixNano(), os.Getpid())
	dateStr := time.Now().Format(time.RFC1123Z)

	var buf bytes.Buffer

	// Headers
	buf.WriteString(fmt.Sprintf("From: %s\r\n", mailData.From))
	buf.WriteString(fmt.Sprintf("To: %s\r\n", strings.Join(mailData.To, ", ")))
	if len(mailData.Cc) > 0 {
		buf.WriteString(fmt.Sprintf("Cc: %s\r\n", strings.Join(mailData.Cc, ", ")))
	}
	buf.WriteString(fmt.Sprintf("Subject: %s\r\n", mime.QEncoding.Encode("utf-8", mailData.Subject)))
	buf.WriteString(fmt.Sprintf("Date: %s\r\n", dateStr))
	buf.WriteString(fmt.Sprintf("Message-ID: %s\r\n", messageID))
	buf.WriteString("MIME-Version: 1.0\r\n")

	hasAttachments := len(mailData.Attachments) > 0
	mixedBoundary := fmt.Sprintf("=_mixed_%d_%d", time.Now().UnixNano(), 1)
	altBoundary := fmt.Sprintf("=_alt_%d_%d", time.Now().UnixNano(), 2)

	if hasAttachments {
		buf.WriteString(fmt.Sprintf("Content-Type: multipart/mixed; boundary=\"%s\"\r\n\r\n", mixedBoundary))
		buf.WriteString(fmt.Sprintf("--%s\r\n", mixedBoundary))
	}

	// Body content: alternative jika ada HTML dan PlainText
	if mailData.BodyHTML != "" && mailData.BodyText != "" {
		buf.WriteString(fmt.Sprintf("Content-Type: multipart/alternative; boundary=\"%s\"\r\n\r\n", altBoundary))
		// Plain text part
		buf.WriteString(fmt.Sprintf("--%s\r\n", altBoundary))
		buf.WriteString("Content-Type: text/plain; charset=UTF-8\r\n")
		buf.WriteString("Content-Transfer-Encoding: base64\r\n\r\n")
		buf.WriteString(encodeBase64Lines([]byte(mailData.BodyText)))
		buf.WriteString("\r\n\r\n")

		// HTML part
		buf.WriteString(fmt.Sprintf("--%s\r\n", altBoundary))
		buf.WriteString("Content-Type: text/html; charset=UTF-8\r\n")
		buf.WriteString("Content-Transfer-Encoding: base64\r\n\r\n")
		buf.WriteString(encodeBase64Lines([]byte(mailData.BodyHTML)))
		buf.WriteString("\r\n\r\n")
		buf.WriteString(fmt.Sprintf("--%s--\r\n", altBoundary))
	} else if mailData.BodyHTML != "" {
		buf.WriteString("Content-Type: text/html; charset=UTF-8\r\n")
		buf.WriteString("Content-Transfer-Encoding: base64\r\n\r\n")
		buf.WriteString(encodeBase64Lines([]byte(mailData.BodyHTML)))
		buf.WriteString("\r\n\r\n")
	} else {
		buf.WriteString("Content-Type: text/plain; charset=UTF-8\r\n")
		buf.WriteString("Content-Transfer-Encoding: base64\r\n\r\n")
		buf.WriteString(encodeBase64Lines([]byte(mailData.BodyText)))
		buf.WriteString("\r\n\r\n")
	}

	// Lampiran jika ada
	if hasAttachments {
		for _, att := range mailData.Attachments {
			cType := att.ContentType
			if cType == "" {
				cType = "application/octet-stream"
			}
			buf.WriteString(fmt.Sprintf("\r\n--%s\r\n", mixedBoundary))
			buf.WriteString(fmt.Sprintf("Content-Type: %s; name=\"%s\"\r\n", cType, filepath.Base(att.FileName)))
			buf.WriteString("Content-Transfer-Encoding: base64\r\n")
			buf.WriteString(fmt.Sprintf("Content-Disposition: attachment; filename=\"%s\"\r\n\r\n", filepath.Base(att.FileName)))
			buf.WriteString(encodeBase64Lines(att.Data))
			buf.WriteString("\r\n")
		}
		buf.WriteString(fmt.Sprintf("\r\n--%s--\r\n", mixedBoundary))
	}

	return buf.Bytes(), messageID, nil
}

// encodeBase64Lines membagi base64 setiap 76 karakter sesuai RFC 2045
func encodeBase64Lines(data []byte) string {
	b64 := base64.StdEncoding.EncodeToString(data)
	var sb strings.Builder
	for len(b64) > 76 {
		sb.WriteString(b64[:76])
		sb.WriteString("\r\n")
		b64 = b64[76:]
	}
	sb.WriteString(b64)
	return sb.String()
}

// ParseAddresses mengurai string daftar email koma/spasi menjadi slice alamat bersih
func ParseAddresses(input string) []string {
	var result []string
	if strings.TrimSpace(input) == "" {
		return result
	}
	list, err := mail.ParseAddressList(input)
	if err == nil {
		for _, addr := range list {
			result = append(result, addr.Address)
		}
		return result
	}

	// Fallback split sederhana
	parts := strings.FieldsFunc(input, func(r rune) bool {
		return r == ',' || r == ';'
	})
	for _, p := range parts {
		clean := strings.TrimSpace(p)
		if clean != "" {
			result = append(result, clean)
		}
	}
	return result
}

// DecodeBase64 mendekode payload attachment base64
func DecodeBase64(input string) ([]byte, error) {
	commaIdx := strings.Index(input, ",")
	if commaIdx != -1 {
		input = input[commaIdx+1:]
	}
	return base64.StdEncoding.DecodeString(strings.TrimSpace(input))
}

