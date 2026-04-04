package util

import (
	"encoding/base64"
	"fmt"
	"strings"
)

// ParseDataURL extracts the MIME type and base64-decoded raw bytes from a data URL string.
// Example: "data:image/png;base64,iVBORw0KGgo..." -> "image/png", []byte, nil
func ParseDataURL(dataURL string) (string, []byte, error) {
	if !strings.HasPrefix(dataURL, "data:") {
		return "", nil, fmt.Errorf("invalid data URL: missing 'data:' prefix")
	}

	// Remove "data:" prefix
	content := strings.TrimPrefix(dataURL, "data:")

	// Split into metadata and data at the first comma
	parts := strings.SplitN(content, ",", 2)
	if len(parts) != 2 {
		return "", nil, fmt.Errorf("invalid data URL: missing comma separator")
	}

	metadata := parts[0]
	rawData := parts[1]

	// Check if it's base64 encoded (which we expect for binary data)
	isBase64 := strings.HasSuffix(metadata, ";base64")
	var mimeType string

	if isBase64 {
		mimeType = strings.TrimSuffix(metadata, ";base64")
	} else {
		mimeType = metadata
	}

	// If no MIME type is specified, default to text/plain per RFC 2397
	if mimeType == "" {
		mimeType = "text/plain;charset=US-ASCII"
	}

	var dataBytes []byte
	var err error

	if isBase64 {
		dataBytes, err = base64.StdEncoding.DecodeString(rawData)
		if err != nil {
			return "", nil, fmt.Errorf("error decoding base64 data: %w", err)
		}
	} else {
		// If not base64, just use raw string bytes
		dataBytes = []byte(rawData)
	}

	return mimeType, dataBytes, nil
}

// BuildDataURL constructs a data URL string from MIME type and raw bytes.
// Example: "image/png", []byte -> "data:image/png;base64,iVBORw0KGgo..."
func BuildDataURL(mimeType string, data []byte) string {
	base64Encoded := base64.StdEncoding.EncodeToString(data)
	return fmt.Sprintf("data:%s;base64,%s", mimeType, base64Encoded)
}

// GetBase64String returns the base64 encoded string of the given data.
func GetBase64String(data []byte) string {
	base64Data := base64.StdEncoding.EncodeToString(data)
	return base64Data
}

// DecodeBase64String decodes a base64 encoded string to raw bytes.
func DecodeBase64String(data string) ([]byte, error) {
	return base64.StdEncoding.DecodeString(data)
}
