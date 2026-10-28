package sanitizer

import (
	"strings"
	"testing"
)

func TestSanitizeHTML(t *testing.T) {
	testCases := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "Script tag elimination",
			input:    "<p>Halo</p><script>alert('XSS')</script><b>Dunia</b>",
			expected: "<p>Halo</p><b>Dunia</b>",
		},
		{
			name:     "Inline onerror event handler",
			input:    `<img src="invalid.jpg" onerror="alert(document.cookie)">`,
			expected: `<img src="invalid.jpg">`,
		},
		{
			name:     "Javascript protocol href",
			input:    `<a href="javascript:alert(1)">Klik Disini</a>`,
			expected: `<a href="#blocked">Klik Disini</a>`,
		},
		{
			name:     "Iframe tag removal",
			input:    `<div>Sebelum</div><iframe src="http://evil.com"></iframe><div>Sesudah</div>`,
			expected: `<div>Sebelum</div><div>Sesudah</div>`,
		},
		{
			name:     "CSS expression removal",
			input:    `<div style="width: expression(alert(1)); color: red;">Test</div>`,
			expected: `<div style="width: ; color: red;">Test</div>`,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			output := SanitizeHTML(tc.input)
			if strings.TrimSpace(output) != strings.TrimSpace(tc.expected) {
				t.Errorf("Expected: %q, Got: %q", tc.expected, output)
			}
		})
	}
}
