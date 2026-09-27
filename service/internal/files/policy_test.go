package files

import (
	"strings"
	"testing"
)

func TestSafeJoinRejectsTraversal(t *testing.T) {
	if _, err := SafeJoin(".data", "public", "../private"); err == nil {
		t.Fatal("path traversal accepted")
	}
	path, err := SafeJoin(".data/objects", "private", "abc.pdf")
	if err != nil || !strings.Contains(path, "private") {
		t.Fatal(path, err)
	}
}

func TestSeparateAreasAndLimits(t *testing.T) {
	if ValidateUpload("archive", "image", 10, 0) != "unknown_storage" {
		t.Fatal("area")
	}
	if ValidateUpload("public", "image", MaxImageBytes+1, 0) != "file_too_large" {
		t.Fatal("image size")
	}
	if ValidateUpload("private", "document", MaxDocumentBytes, 0) != "" {
		t.Fatal("document should fit")
	}
	if ValidateUpload("public", "image", 100, MaxImages) != "too_many_images" {
		t.Fatal("image count")
	}
	if KindFromType("image/jpeg") != "image" || KindFromType("text/html") != "" {
		t.Fatal("content type")
	}
}
