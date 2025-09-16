package analyzer

import (
	"image"
	"image/color"
	"testing"
)

// createTestImage creates a simple test image
func createTestImage(width, height int) image.Image {
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	
	// Fill with a gradient pattern
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			r := uint8((x * 255) / width)
			g := uint8((y * 255) / height)
			b := uint8(128)
			img.Set(x, y, color.RGBA{r, g, b, 255})
		}
	}
	
	return img
}

func TestNew(t *testing.T) {
	analyzer := New()
	if analyzer == nil {
		t.Error("New() returned nil")
	}
	
	if analyzer.config.DefaultQuality != 85 {
		t.Errorf("Expected default quality 85, got %d", analyzer.config.DefaultQuality)
	}
}

func TestNewWithConfig(t *testing.T) {
	cfg := Config{
		DefaultQuality:   95,
		SupportedFormats: []string{"png"},
		MinImageSize:     200,
	}
	
	analyzer := NewWithConfig(cfg)
	if analyzer == nil {
		t.Error("NewWithConfig() returned nil")
	}
	
	if analyzer.config.DefaultQuality != 95 {
		t.Errorf("Expected quality 95, got %d", analyzer.config.DefaultQuality)
	}
	
	if analyzer.config.MinImageSize != 200 {
		t.Errorf("Expected min size 200, got %d", analyzer.config.MinImageSize)
	}
}

func TestGetImageInfo(t *testing.T) {
	analyzer := New()
	img := createTestImage(400, 300)
	
	info := analyzer.GetImageInfo(img)
	
	if info.Width != 400 {
		t.Errorf("Expected width 400, got %d", info.Width)
	}
	
	if info.Height != 300 {
		t.Errorf("Expected height 300, got %d", info.Height)
	}
	
	expectedRatio := float64(400) / float64(300)
	if info.AspectRatio != expectedRatio {
		t.Errorf("Expected aspect ratio %f, got %f", expectedRatio, info.AspectRatio)
	}
	
	if info.Area != 120000 {
		t.Errorf("Expected area 120000, got %d", info.Area)
	}
}

func TestValidateImage(t *testing.T) {
	analyzer := New()
	
	// Valid image
	validImg := createTestImage(200, 200)
	if err := analyzer.ValidateImage(validImg); err != nil {
		t.Errorf("Valid image should pass validation: %v", err)
	}
	
	// Invalid image (too small)
	invalidImg := createTestImage(50, 50)
	if err := analyzer.ValidateImage(invalidImg); err == nil {
		t.Error("Small image should fail validation")
	}
}

func TestIsFormatSupported(t *testing.T) {
	analyzer := New()
	
	supportedFormats := []string{"jpg", "jpeg", "png", "JPG", "JPEG", "PNG"}
	for _, format := range supportedFormats {
		if !analyzer.isFormatSupported(format) {
			t.Errorf("Format %s should be supported", format)
		}
	}
	
	unsupportedFormats := []string{"gif", "bmp", "tiff"}
	for _, format := range unsupportedFormats {
		if analyzer.isFormatSupported(format) {
			t.Errorf("Format %s should not be supported", format)
		}
	}
}

func BenchmarkGetImageInfo(b *testing.B) {
	analyzer := New()
	img := createTestImage(1920, 1080)
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		analyzer.GetImageInfo(img)
	}
}

func BenchmarkValidateImage(b *testing.B) {
	analyzer := New()
	img := createTestImage(1920, 1080)
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		analyzer.ValidateImage(img)
	}
}