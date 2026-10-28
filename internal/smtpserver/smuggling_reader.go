package smtpserver

import (
	"errors"
	"io"
)

var (
	ErrNakedLFDetected = errors.New("554 5.6.0 SMTP Smuggling detected: naked LF without CR is not permitted (RFC 5321)")
)

// SmugglingReader membungkus io.Reader untuk mendeteksi 'naked LF' (karakter \n yang tidak didahului oleh \r)
// Ini mencegah eksploitasi SMTP Smuggling (CVE-2023-51764 & CVE-2023-51765)
type SmugglingReader struct {
	r      io.Reader
	lastCh byte
}

func NewSmugglingReader(r io.Reader) *SmugglingReader {
	return &SmugglingReader{r: r}
}

func (sr *SmugglingReader) Read(p []byte) (n int, err error) {
	n, err = sr.r.Read(p)
	if n <= 0 {
		return n, err
	}

	for i := 0; i < n; i++ {
		ch := p[i]
		if ch == '\n' {
			// Jika \n muncul tanpa \r di depannya (baik dari buffer sebelumnya atau byte sebelum ini)
			if sr.lastCh != '\r' {
				return 0, ErrNakedLFDetected
			}
		}
		sr.lastCh = ch
	}

	return n, err
}
