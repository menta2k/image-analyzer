package analyzer

import (
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"io"
	"os"
	"strings"
)

// ImageAnalyzer provides intelligent image analysis and cropping capabilities
type ImageAnalyzer struct {
	config Config
}

// Config holds configuration for the image analyzer
type Config struct {
	DefaultQuality int
	SupportedFormats []string
	MinImageSize   int
}

// New creates a new ImageAnalyzer with default configuration
func New() *ImageAnalyzer {
	return &ImageAnalyzer{
		config: Config{
			DefaultQuality:   85,
			SupportedFormats: []string{"jpg", "jpeg", "png"},
			MinImageSize:     100,
		},
	}
}

// NewWithConfig creates a new ImageAnalyzer with custom configuration
func NewWithConfig(config Config) *ImageAnalyzer {
	return &ImageAnalyzer{config: config}
}

// LoadImage loads an image from file
func (a *ImageAnalyzer) LoadImage(filepath string) (image.Image, error) {
	file, err := os.Open(filepath)
	if err != nil {
		return nil, fmt.Errorf("failed to open image file: %w", err)
	}
	defer file.Close()

	img, format, err := image.Decode(file)
	if err != nil {
		return nil, fmt.Errorf("failed to decode image: %w", err)
	}

	if !a.isFormatSupported(format) {
		return nil, fmt.Errorf("unsupported image format: %s", format)
	}

	return img, nil
}

// LoadImageFromReader loads an image from an io.Reader
func (a *ImageAnalyzer) LoadImageFromReader(reader io.Reader) (image.Image, error) {
	img, format, err := image.Decode(reader)
	if err != nil {
		return nil, fmt.Errorf("failed to decode image: %w", err)
	}

	if !a.isFormatSupported(format) {
		return nil, fmt.Errorf("unsupported image format: %s", format)
	}

	return img, nil
}

// SaveImage saves an image to file
func (a *ImageAnalyzer) SaveImage(img image.Image, filepath string) error {
	file, err := os.Create(filepath)
	if err != nil {
		return fmt.Errorf("failed to create output file: %w", err)
	}
	defer file.Close()

	ext := strings.ToLower(filepath[strings.LastIndex(filepath, ".")+1:])
	
	switch ext {
	case "jpg", "jpeg":
		return jpeg.Encode(file, img, &jpeg.Options{Quality: a.config.DefaultQuality})
	case "png":
		return png.Encode(file, img)
	default:
		return fmt.Errorf("unsupported output format: %s", ext)
	}
}

// GetImageInfo returns basic information about an image
func (a *ImageAnalyzer) GetImageInfo(img image.Image) ImageInfo {
	bounds := img.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()
	
	return ImageInfo{
		Width:       width,
		Height:      height,
		AspectRatio: float64(width) / float64(height),
		Area:        width * height,
	}
}

// ImageInfo contains basic image metadata
type ImageInfo struct {
	Width       int
	Height      int
	AspectRatio float64
	Area        int
}

func (a *ImageAnalyzer) isFormatSupported(format string) bool {
	for _, supported := range a.config.SupportedFormats {
		if strings.EqualFold(format, supported) {
			return true
		}
	}
	return false
}

// ValidateImage checks if an image meets minimum requirements
func (a *ImageAnalyzer) ValidateImage(img image.Image) error {
	bounds := img.Bounds()
	if bounds.Dx() < a.config.MinImageSize || bounds.Dy() < a.config.MinImageSize {
		return fmt.Errorf("image too small: %dx%d (minimum: %d)", 
			bounds.Dx(), bounds.Dy(), a.config.MinImageSize)
	}
	return nil
}