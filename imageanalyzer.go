// Package imageanalyzer provides intelligent image analysis and cropping functionality.
//
// This package combines computer vision techniques with smart cropping algorithms
// to automatically detect subjects in images and create optimally cropped versions
// for different aspect ratios.
//
// Basic usage:
//
//	package main
//
//	import (
//		"fmt"
//		"log"
//		"github.com/menta2k/image-analyzer/pkg/analyzer"
//		"github.com/menta2k/image-analyzer/pkg/cropper"
//	)
//
//	func main() {
//		// Initialize analyzer and cropper
//		imgAnalyzer := analyzer.New()
//		smartCropper := cropper.New()
//		
//		// Load and analyze image
//		img, err := imgAnalyzer.LoadImage("photo.jpg")
//		if err != nil {
//			log.Fatal(err)
//		}
//		
//		// Get image information
//		info := imgAnalyzer.GetImageInfo(img)
//		fmt.Printf("Image: %dx%d (ratio: %.2f)\n", info.Width, info.Height, info.AspectRatio)
//		
//		// Crop to square aspect ratio
//		result, err := smartCropper.CropToAspectRatio(img, cropper.Square)
//		if err != nil {
//			log.Fatal(err)
//		}
//		
//		// Save cropped image
//		if err := imgAnalyzer.SaveImage(result.Image, "photo_square.jpg"); err != nil {
//			log.Fatal(err)
//		}
//		
//		fmt.Printf("Cropped image saved with quality score: %.2f\n", result.Quality)
//	}
//
// The package consists of three main components:
//
// 1. Analyzer (pkg/analyzer): Handles image loading, saving, and basic analysis
// 2. Vision (pkg/vision): Provides subject detection and saliency analysis
// 3. Cropper (pkg/cropper): Implements intelligent cropping algorithms
//
// Features:
//
//   - Automatic subject detection using computer vision techniques
//   - Smart cropping that preserves important image regions
//   - Support for multiple aspect ratios (square, portrait, landscape, etc.)
//   - Quality scoring to evaluate crop effectiveness
//   - Configurable parameters for different use cases
//   - CLI tool for batch processing
//
// The vision system uses edge detection, contrast analysis, and saliency mapping
// to identify important regions in images. The cropping algorithm then finds the
// optimal crop position that maximizes the inclusion of these important regions
// while achieving the target aspect ratio.
package imageanalyzer

import (
	"fmt"
	"image"
	"io"

	"github.com/menta2k/image-analyzer/pkg/analyzer"
	"github.com/menta2k/image-analyzer/pkg/cropper"
	"github.com/menta2k/image-analyzer/pkg/vision"
)

// Version of the image analyzer library
const Version = "1.0.0"

// ImageAnalyzer provides a high-level interface for image analysis and cropping
type ImageAnalyzer struct {
	analyzer *analyzer.ImageAnalyzer
	detector *vision.SubjectDetector
	cropper  *cropper.SmartCropper
}

// New creates a new ImageAnalyzer with default configuration
func New() *ImageAnalyzer {
	return &ImageAnalyzer{
		analyzer: analyzer.New(),
		detector: vision.New(),
		cropper:  cropper.New(),
	}
}

// NewWithConfig creates a new ImageAnalyzer with custom configuration
func NewWithConfig(analyzerConfig analyzer.Config, visionConfig vision.DetectionConfig, cropperConfig cropper.CropConfig) *ImageAnalyzer {
	detector := vision.NewWithConfig(visionConfig)
	smartCropper := cropper.NewWithConfig(cropperConfig)
	smartCropper.SetDetector(detector)
	
	return &ImageAnalyzer{
		analyzer: analyzer.NewWithConfig(analyzerConfig),
		detector: detector,
		cropper:  smartCropper,
	}
}

// AnalysisResult contains comprehensive analysis results for an image
type AnalysisResult struct {
	Info     analyzer.ImageInfo `json:"info"`
	Subjects []vision.Region    `json:"subjects"`
	Crops    map[string]cropper.CropResult `json:"crops"`
}

// LoadImage loads an image from file
func (ia *ImageAnalyzer) LoadImage(filepath string) (image.Image, error) {
	return ia.analyzer.LoadImage(filepath)
}

// LoadImageFromReader loads an image from an io.Reader
func (ia *ImageAnalyzer) LoadImageFromReader(reader io.Reader) (image.Image, error) {
	return ia.analyzer.LoadImageFromReader(reader)
}

// SaveImage saves an image to file
func (ia *ImageAnalyzer) SaveImage(img image.Image, filepath string) error {
	return ia.analyzer.SaveImage(img, filepath)
}

// AnalyzeImage performs comprehensive analysis on an image
func (ia *ImageAnalyzer) AnalyzeImage(img image.Image) (AnalysisResult, error) {
	// Get basic image information
	info := ia.analyzer.GetImageInfo(img)
	
	// Detect subjects
	subjects, err := ia.detector.DetectSubjects(img)
	if err != nil {
		return AnalysisResult{}, fmt.Errorf("subject detection failed: %w", err)
	}
	
	// Get optimal crops for common aspect ratios
	crops, err := ia.cropper.GetOptimalCrops(img)
	if err != nil {
		return AnalysisResult{}, fmt.Errorf("crop analysis failed: %w", err)
	}
	
	return AnalysisResult{
		Info:     info,
		Subjects: subjects,
		Crops:    crops,
	}, nil
}

// CropToAspectRatio crops an image to a specific aspect ratio
func (ia *ImageAnalyzer) CropToAspectRatio(img image.Image, aspectRatio cropper.AspectRatio) (cropper.CropResult, error) {
	return ia.cropper.CropToAspectRatio(img, aspectRatio)
}

// CropToRatio crops an image to a specific aspect ratio (as float)
func (ia *ImageAnalyzer) CropToRatio(img image.Image, ratio float64) (cropper.CropResult, error) {
	return ia.cropper.CropToRatio(img, ratio)
}

// CropToMultipleRatios crops an image to multiple aspect ratios
func (ia *ImageAnalyzer) CropToMultipleRatios(img image.Image, ratios []cropper.AspectRatio) ([]cropper.CropResult, error) {
	return ia.cropper.CropToMultipleRatios(img, ratios)
}

// DetectSubjects detects subjects/regions of interest in an image
func (ia *ImageAnalyzer) DetectSubjects(img image.Image) ([]vision.Region, error) {
	return ia.detector.DetectSubjects(img)
}

// GetImageInfo returns basic information about an image
func (ia *ImageAnalyzer) GetImageInfo(img image.Image) analyzer.ImageInfo {
	return ia.analyzer.GetImageInfo(img)
}

// ValidateImage checks if an image meets requirements
func (ia *ImageAnalyzer) ValidateImage(img image.Image) error {
	return ia.analyzer.ValidateImage(img)
}

// ProcessImageFile is a convenience function that loads, analyzes, and crops an image
func (ia *ImageAnalyzer) ProcessImageFile(inputPath, outputDir string, ratios []cropper.AspectRatio) error {
	// Load image
	img, err := ia.LoadImage(inputPath)
	if err != nil {
		return fmt.Errorf("failed to load image: %w", err)
	}
	
	// Validate image
	if err := ia.ValidateImage(img); err != nil {
		return fmt.Errorf("image validation failed: %w", err)
	}
	
	// Crop to all specified ratios
	results, err := ia.CropToMultipleRatios(img, ratios)
	if err != nil {
		return fmt.Errorf("cropping failed: %w", err)
	}
	
	// Save cropped images
	for i, result := range results {
		outputPath := fmt.Sprintf("%s/%s_%s.jpg", outputDir, getBaseName(inputPath), ratios[i].Name)
		if err := ia.SaveImage(result.Image, outputPath); err != nil {
			return fmt.Errorf("failed to save crop %s: %w", ratios[i].Name, err)
		}
	}
	
	return nil
}

// GetVersion returns the library version
func GetVersion() string {
	return Version
}

// getBaseName extracts the base filename without extension
func getBaseName(filepath string) string {
	base := filepath
	for i := len(base) - 1; i >= 0; i-- {
		if base[i] == '/' || base[i] == '\\' {
			base = base[i+1:]
			break
		}
	}
	for i := len(base) - 1; i >= 0; i-- {
		if base[i] == '.' {
			base = base[:i]
			break
		}
	}
	return base
}