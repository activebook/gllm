package service

import (
	"bytes"
	"image"
	_ "image/gif"  // Register GIF format
	_ "image/jpeg" // Register JPEG format
	_ "image/png"  // Register PNG format
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

type FileData struct {
	format string
	data   []byte
	path   string
}

func NewFileData(format string, data []byte, path string) *FileData {
	return &FileData{
		format: format,
		data:   data,
		path:   path,
	}
}

func (i *FileData) Format() string {
	return i.format
}
func (i *FileData) Data() []byte {
	return i.data
}

func (i *FileData) Path() string {
	return i.path
}

/*
The key differences between `image.Decode` and `image.DecodeConfig` are:

**image.Decode**
- Fully decodes the entire image including all pixel data
- Returns an `Image` interface, format name, and error
- More resource-intensive and slower
- Use when you need to process or manipulate the actual image content

```go
img, format, err := image.Decode(reader)
// img contains all pixel data and can be manipulated
```

**image.DecodeConfig**
- Only reads image headers/metadata (not pixel data)
- Returns a `Config` struct (with width, height, color model), format name, and error
- Much faster and more memory-efficient
- Perfect for just detecting if data is an image or getting dimensions

```go
config, format, err := image.DecodeConfig(reader)
// config.Width, config.Height available, but no pixel data
```
*/

// checkIfImage attempts to decode a file as an image
func CheckIfImageFromPath(filePath string) (bool, string, error) {
	mimeType := GetMIMEType(filePath)
	if IsImageMIMEType(mimeType) {
		return true, mimeType, nil
	}

	file, err := os.Open(filePath)
	if err != nil {
		return false, "", err
	}
	defer file.Close()

	_, format, err := image.DecodeConfig(file)
	if err != nil {
		return false, "", nil // Not an image or unsupported format
	}

	// Convert format to MIME type
	mimeType = "image/" + format
	return true, mimeType, nil
}

func CheckIfImageFromBytes(data []byte) (bool, string, error) {
	_, format, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return false, "", nil // Return the actual error
	}
	// Convert format to MIME type
	mimeType := "image/" + format
	return true, mimeType, nil
}

func GetMIMETypeByContent(data []byte) string {
	detectedType := http.DetectContentType(data)
	// if detectedType == "application/octet-stream" {
	// 	return "text/plain"
	// }
	return detectedType
}

func GetMIMEType(filePath string) string {
	// Ensure the extension starts with a dot
	ext := filepath.Ext(filePath)
	if len(ext) > 0 && ext[0] != '.' {
		ext = "." + ext
	}

	// Get MIME type from the standard mime package
	mimeType := mime.TypeByExtension(ext)
	if mimeType == "" {
		// Default MIME types for common extensions not in the mime package
		switch ext {
		case ".xlsx":
			return "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
		case ".xls":
			return "application/vnd.ms-excel"
		case ".pdf":
			return "application/pdf"
		case ".js":
			return "application/x-javascript"
		case ".py":
			return "application/x-python"
		case ".txt":
			return "text/plain"
		case ".html":
			return "text/html"
		case ".css":
			return "text/css"
		case ".md":
			return "text/md"
		case ".csv":
			return "text/csv"
		case ".xml":
			return "text/xml"
		case ".rtf":
			return "text/rtf"
		case ".mp3":
			return "audio/mp3"
		case ".wav":
			return "audio/wav"
		case ".aiff":
			return "audio/aiff"
		case ".acc":
			return "audio/acc"
		case ".ogg":
			return "audio/ogg"
		case ".flac":
			return "audio/flac"
		default:
			//return "text/plain" // Default txt

			// used for unknown binary files.
			return "application/octet-stream"
		}
	}
	return mimeType
}

func IsStdinPipe(source string) bool {
	return source == "-"
}

func IsPDFMIMEType(mimeType string) bool {
	return mimeType == "application/pdf"
}

func IsAudioMIMEType(mimeType string) bool {
	return mimeType == "audio/mp3" || mimeType == "audio/mpeg" || mimeType == "audio/ogg" ||
		mimeType == "audio/vnd.wave" || mimeType == "audio/x-wav" || mimeType == "audio/wav" ||
		mimeType == "audio/aiff" || mimeType == "audio/x-aiff" || mimeType == "audio/flac" || mimeType == "audio/x-flac" ||
		mimeType == "audio/aac" || mimeType == "audio/x-aac" || mimeType == "audio/x-ms-wma"
}

func IsExcelMIMEType(mimeType string) bool {
	return mimeType == "application/vnd.ms-excel" ||
		mimeType == "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
}

func IsImageMIMEType(mimeType string) bool {
	return mimeType == "image/jpeg" || mimeType == "image/png" || mimeType == "image/gif" ||
		mimeType == "image/webp" || mimeType == "image/heic" || mimeType == "image/heif"
}

func IsTextMIMEType(mimeType string) bool {
	if strings.HasPrefix(mimeType, "text/") {
		return true
	}
	switch mimeType {
	case "application/json",
		"application/xml",
		"application/javascript",
		"application/x-javascript",
		"application/x-python",
		"application/x-sh",
		"application/x-csh",
		"application/x-perl",
		"application/x-httpd-php",
		"application/x-ruby",
		"application/x-markdown",
		"text/md",
		"text/markdown":
		return true
	}
	// Exclude known binary types
	if IsImageMIMEType(mimeType) || IsPDFMIMEType(mimeType) || IsExcelMIMEType(mimeType) || IsAudioMIMEType(mimeType) {
		return false
	}
	return false
}

func IsUnknownMIMEType(mimeType string) bool {
	// used for unknown binary files.
	return mimeType == "application/octet-stream"
}
