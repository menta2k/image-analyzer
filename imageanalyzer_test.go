package imageanalyzer

import (
	"image"
	"image/color"
	"testing"

	"github.com/menta2k/image-analyzer/pkg/analyzer"
	"github.com/menta2k/image-analyzer/pkg/cropper"
	"github.com/menta2k/image-analyzer/pkg/vision"
)

// createTestImage creates a simple test image
func createTestImage(width, height int) image.Image {
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	
	// Create a pattern with a bright subject in the center
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
	analyzer := New()
	if analyzer == nil {
		t.Error("New() returned nil")
	}
	
	if analyzer.analyzer == nil {
		t.Error("analyzer component is nil")
	}
	
	if analyzer.detector == nil {
		t.Error("detector component is nil")
	}
	
	if analyzer.cropper == nil {
		t.Error("cropper component is nil")
	}
}

func TestNewWithConfig(t *testing.T) {
	analyzerConfig := analyzer.Config{
		DefaultQuality:   95,
		SupportedFormats: []string{"png"},
		MinImageSize:     200,
	}
	
	visionConfig := vision.DetectionConfig{
		EdgeThreshold:   0.2,
		ContrastWeight:  0.4,
		ColorWeight:     0.3,
		SaliencyWeight:  0.6,
		MinSubjectRatio: 0.2,
	}
	
	cropperConfig := cropper.CropConfig{
		PreserveAspectRatio: false,
		AllowUpscaling:      true,
		PaddingRatio:        0.2,
		QualityThreshold:    0.5,
	}
	
	imageAnalyzer := NewWithConfig(analyzerConfig, visionConfig, cropperConfig)
	
	if imageAnalyzer == nil {
		t.Error("NewWithConfig() returned nil")
	}
	
	// Components should be initialized
	if imageAnalyzer.analyzer == nil {
		t.Error("analyzer component is nil")
	}
	
	if imageAnalyzer.detector == nil {
		t.Error("detector component is nil")
	}
	
	if imageAnalyzer.cropper == nil {
		t.Error("cropper component is nil")
	}
}

func TestAnalyzeImage(t *testing.T) {
	analyzer := New()
	img := createTestImage(400, 300)
	
	result, err := analyzer.AnalyzeImage(img)
	if err != nil {
		t.Fatalf("AnalyzeImage failed: %v", err)
	}
	
	// Check image info
	if result.Info.Width != 400 {
		t.Errorf("Expected width 400, got %d", result.Info.Width)
	}
	
	if result.Info.Height != 300 {
		t.Errorf("Expected height 300, got %d", result.Info.Height)
	}
	
	// Should have detected some subjects
	if len(result.Subjects) == 0 {
		t.Error("Expected to detect at least one subject")
	}
	
	// Should have generated some crops
	if len(result.Crops) == 0 {
		t.Error("Expected to generate at least one crop")
	}
}

func TestCropToAspectRatio(t *testing.T) {
	analyzer := New()
	img := createTestImage(400, 300)
	
	result, err := analyzer.CropToAspectRatio(img, cropper.Square)
	if err != nil {
		t.Fatalf("CropToAspectRatio failed: %v", err)
	}
	
	if result.Image == nil {
		t.Error("Expected cropped image to be non-nil")
	}
	
	// Check aspect ratio
	bounds := result.Image.Bounds()
	width, height := bounds.Dx(), bounds.Dy()
	ratio := float64(width) / float64(height)
	
	if ratio < 0.99 || ratio > 1.01 {
		t.Errorf("Expected square ratio (1.0), got %f", ratio)
	}
}

func TestCropToRatio(t *testing.T) {
	analyzer := New()
	img := createTestImage(400, 300)
	
	targetRatio := 16.0 / 9.0
	result, err := analyzer.CropToRatio(img, targetRatio)
	if err != nil {
		t.Fatalf("CropToRatio failed: %v", err)
	}
	
	if result.Image == nil {
		t.Error("Expected cropped image to be non-nil")
	}
	
	bounds := result.Image.Bounds()
	width, height := bounds.Dx(), bounds.Dy()
	actualRatio := float64(width) / float64(height)
	
	if actualRatio < targetRatio-0.01 || actualRatio > targetRatio+0.01 {
		t.Errorf("Expected ratio %f, got %f", targetRatio, actualRatio)
	}
}

func TestCropToMultipleRatios(t *testing.T) {
	analyzer := New()
	img := createTestImage(400, 300)
	
	ratios := []cropper.AspectRatio{
		cropper.Square,
		cropper.Portrait,
		cropper.Landscape,
	}
	
	results, err := analyzer.CropToMultipleRatios(img, ratios)
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

func TestDetectSubjects(t *testing.T) {
	analyzer := New()
	img := createTestImage(400, 300)
	
	subjects, err := analyzer.DetectSubjects(img)
	if err != nil {
		t.Fatalf("DetectSubjects failed: %v", err)
	}
	
	if len(subjects) == 0 {
		t.Error("Expected to detect at least one subject")
	}
	
	// Check subject properties
	for i, subject := range subjects {
		if subject.Width <= 0 || subject.Height <= 0 {
			t.Errorf("Subject %d has invalid dimensions: %dx%d", 
				i, subject.Width, subject.Height)
		}
		
		if subject.Score < 0 {
			t.Errorf("Subject %d has negative score: %f", i, subject.Score)
		}
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

func TestGetVersion(t *testing.T) {
	version := GetVersion()
	if version == "" {
		t.Error("Version should not be empty")
	}
	
	if version != Version {
		t.Errorf("GetVersion() returned %s, expected %s", version, Version)
	}
}

func TestGetBaseName(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"photo.jpg", "photo"},
		{"path/to/photo.jpg", "photo"},
		{"C:\\path\\to\\photo.png", "photo"},
		{"image", "image"},
		{"test.image.jpg", "test.image"},
	}
	
	for _, test := range tests {
		result := getBaseName(test.input)
		if result != test.expected {
			t.Errorf("getBaseName(%s) = %s, expected %s", 
				test.input, result, test.expected)
		}
	}
}

func BenchmarkAnalyzeImage(b *testing.B) {
	analyzer := New()
	img := createTestImage(400, 300)
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		analyzer.AnalyzeImage(img)
	}
}

func BenchmarkCropToAspectRatio(b *testing.B) {
	analyzer := New()
	img := createTestImage(1920, 1080)
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		analyzer.CropToAspectRatio(img, cropper.Square)
	}
}