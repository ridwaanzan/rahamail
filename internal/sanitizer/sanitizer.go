package sanitizer

import (
	"regexp"
	"strings"
)

var (
	// Regex untuk membuang tag berbahaya
	scriptTagRegex  = regexp.MustCompile(`(?i)<\s*script[^>]*>[\s\S]*?<\s*/\s*script\s*>`)
	styleTagRegex   = regexp.MustCompile(`(?i)<\s*style[^>]*>[\s\S]*?<\s*/\s*style\s*>`)
	iframeTagRegex  = regexp.MustCompile(`(?i)<\s*iframe[^>]*>[\s\S]*?<\s*/\s*iframe\s*>`)
	objectTagRegex  = regexp.MustCompile(`(?i)<\s*object[^>]*>[\s\S]*?<\s*/\s*object\s*>`)
	embedTagRegex   = regexp.MustCompile(`(?i)<\s*embed[^>]*>[\s\S]*?<\s*/\s*embed\s*>`)
	baseTagRegex    = regexp.MustCompile(`(?i)<\s*base[^>]*>`)
	metaTagRegex    = regexp.MustCompile(`(?i)<\s*meta[^>]*>`)
	linkTagRegex    = regexp.MustCompile(`(?i)<\s*link[^>]*>`)
	formTagRegex    = regexp.MustCompile(`(?i)<\s*form[^>]*>[\s\S]*?<\s*/\s*form\s*>`)

	// Regex untuk membuang event handler on* (onload, onerror, onclick, etc)
	onEventHandlerRegex = regexp.MustCompile(`(?i)\s*on[a-z]+\s*=\s*("[^"]*"|'[^']*'|[^\s>]+)`)

	// Regex untuk membuang javascript: dan vbscript: URI
	jsProtocolRegex = regexp.MustCompile(`(?i)\s*(href|src|action)\s*=\s*["']?\s*(javascript|vbscript|data\s*:\s*text/html):[^"'>\s]*["']?`)

	// Regex untuk membuang expression CSS
	cssExpressionRegex = regexp.MustCompile(`(?i)expression\s*\([^)]*\)*`)
)

// SanitizeHTML membersihkan string HTML dari berbagai vektor serangan Stored XSS
func SanitizeHTML(input string) string {
	if strings.TrimSpace(input) == "" {
		return ""
	}

	clean := input

	// 1. Buang tag berbahaya yang dapat mengeksekusi skrip atau memanipulasi DOM
	clean = scriptTagRegex.ReplaceAllString(clean, "")
	clean = iframeTagRegex.ReplaceAllString(clean, "")
	clean = objectTagRegex.ReplaceAllString(clean, "")
	clean = embedTagRegex.ReplaceAllString(clean, "")
	clean = formTagRegex.ReplaceAllString(clean, "")
	clean = baseTagRegex.ReplaceAllString(clean, "")
	clean = metaTagRegex.ReplaceAllString(clean, "")
	clean = linkTagRegex.ReplaceAllString(clean, "")

	// 2. Buang inline style tag yang berisiko membawa stylesheet eksternal atau font tracker
	clean = styleTagRegex.ReplaceAllString(clean, "")

	// 3. Buang seluruh inline event handler on* (onload, onerror, onmouseover, etc)
	clean = onEventHandlerRegex.ReplaceAllString(clean, "")

	// 4. Buang URI protokol javascript: dan data:text/html
	clean = jsProtocolRegex.ReplaceAllString(clean, " $1=\"#blocked\"")

	// 5. Buang CSS expression
	clean = cssExpressionRegex.ReplaceAllString(clean, "")

	return clean
}
