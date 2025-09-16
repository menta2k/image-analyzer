package cropper

import (
	"fmt"
	"image"
	"image/color"
	"math"

	"github.com/menta2k/image-analyzer/pkg/vision"
)

// SmartCropper provides intelligent cropping functionality
type SmartCropper struct {
	detector *vision.SubjectDetector
	config   CropConfig
}

// CropConfig holds configuration for smart cropping
type CropConfig struct {
	PreserveAspectRatio bool
	AllowUpscaling      bool
	PaddingRatio        float64
	QualityThreshold    float64
}

// AspectRatio represents common aspect ratios
type AspectRatio struct {
	Width  int
	Height int
	Name   string
}

// Common aspect ratios
var (
	Square    = AspectRatio{1, 1, "square"}
	Portrait  = AspectRatio{3, 4, "portrait"}
	Landscape = AspectRatio{4, 3, "landscape"}
	Widescreen = AspectRatio{16, 9, "widescreen"}
	Instagram = AspectRatio{4, 5, "instagram"}
	Story     = AspectRatio{9, 16, "story"}
)

// CommonAspectRatios returns a list of commonly used aspect ratios
func CommonAspectRatios() []AspectRatio {
	return []AspectRatio{Square, Portrait, Landscape, Widescreen, Instagram, Story}
}

// New creates a new SmartCropper with default configuration
func New() *SmartCropper {
	return &SmartCropper{
		detector: vision.New(),
		config: CropConfig{
			PreserveAspectRatio: true,
			AllowUpscaling:      false,
			PaddingRatio:        0.1,
			QualityThreshold:    0.7,
		},
	}
}

// NewWithConfig creates a new SmartCropper with custom configuration
func NewWithConfig(config CropConfig) *SmartCropper {
	return &SmartCropper{
		detector: vision.New(),
		config:   config,
	}
}

// SetDetector allows setting a custom subject detector
func (c *SmartCropper) SetDetector(detector *vision.SubjectDetector) {
	c.detector = detector
}

// CropResult contains the result of a cropping operation
type CropResult struct {
	Image       image.Image
	Region      vision.Region
	AspectRatio float64
	Quality     float64
}

// CropToAspectRatio crops an image to a specific aspect ratio while preserving important subjects
func (c *SmartCropper) CropToAspectRatio(img image.Image, aspectRatio AspectRatio) (CropResult, error) {
	targetRatio := float64(aspectRatio.Width) / float64(aspectRatio.Height)
	return c.CropToRatio(img, targetRatio)
}

// CropToRatio crops an image to a specific aspect ratio
func (c *SmartCropper) CropToRatio(img image.Image, targetRatio float64) (CropResult, error) {
	bounds := img.Bounds()
	originalWidth, originalHeight := bounds.Dx(), bounds.Dy()
	
	if originalWidth == 0 || originalHeight == 0 {
		return CropResult{}, fmt.Errorf("invalid image dimensions")
	}
	
	// Find the best crop region using subject detection
	cropRegion, err := c.detector.FindBestCropRegion(img, targetRatio)
	if err != nil {
		return CropResult{}, fmt.Errorf("failed to find optimal crop region: %w", err)
	}
	
	// Crop the image
	croppedImg := c.cropImageToRegion(img, cropRegion)
	
	// Calculate quality score
	quality := c.calculateCropQuality(img, cropRegion, targetRatio)
	
	return CropResult{
		Image:       croppedImg,
		Region:      cropRegion,
		AspectRatio: targetRatio,
		Quality:     quality,
	}, nil
}

// CropToMultipleRatios crops an image to multiple aspect ratios
func (c *SmartCropper) CropToMultipleRatios(img image.Image, ratios []AspectRatio) ([]CropResult, error) {
	var results []CropResult
	
	for _, ratio := range ratios {
		result, err := c.CropToAspectRatio(img, ratio)
		if err != nil {
			return nil, fmt.Errorf("failed to crop to %s: %w", ratio.Name, err)
		}
		results = append(results, result)
	}
	
	return results, nil
}

// CropToSize crops an image to specific dimensions
func (c *SmartCropper) CropToSize(img image.Image, targetWidth, targetHeight int) (CropResult, error) {
	bounds := img.Bounds()
	originalWidth, originalHeight := bounds.Dx(), bounds.Dy()
	
	if !c.config.AllowUpscaling {
		if targetWidth > originalWidth || targetHeight > originalHeight {
			return CropResult{}, fmt.Errorf("target size (%dx%d) is larger than original (%dx%d) and upscaling is disabled",
				targetWidth, targetHeight, originalWidth, originalHeight)
		}
	}
	
	targetRatio := float64(targetWidth) / float64(targetHeight)
	return c.CropToRatio(img, targetRatio)
}

// CropWithPadding crops an image with additional padding around subjects
func (c *SmartCropper) CropWithPadding(img image.Image, targetRatio float64, paddingRatio float64) (CropResult, error) {
	// Temporarily adjust padding for this operation
	originalPadding := c.config.PaddingRatio
	c.config.PaddingRatio = paddingRatio
	defer func() { c.config.PaddingRatio = originalPadding }()
	
	return c.CropToRatio(img, targetRatio)
}

// GetOptimalCrops suggests the best crops for common use cases
func (c *SmartCropper) GetOptimalCrops(img image.Image) (map[string]CropResult, error) {
	results := make(map[string]CropResult)
	commonRatios := CommonAspectRatios()
	
	for _, ratio := range commonRatios {
		result, err := c.CropToAspectRatio(img, ratio)
		if err == nil && result.Quality >= c.config.QualityThreshold {
			results[ratio.Name] = result
		}
	}
	
	return results, nil
}

func (c *SmartCropper) cropImageToRegion(img image.Image, region vision.Region) image.Image {
	bounds := img.Bounds()
	
	// Ensure crop region is within image bounds
	x1 := int(math.Max(float64(region.X), float64(bounds.Min.X)))
	y1 := int(math.Max(float64(region.Y), float64(bounds.Min.Y)))
	x2 := int(math.Min(float64(region.X+region.Width), float64(bounds.Max.X)))
	y2 := int(math.Min(float64(region.Y+region.Height), float64(bounds.Max.Y)))
	
	// Create crop rectangle
	cropRect := image.Rect(x1, y1, x2, y2)
	
	// Create new image with cropped bounds
	return &croppedImage{
		original: img,
		bounds:   cropRect,
	}
}

func (c *SmartCropper) calculateCropQuality(img image.Image, region vision.Region, targetRatio float64) float64 {
	bounds := img.Bounds()
	originalWidth, originalHeight := bounds.Dx(), bounds.Dy()
	
	// Quality factors
	var quality float64
	
	// 1. How much of the original image is preserved
	originalArea := originalWidth * originalHeight
	cropArea := region.Width * region.Height
	preservationRatio := float64(cropArea) / float64(originalArea)
	
	// 2. How close is the crop ratio to the target ratio
	cropRatio := float64(region.Width) / float64(region.Height)
	ratioAccuracy := 1.0 - math.Abs(cropRatio-targetRatio)/math.Max(cropRatio, targetRatio)
	
	// 3. Subject preservation (from region score)
	subjectScore := region.Score
	
	// 4. Centering score (how well-centered the crop is)
	centerX := originalWidth / 2
	centerY := originalHeight / 2
	cropCenterX := region.X + region.Width/2
	cropCenterY := region.Y + region.Height/2
	
	maxDistance := math.Sqrt(float64(originalWidth*originalWidth + originalHeight*originalHeight))
	distance := math.Sqrt(float64((centerX-cropCenterX)*(centerX-cropCenterX) + (centerY-cropCenterY)*(centerY-cropCenterY)))
	centeringScore := 1.0 - (distance / maxDistance)
	
	// Combine scores with weights
	quality = 0.3*preservationRatio + 0.3*ratioAccuracy + 0.3*subjectScore + 0.1*centeringScore
	
	// Ensure quality is between 0 and 1
	if quality > 1.0 {
		quality = 1.0
	}
	if quality < 0.0 {
		quality = 0.0
	}
	
	return quality
}

// croppedImage implements the image.Image interface for cropped images
type croppedImage struct {
	original image.Image
	bounds   image.Rectangle
}

func (c *croppedImage) ColorModel() color.Model {
	return c.original.ColorModel()
}

func (c *croppedImage) Bounds() image.Rectangle {
	return image.Rect(0, 0, c.bounds.Dx(), c.bounds.Dy())
}

func (c *croppedImage) At(x, y int) color.Color {
	pt := image.Point{x, y}
	if !pt.In(c.Bounds()) {
		return color.RGBA{}
	}
	return c.original.At(x+c.bounds.Min.X, y+c.bounds.Min.Y)
}

// SmartResize resizes an image while maintaining quality and subject focus
func (c *SmartCropper) SmartResize(img image.Image, targetWidth, targetHeight int) (image.Image, error) {
	bounds := img.Bounds()
	originalWidth, originalHeight := bounds.Dx(), bounds.Dy()
	
	if originalWidth == targetWidth && originalHeight == targetHeight {
		return img, nil
	}
	
	targetRatio := float64(targetWidth) / float64(targetHeight)
	originalRatio := float64(originalWidth) / float64(originalHeight)
	
	// If ratios are very close, just resize
	if math.Abs(targetRatio-originalRatio) < 0.01 {
		return c.simpleResize(img, targetWidth, targetHeight), nil
	}
	
	// Otherwise, smart crop first, then resize
	cropResult, err := c.CropToRatio(img, targetRatio)
	if err != nil {
		return nil, err
	}
	
	// Then resize the cropped image to exact dimensions
	return c.simpleResize(cropResult.Image, targetWidth, targetHeight), nil
}

func (c *SmartCropper) simpleResize(img image.Image, targetWidth, targetHeight int) image.Image {
	bounds := img.Bounds()
	originalWidth, originalHeight := bounds.Dx(), bounds.Dy()
	
	// Simple nearest neighbor resize for now
	// In a production environment, you'd want to use a proper image resizing library
	return &resizedImage{
		original:     img,
		targetWidth:  targetWidth,
		targetHeight: targetHeight,
		scaleX:       float64(originalWidth) / float64(targetWidth),
		scaleY:       float64(originalHeight) / float64(targetHeight),
	}
}

// resizedImage implements the image.Image interface for resized images
type resizedImage struct {
	original     image.Image
	targetWidth  int
	targetHeight int
	scaleX       float64
	scaleY       float64
}

func (r *resizedImage) ColorModel() color.Model {
	return r.original.ColorModel()
}

func (r *resizedImage) Bounds() image.Rectangle {
	return image.Rect(0, 0, r.targetWidth, r.targetHeight)
}

func (r *resizedImage) At(x, y int) color.Color {
	pt := image.Point{x, y}
	if !pt.In(r.Bounds()) {
		return color.RGBA{}
	}
	
	// Map target coordinates to original coordinates
	origX := int(float64(x) * r.scaleX)
	origY := int(float64(y) * r.scaleY)
	
	bounds := r.original.Bounds()
	if origX >= bounds.Max.X {
		origX = bounds.Max.X - 1
	}
	if origY >= bounds.Max.Y {
		origY = bounds.Max.Y - 1
	}
	
	return r.original.At(origX+bounds.Min.X, origY+bounds.Min.Y)
}