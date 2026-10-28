package mailreader

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"strings"

	"mail-server/internal/sanitizer"

	"github.com/emersion/go-message/mail"
)

// ParsedEmail merepresentasikan hasil parsing detail sebuah email
type ParsedEmail struct {
	Subject     string       `json:"subject"`
	From        string       `json:"from"`
	To          string       `json:"to"`
	Cc          string       `json:"cc"`
	Date        string       `json:"date"`
	BodyHTML    string       `json:"body_html"`
	BodyText    string       `json:"body_text"`
	Attachments []Attachment `json:"attachments"`
}

type Attachment struct {
	FileName    string `json:"file_name"`
	ContentType string `json:"content_type"`
	Size        int64  `json:"size"`
	PartIndex   int    `json:"part_index"`
}

// ParseEML membaca file .eml dari disk dan mengekstrak isi teks, HTML, dan attachments
func ParseEML(filePath string, fallbackSnippet string) (*ParsedEmail, error) {
	result := &ParsedEmail{
		Attachments: make([]Attachment, 0),
	}

	if filePath == "" {
		result.BodyText = fallbackSnippet
		return result, nil
	}

	data, err := os.ReadFile(filePath)
	if err != nil {
		result.BodyText = fallbackSnippet
		return result, nil
	}

	mr, err := mail.CreateReader(bytes.NewReader(data))
	if err != nil {
		result.BodyText = string(data)
		return result, nil
	}

	header := mr.Header
	if subj, err := header.Subject(); err == nil {
		result.Subject = subj
	}
	if fromList, err := header.AddressList("From"); err == nil && len(fromList) > 0 {
		result.From = fromList[0].String()
	}
	if toList, err := header.AddressList("To"); err == nil && len(toList) > 0 {
		var toStr []string
		for _, to := range toList {
			toStr = append(toStr, to.String())
		}
		result.To = strings.Join(toStr, ", ")
	}
	if ccList, err := header.AddressList("Cc"); err == nil && len(ccList) > 0 {
		var ccStr []string
		for _, cc := range ccList {
			ccStr = append(ccStr, cc.String())
		}
		result.Cc = strings.Join(ccStr, ", ")
	}
	if date, err := header.Date(); err == nil {
		result.Date = date.Format("02 Jan 2006 15:04:05 MST")
	}

	partIdx := 0
	for {
		p, err := mr.NextPart()
		if err != nil {
			break
		}
		partIdx++

		switch h := p.Header.(type) {
		case *mail.InlineHeader:
			contentType, _, _ := h.ContentType()
			b, _ := io.ReadAll(p.Body)
			if strings.HasPrefix(contentType, "text/html") && result.BodyHTML == "" {
				result.BodyHTML = sanitizer.SanitizeHTML(string(b))
			} else if strings.HasPrefix(contentType, "text/plain") && result.BodyText == "" {
				result.BodyText = string(b)
			}
		case *mail.AttachmentHeader:
			filename, _ := h.Filename()
			if filename == "" {
				filename = fmt.Sprintf("attachment_%d.bin", partIdx)
			}
			contentType, _, _ := h.ContentType()
			b, _ := io.ReadAll(p.Body)
			result.Attachments = append(result.Attachments, Attachment{
				FileName:    filename,
				ContentType: contentType,
				Size:        int64(len(b)),
				PartIndex:   partIdx,
			})
		}
	}

	// Fallback jika tidak ada body html/text
	if result.BodyHTML == "" && result.BodyText == "" {
		result.BodyText = fallbackSnippet
	} else if result.BodyHTML != "" {
		result.BodyHTML = sanitizer.SanitizeHTML(result.BodyHTML)
	}

	return result, nil
}
