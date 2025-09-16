package cropper

import (
	"image"
	"image/color"
	"testing"
)

// createTestImage creates a simple test image
func createTestImage(width, height int) image.Image {
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	
	// Create a pattern with some high-contrast areas
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			if x > width/3 && x < 2*width/3 && y > height/3 && y < 2*height/3 {
				// Central bright region (subject)
				img.Set(x, y, color.RGBA{255, 255, 255, 255})
			} else {
				// Background
				img.Set(x, y, color.RGBA{64, 64, 64, 255})
			}
		}
	}
	
	return img
}

func TestNew(t *testing.T) {
	cropper := New()
	if cropper == nil {
		t.Error("New() returned nil")
	}
	
	if !cropper.config.PreserveAspectRatio {
		t.Error("Expected PreserveAspectRatio to be true by default")
	}
	
	if cropper.config.AllowUpscaling {
		t.Error("Expected AllowUpscaling to be false by default")
	}
}

func TestNewWithConfig(t *testing.T) {
	cfg := CropConfig{
		PreserveAspectRatio: false,
		AllowUpscaling:      true,
		PaddingRatio:        0.2,
		QualityThreshold:    0.5,
	}
	
	cropper := NewWithConfig(cfg)
	if cropper == nil {
		t.Error("NewWithConfig() returned nil")
	}
	
	if cropper.config.PreserveAspectRatio {
		t.Error("Expected PreserveAspectRatio to be false")
	}
	
	if !cropper.config.AllowUpscaling {
		t.Error("Expected AllowUpscaling to be true")
	}
}

func TestCommonAspectRatios(t *testing.T) {
	ratios := CommonAspectRatios()
	
	if len(ratios) == 0 {
		t.Error("Expected at least one common aspect ratio")
	}
	
	// Check that square ratio exists
	foundSquare := false
	for _, ratio := range ratios {
		if ratio.Name == "square" && ratio.Width == 1 && ratio.Height == 1 {
			foundSquare = true
			break
		}
	}
	
	if !foundSquare {
		t.Error("Expected to find square aspect ratio")
	}
}

func TestCropToAspectRatio(t *testing.T) {
	cropper := New()
	img := createTestImage(400, 300)
	
	// Test cropping to square
	result, err := cropper.CropToAspectRatio(img, Square)
	if err != nil {
		t.Fatalf("CropToAspectRatio failed: %v", err)
	}
	
	if result.Image == nil {
		t.Error("Expected cropped image to be non-nil")
	}
	
	// Check aspect ratio of result
	bounds := result.Image.Bounds()
	width, height := bounds.Dx(), bounds.Dy()
	ratio := float64(width) / float64(height)
	
	expectedRatio := float64(Square.Width) / float64(Square.Height)
	if ratio < expectedRatio-0.01 || ratio > expectedRatio+0.01 {
		t.Errorf("Expected aspect ratio %f, got %f", expectedRatio, ratio)
	}
	
	// Check that quality is reasonable
	if result.Quality < 0 || result.Quality > 1 {
		t.Errorf("Expected quality between 0 and 1, got %f", result.Quality)
	}
}

func TestCropToRatio(t *testing.T) {
	cropper := New()
	img := createTestImage(400, 300)
	
	// Test various ratios
	testRatios := []float64{1.0, 4.0/3.0, 16.0/9.0, 3.0/4.0}
	
	for _, targetRatio := range testRatios {
		result, err := cropper.CropToRatio(img, targetRatio)
		if err != nil {
			t.Fatalf("CropToRatio failed for ratio %f: %v", targetRatio, err)
		}
		
		bounds := result.Image.Bounds()
		width, height := bounds.Dx(), bounds.Dy()
		actualRatio := float64(width) / float64(height)
		
		if actualRatio < targetRatio-0.01 || actualRatio > targetRatio+0.01 {
			t.Errorf("Expected ratio %f, got %f", targetRatio, actualRatio)
		}
	}
}

func TestCropToMultipleRatios(t *testing.T) {
	cropper := New()
	img := createTestImage(400, 300)
	
	ratios := []AspectRatio{Square, Portrait, Landscape}
	results, err := cropper.CropToMultipleRatios(img, ratios)
	if err != nil {
		t.Fatalf("CropToMultipleRatios failed: %v", err)
	}
	
	if len(results) != len(ratios) {
		t.Errorf("Expected %d results, got %d", len(ratios), len(results))
	}
	
	for i, result := range results {
		if result.Image == nil {
			t.Errorf("Result %d has nil image", i)
		}
	}
}

func TestCropToSize(t *testing.T) {
	cropper := New()
	img := createTestImage(400, 300)
	
	// Test cropping to smaller size
	result, err := cropper.CropToSize(img, 200, 200)
	if err != nil {
		t.Fatalf("CropToSize failed: %v", err)
	}
	
	if result.Image == nil {
		t.Error("Expected cropped image to be non-nil")
	}
}

func TestCropToSizeUpscaling(t *testing.T) {
	// Test with upscaling disabled (default)
	cropper := New()
	img := createTestImage(200, 200)
	
	_, err := cropper.CropToSize(img, 400, 400)
	if err == nil {
		t.Error("Expected error when upscaling is disabled")
	}
	
	// Test with upscaling enabled
	cfg := CropConfig{
		AllowUpscaling:   true,
		QualityThreshold: 0.0,
	}
	cropperWithUpscaling := NewWithConfig(cfg)
	
	result, err := cropperWithUpscaling.CropToSize(img, 400, 400)
	if err != nil {
		t.Errorf("Expected no error with upscaling enabled: %v", err)
	}
	
	if result.Image == nil {
		t.Error("Expected result image to be non-nil")
	}
}

func TestGetOptimalCrops(t *testing.T) {
	cropper := New()
	img := createTestImage(400, 300)
	
	results, err := cropper.GetOptimalCrops(img)
	if err != nil {
		t.Fatalf("GetOptimalCrops failed: %v", err)
	}
	
	if len(results) == 0 {
		t.Error("Expected at least one optimal crop")
	}
	
	// Check that all results meet the quality threshold
	for name, result := range results {
		if result.Quality < cropper.config.QualityThreshold {
			t.Errorf("Crop %s has quality %f below threshold %f", 
				name, result.Quality, cropper.config.QualityThreshold)
		}
	}
}

func TestSmartResize(t *testing.T) {
	cropper := New()
	img := createTestImage(400, 300)
	
	// Test resizing to same aspect ratio
	resized, err := cropper.SmartResize(img, 200, 150)
	if err != nil {
		t.Fatalf("SmartResize failed: %v", err)
	}
	
	bounds := resized.Bounds()
	if bounds.Dx() != 200 || bounds.Dy() != 150 {
		t.Errorf("Expected 200x150, got %dx%d", bounds.Dx(), bounds.Dy())
	}
}

func TestCroppedImage(t *testing.T) {
	originalImg := createTestImage(200, 200)
	
	// Create a cropped image
	cropBounds := image.Rect(50, 50, 150, 150)
	croppedImg := &croppedImage{
		original: originalImg,
		bounds:   cropBounds,
	}
	
	// Test bounds
	bounds := croppedImg.Bounds()
	expectedWidth, expectedHeight := 100, 100
	if bounds.Dx() != expectedWidth || bounds.Dy() != expectedHeight {
		t.Errorf("Expected bounds %dx%d, got %dx%d", 
			expectedWidth, expectedHeight, bounds.Dx(), bounds.Dy())
	}
	
	// Test color model
	if croppedImg.ColorModel() != originalImg.ColorModel() {
		t.Error("Cropped image should have same color model as original")
	}
	
	// Test pixel access
	color1 := croppedImg.At(0, 0)
	color2 := originalImg.At(50, 50)
	
	if color1 != color2 {
		t.Error("Cropped image pixel should match original image pixel")
	}
}

func BenchmarkCropToRatio(b *testing.B) {
	cropper := New()
	img := createTestImage(1920, 1080)
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cropper.CropToRatio(img, 1.0)
	}
}

func BenchmarkCropToMultipleRatios(b *testing.B) {
	cropper := New()
	img := createTestImage(1920, 1080)
	ratios := CommonAspectRatios()
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cropper.CropToMultipleRatios(img, ratios)
	}
}