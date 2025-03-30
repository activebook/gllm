package service

import (
	"bytes"
	"image"
	_ "image/gif"  // Register GIF format
	_ "image/jpeg" // Register JPEG format
	_ "image/png"  // Register PNG format
	"os"
)

type ImageData struct {
	format string
	data   []byte
}

func NewImageData(format string, data []byte) *ImageData {
	return &ImageData{
		format: format,
		data:   data,
	}
}

func (i *ImageData) Format() string {
	return i.format
}
func (i *ImageData) Data() []byte {
	return i.data
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
	file, err := os.Open(filePath)
	if err != nil {
		return false, "", err
	}
	defer file.Close()

	_, format, err := image.DecodeConfig(file)
	if err != nil {
		return false, "", nil // Not an image or unsupported format
	}

	return true, format, nil
}

func CheckIfImageFromBytes(data []byte) (bool, string, error) {
	_, format, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return false, "", nil // Not an image or unsupported format
	}
	return true, format, nil
}
