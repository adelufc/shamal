package messages

import "testing"

func TestMessageRules(t *testing.T) {
	if BodyCode("  ") != "validation" || BodyCode(stringsRepeat(5001)) != "body_too_long" || BodyCode("مرحبا") != "" {
		t.Fatal("body limits")
	}
	if AttachmentCode("image/jpeg", "a.jpg", 10) != "" {
		t.Fatal("jpeg")
	}
	if AttachmentCode("application/pdf", "a.pdf", MaxAttachment+1) != "file_too_large" {
		t.Fatal("size")
	}
	if AttachmentCode("application/octet-stream", "run.exe", 10) != "unsupported_type" {
		t.Fatal("executable")
	}
	if AttachmentCode("image/svg+xml", "a.svg", 10) != "unsupported_type" {
		t.Fatal("svg")
	}
	if !RefusesCard(map[string]string{"note": "4111111111111111"}) || RefusesCard(map[string]string{"note": "مرحبا"}) {
		t.Fatal("card")
	}
	if SubjectKind("listing") != true || SubjectKind("invoice") != false {
		t.Fatal("subject")
	}
}

func stringsRepeat(n int) string {
	return string(make([]rune, n))
}
