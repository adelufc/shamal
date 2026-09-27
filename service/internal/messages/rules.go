package messages

import (
	"path"
	"regexp"
	"strings"
	"unicode/utf8"
)

const MaxBody = 5000

const MaxAttachment = 10 << 20

var cardKeys = map[string]bool{
	"pan": true, "card_number": true, "cardnumber": true, "cvv": true, "cvc": true, "security_code": true,
}

var cardNumber = regexp.MustCompile(`\d{13,19}`)

func RefusesCard(fields map[string]string) bool {
	for key, value := range fields {
		if cardKeys[strings.ToLower(key)] || cardNumber.MatchString(value) {
			return true
		}
	}
	return false
}

func BodyCode(body string) string {
	count := utf8.RuneCountInString(strings.TrimSpace(body))
	if count == 0 {
		return "validation"
	}
	if count > MaxBody {
		return "body_too_long"
	}
	return ""
}

func AttachmentCode(contentType, name string, size int64) string {
	if size <= 0 {
		return "empty_file"
	}
	if size > MaxAttachment {
		return "file_too_large"
	}
	ext := strings.ToLower(path.Ext(name))
	switch ext {
	case ".exe", ".bat", ".cmd", ".com", ".scr", ".msi", ".dll", ".js", ".html", ".htm", ".svg":
		return "unsupported_type"
	}
	switch strings.ToLower(contentType) {
	case "image/jpeg", "image/png", "image/webp", "application/pdf":
		return ""
	default:
		return "unsupported_type"
	}
}

func SubjectKind(kind string) bool {
	switch kind {
	case "listing", "visit", "booking", "deal", "dispute":
		return true
	default:
		return false
	}
}
