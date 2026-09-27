package files

const (
	MaxImageBytes    int64 = 10 << 20
	MaxVideoBytes    int64 = 200 << 20
	MaxDocumentBytes int64 = 20 << 20
	MaxImages              = 30
)

func ValidateUpload(area, kind string, size int64, existingImages int) string {
	if area != "public" && area != "private" {
		return "unknown_storage"
	}
	if size <= 0 {
		return "empty_file"
	}
	switch kind {
	case "image":
		if size > MaxImageBytes {
			return "file_too_large"
		}
		if existingImages >= MaxImages {
			return "too_many_images"
		}
	case "video":
		if size > MaxVideoBytes {
			return "file_too_large"
		}
	case "document":
		if size > MaxDocumentBytes {
			return "file_too_large"
		}
	default:
		return "unsupported_type"
	}
	return ""
}

func KindFromType(contentType string) string {
	switch contentType {
	case "image/jpeg", "image/png", "image/webp":
		return "image"
	case "video/mp4":
		return "video"
	case "application/pdf":
		return "document"
	default:
		return ""
	}
}
