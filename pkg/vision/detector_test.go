package vision

import (
	"image"
	"image/color"
	"testing"
)

// createTestImage creates a simple test image with some patterns
func createTestImage(width, height int) image.Image {
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	
	// Create a simple pattern with high contrast areas (simulate subjects)
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			// Create a high contrast square in the center-left
			if x > width/4 && x < width/2 && y > height/4 && y < 3*height/4 {
				img.Set(x, y, color.RGBA{255, 255, 255, 255}) // White square
			} else if x > 3*width/4 && y > height/4 && y < 3*height/4 {
				img.Set(x, y, color.RGBA{0, 0, 0, 255}) // Black square
			} else {
				// Background gradient
				r := uint8((x * 128) / width)
				g := uint8((y * 128) / height)
				img.Set(x, y, color.RGBA{r, g, 64, 255})
			}
		}
	}
	
	return img
}

func TestNew(t *testing.T) {
	detector := New()
	if detector == nil {
		t.Error("New() returned nil")
	}
	
	if detector.config.EdgeThreshold != 0.01 {
		t.Errorf("Expected edge threshold 0.01, got %f", detector.config.EdgeThreshold)
	}
}

func TestNewWithConfig(t *testing.T) {
	cfg := DetectionConfig{
		EdgeThreshold:   0.2,
		ContrastWeight:  0.4,
		ColorWeight:     0.3,
		SaliencyWeight:  0.6,
		MinSubjectRatio: 0.2,
	}
	
	detector := NewWithConfig(cfg)
	if detector == nil {
		t.Error("NewWithConfig() returned nil")
	}
	
	if detector.config.EdgeThreshold != 0.2 {
		t.Errorf("Expected edge threshold 0.2, got %f", detector.config.EdgeThreshold)
	}
}

func TestRegionCenter(t *testing.T) {
	region := Region{X: 10, Y: 20, Width: 100, Height: 80}
	
	centerX, centerY := region.Center()
	
	expectedX := 10 + 100/2 // 60
	expectedY := 20 + 80/2  // 60
	
	if centerX != expectedX {
		t.Errorf("Expected center X %d, got %d", expectedX, centerX)
	}
	
	if centerY != expectedY {
		t.Errorf("Expected center Y %d, got %d", expectedY, centerY)
	}
}

func TestRegionArea(t *testing.T) {
	region := Region{X: 10, Y: 20, Width: 100, Height: 80}
	
	area := region.Area()
	expected := 100 * 80
	
	if area != expected {
		t.Errorf("Expected area %d, got %d", expected, area)
	}
}

func TestDetectSubjects(t *testing.T) {
	detector := New()
	img := createTestImage(400, 300)
	
	regions, err := detector.DetectSubjects(img)
	if err != nil {
		t.Fatalf("DetectSubjects failed: %v", err)
	}
	
	if len(regions) == 0 {
		t.Error("Expected to detect at least one region")
	}
	
	// Check that regions have valid properties
	for i, region := range regions {
		if region.Width <= 0 || region.Height <= 0 {
			t.Errorf("Region %d has invalid dimensions: %dx%d", i, region.Width, region.Height)
		}
		
		if region.Score < 0 {
			t.Errorf("Region %d has negative score: %f", i, region.Score)
		}
	}
}

func TestFindBestCropRegion(t *testing.T) {
	detector := New()
	img := createTestImage(400, 300)
	
	// Test square crop
	region, err := detector.FindBestCropRegion(img, 1.0)
	if err != nil {
		t.Fatalf("FindBestCropRegion failed: %v", err)
	}
	
	// Check that the region is valid
	if region.Width <= 0 || region.Height <= 0 {
		t.Errorf("Invalid crop region dimensions: %dx%d", region.Width, region.Height)
	}
	
	// Check that the crop region fits within the image
	if region.X < 0 || region.Y < 0 {
		t.Errorf("Crop region starts outside image bounds: (%d, %d)", region.X, region.Y)
	}
	
	if region.X+region.Width > 400 || region.Y+region.Height > 300 {
		t.Errorf("Crop region extends outside image bounds")
	}
	
	// For a square crop (1:1), width should equal height
	ratio := float64(region.Width) / float64(region.Height)
	if ratio < 0.95 || ratio > 1.05 { // Allow small tolerance
		t.Errorf("Square crop should have 1:1 aspect ratio, got %f", ratio)
	}
}

func TestGetDominantColors(t *testing.T) {
	detector := New()
	img := createTestImage(200, 200)
	
	// Test on the white square region
	region := Region{X: 50, Y: 50, Width: 50, Height: 100}
	colors := detector.GetDominantColors(img, region)
	
	if len(colors) == 0 {
		t.Error("Expected to find at least one dominant color")
	}
	
	// Should find white as one of the dominant colors
	foundWhite := false
	for _, c := range colors {
		r, g, b, _ := c.RGBA()
		// Check if it's close to white (allowing for quantization)
		if r > 0xf000 && g > 0xf000 && b > 0xf000 {
			foundWhite = true
			break
		}
	}
	
	if !foundWhite {
		t.Error("Expected to find white as a dominant color in the white square region")
	}
}

func TestCalculateSaliencyMap(t *testing.T) {
	detector := New()
	img := createTestImage(100, 100)
	
	saliencyMap := detector.calculateSaliencyMap(img)
	
	// Check dimensions
	if len(saliencyMap) != 100 {
		t.Errorf("Expected saliency map height 100, got %d", len(saliencyMap))
	}
	
	if len(saliencyMap[0]) != 100 {
		t.Errorf("Expected saliency map width 100, got %d", len(saliencyMap[0]))
	}
	
	// Check that there are some non-zero values (edges detected)
	hasNonZero := false
	for y := 1; y < 99; y++ {
		for x := 1; x < 99; x++ {
			if saliencyMap[y][x] > 0 {
				hasNonZero = true
				break
			}
		}
		if hasNonZero {
			break
		}
	}
	
	if !hasNonZero {
		t.Error("Expected saliency map to have some non-zero values")
	}
}

func BenchmarkDetectSubjects(b *testing.B) {
	detector := New()
	img := createTestImage(400, 300)
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		detector.DetectSubjects(img)
	}
}

func BenchmarkFindBestCropRegion(b *testing.B) {
	detector := New()
	img := createTestImage(400, 300)
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		detector.FindBestCropRegion(img, 1.0)
	}
}