package vision

import (
	"image"
	"image/color"
	"math"
)

// SubjectDetector provides functionality to detect subjects/important regions in images
type SubjectDetector struct {
	config DetectionConfig
}

// DetectionConfig holds configuration for subject detection
type DetectionConfig struct {
	EdgeThreshold    float64
	ContrastWeight   float64
	ColorWeight      float64
	SaliencyWeight   float64
	MinSubjectRatio  float64
}

// New creates a new SubjectDetector with default configuration
func New() *SubjectDetector {
	return &SubjectDetector{
		config: DetectionConfig{
			EdgeThreshold:   0.01, // More sensitive
			ContrastWeight:  0.3,
			ColorWeight:     0.2,
			SaliencyWeight:  0.5,
			MinSubjectRatio: 0.05, // Smaller minimum
		},
	}
}

// NewWithConfig creates a new SubjectDetector with custom configuration
func NewWithConfig(config DetectionConfig) *SubjectDetector {
	return &SubjectDetector{config: config}
}

// Region represents a rectangular region of interest
type Region struct {
	X      int
	Y      int
	Width  int
	Height int
	Score  float64
}

// Center returns the center point of the region
func (r Region) Center() (int, int) {
	return r.X + r.Width/2, r.Y + r.Height/2
}

// Area returns the area of the region
func (r Region) Area() int {
	return r.Width * r.Height
}

// DetectSubjects analyzes an image and returns regions of interest
func (d *SubjectDetector) DetectSubjects(img image.Image) ([]Region, error) {
	bounds := img.Bounds()
	width, height := bounds.Dx(), bounds.Dy()
	
	// Create saliency map
	saliencyMap := d.calculateSaliencyMap(img)
	
	// Find regions with high saliency
	regions := d.findImportantRegions(saliencyMap, width, height)
	
	// Filter and score regions
	filteredRegions := d.filterAndScoreRegions(regions, width, height)
	
	// Limit to top regions to avoid too many results
	maxRegions := 10
	if len(filteredRegions) > maxRegions {
		filteredRegions = filteredRegions[:maxRegions]
	}
	
	return filteredRegions, nil
}

// FindBestCropRegion finds the optimal region for cropping to a specific aspect ratio
func (d *SubjectDetector) FindBestCropRegion(img image.Image, targetAspectRatio float64) (Region, error) {
	bounds := img.Bounds()
	width, height := bounds.Dx(), bounds.Dy()
	
	subjects, err := d.DetectSubjects(img)
	if err != nil {
		return Region{}, err
	}
	
	// Calculate optimal crop dimensions
	var cropWidth, cropHeight int
	currentRatio := float64(width) / float64(height)
	
	if targetAspectRatio > currentRatio {
		// Target is wider, constrain by width
		cropWidth = width
		cropHeight = int(float64(width) / targetAspectRatio)
	} else {
		// Target is taller, constrain by height
		cropHeight = height
		cropWidth = int(float64(height) * targetAspectRatio)
	}
	
	// Find best position that includes the most important subjects
	bestRegion := d.findOptimalCropPosition(subjects, cropWidth, cropHeight, width, height)
	
	return bestRegion, nil
}

func (d *SubjectDetector) calculateSaliencyMap(img image.Image) [][]float64 {
	bounds := img.Bounds()
	width, height := bounds.Dx(), bounds.Dy()
	
	saliencyMap := make([][]float64, height)
	for i := range saliencyMap {
		saliencyMap[i] = make([]float64, width)
	}
	
	// Simple saliency calculation based on edge detection and contrast
	for y := 1; y < height-1; y++ {
		for x := 1; x < width-1; x++ {
			// Get current pixel
			currentColor := img.At(x+bounds.Min.X, y+bounds.Min.Y)
			r1, g1, b1, _ := currentColor.RGBA()
			
			// Calculate edge strength using Sobel-like operator
			var edgeStrength float64
			
			// Check 8 neighboring pixels
			neighbors := [][]int{{-1, -1}, {-1, 0}, {-1, 1}, {0, -1}, {0, 1}, {1, -1}, {1, 0}, {1, 1}}
			
			for _, offset := range neighbors {
				nx, ny := x+offset[0], y+offset[1]
				neighborColor := img.At(nx+bounds.Min.X, ny+bounds.Min.Y)
				r2, g2, b2, _ := neighborColor.RGBA()
				
				// Calculate color difference
				dr := float64(r1) - float64(r2)
				dg := float64(g1) - float64(g2)
				db := float64(b1) - float64(b2)
				
				colorDiff := math.Sqrt(dr*dr + dg*dg + db*db)
				edgeStrength += colorDiff
			}
			
			// Normalize edge strength
			edgeStrength /= (8.0 * 65535.0) // 8 neighbors, max color value 65535
			
			// Calculate brightness for contrast
			brightness := (float64(r1) + float64(g1) + float64(b1)) / (3.0 * 65535.0)
			
			// Combine edge and contrast information
			saliency := d.config.ContrastWeight*edgeStrength + d.config.ColorWeight*brightness
			saliencyMap[y][x] = saliency
		}
	}
	
	return saliencyMap
}

func (d *SubjectDetector) findImportantRegions(saliencyMap [][]float64, width, height int) []Region {
	var regions []Region
	
	// Use sliding window approach to find high-saliency regions
	windowSizes := []int{width / 20, width / 16, width / 12, width / 8, width / 4} // Smaller windows too
	
	for _, windowSize := range windowSizes {
		if windowSize < 10 {
			continue // Skip very small windows
		}
		windowHeight := windowSize
		
		for y := 0; y <= height-windowHeight; y += windowSize / 8 { // Smaller steps
			for x := 0; x <= width-windowSize; x += windowSize / 8 {
				score := d.calculateRegionScore(saliencyMap, x, y, windowSize, windowHeight)
				
				if score > d.config.EdgeThreshold {
					regions = append(regions, Region{
						X:      x,
						Y:      y,
						Width:  windowSize,
						Height: windowHeight,
						Score:  score,
					})
				}
			}
		}
	}
	
	return regions
}

func (d *SubjectDetector) calculateRegionScore(saliencyMap [][]float64, x, y, width, height int) float64 {
	var totalScore float64
	count := 0
	
	for ry := y; ry < y+height && ry < len(saliencyMap); ry++ {
		for rx := x; rx < x+width && rx < len(saliencyMap[0]); rx++ {
			totalScore += saliencyMap[ry][rx]
			count++
		}
	}
	
	if count == 0 {
		return 0
	}
	
	return totalScore / float64(count)
}

func (d *SubjectDetector) filterAndScoreRegions(regions []Region, imageWidth, imageHeight int) []Region {
	var filtered []Region
	
	imageArea := imageWidth * imageHeight
	minArea := int(float64(imageArea) * d.config.MinSubjectRatio)
	
	for _, region := range regions {
		if region.Area() >= minArea {
			filtered = append(filtered, region)
		}
	}
	
	// Sort by score (descending)
	for i := 0; i < len(filtered)-1; i++ {
		for j := i + 1; j < len(filtered); j++ {
			if filtered[i].Score < filtered[j].Score {
				filtered[i], filtered[j] = filtered[j], filtered[i]
			}
		}
	}
	
	return filtered
}

func (d *SubjectDetector) findOptimalCropPosition(subjects []Region, cropWidth, cropHeight, imageWidth, imageHeight int) Region {
	bestScore := 0.0
	bestRegion := Region{
		X:      (imageWidth - cropWidth) / 2,
		Y:      (imageHeight - cropHeight) / 2,
		Width:  cropWidth,
		Height: cropHeight,
		Score:  0,
	}
	
	// Try different positions
	stepSize := int(math.Max(float64(cropWidth)/20, float64(cropHeight)/20))
	if stepSize < 10 {
		stepSize = 10
	}
	
	for y := 0; y <= imageHeight-cropHeight; y += stepSize {
		for x := 0; x <= imageWidth-cropWidth; x += stepSize {
			score := d.scorecropPosition(subjects, x, y, cropWidth, cropHeight)
			
			if score > bestScore {
				bestScore = score
				bestRegion = Region{
					X:      x,
					Y:      y,
					Width:  cropWidth,
					Height: cropHeight,
					Score:  score,
				}
			}
		}
	}
	
	return bestRegion
}

func (d *SubjectDetector) scorecropPosition(subjects []Region, cropX, cropY, cropWidth, cropHeight int) float64 {
	if len(subjects) == 0 {
		return 1.0 // Default score if no subjects detected
	}
	
	score := 0.0
	
	for _, subject := range subjects {
		// Calculate overlap between crop region and subject
		overlapX1 := int(math.Max(float64(cropX), float64(subject.X)))
		overlapY1 := int(math.Max(float64(cropY), float64(subject.Y)))
		overlapX2 := int(math.Min(float64(cropX+cropWidth), float64(subject.X+subject.Width)))
		overlapY2 := int(math.Min(float64(cropY+cropHeight), float64(subject.Y+subject.Height)))
		
		if overlapX2 > overlapX1 && overlapY2 > overlapY1 {
			overlapArea := (overlapX2 - overlapX1) * (overlapY2 - overlapY1)
			overlapRatio := float64(overlapArea) / float64(subject.Area())
			
			// Weight by subject importance (score)
			score += overlapRatio * subject.Score
		}
	}
	
	return score
}

// GetDominantColors extracts dominant colors from an image region
func (d *SubjectDetector) GetDominantColors(img image.Image, region Region) []color.Color {
	bounds := img.Bounds()
	
	// Color histogram
	colorMap := make(map[uint32]int)
	
	startX := int(math.Max(float64(region.X), float64(bounds.Min.X)))
	startY := int(math.Max(float64(region.Y), float64(bounds.Min.Y)))
	endX := int(math.Min(float64(region.X+region.Width), float64(bounds.Max.X)))
	endY := int(math.Min(float64(region.Y+region.Height), float64(bounds.Max.Y)))
	
	for y := startY; y < endY; y++ {
		for x := startX; x < endX; x++ {
			c := img.At(x, y)
			r, g, b, _ := c.RGBA()
			
			// Quantize colors to reduce noise
			r = (r >> 8) & 0xf0
			g = (g >> 8) & 0xf0
			b = (b >> 8) & 0xf0
			
			colorKey := (r << 16) | (g << 8) | b
			colorMap[colorKey]++
		}
	}
	
	// Find most frequent colors
	var colors []color.Color
	maxCount := 0
	
	for _, count := range colorMap {
		if count > maxCount {
			maxCount = count
		}
	}
	
	threshold := maxCount / 4 // Top 25% of colors
	
	for colorKey, count := range colorMap {
		if count >= threshold {
			r := uint8((colorKey >> 16) & 0xff)
			g := uint8((colorKey >> 8) & 0xff)
			b := uint8(colorKey & 0xff)
			colors = append(colors, color.RGBA{r, g, b, 255})
			
			if len(colors) >= 5 { // Limit to top 5 colors
				break
			}
		}
	}
	
	return colors
}