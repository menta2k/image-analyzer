package main

import (
	"fmt"
	"image"
	"image/color"
	"log"

	imageanalyzer "github.com/menta2k/image-analyzer"
	"github.com/menta2k/image-analyzer/pkg/cropper"
)

// createSampleImage creates a sample image for demonstration
func createSampleImage() image.Image {
	width, height := 800, 600
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	
	// Create a background gradient
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			r := uint8((x * 128) / width + 64)
			g := uint8((y * 128) / height + 64)
			b := uint8(128)
			img.Set(x, y, color.RGBA{r, g, b, 255})
		}
	}
	
	// Add a bright subject in the upper left
	for y := height/4; y < height/2; y++ {
		for x := width/6; x < width/2; x++ {
			img.Set(x, y, color.RGBA{255, 255, 255, 255})
		}
	}
	
	// Add a dark subject in the lower right
	for y := 2*height/3; y < 5*height/6; y++ {
		for x := 2*width/3; x < 5*width/6; x++ {
			img.Set(x, y, color.RGBA{0, 0, 0, 255})
		}
	}
	
	return img
}

func main() {
	fmt.Println("Image Analyzer Example")
	fmt.Println("====================")
	
	// Create analyzer
	analyzer := imageanalyzer.New()
	
	// Create a sample image
	img := createSampleImage()
	fmt.Println("Created sample image: 800x600")
	
	// Get basic image info
	info := analyzer.GetImageInfo(img)
	fmt.Printf("Image info: %dx%d (ratio: %.2f, area: %d)\n", 
		info.Width, info.Height, info.AspectRatio, info.Area)
	
	// Perform comprehensive analysis
	analysis, err := analyzer.AnalyzeImage(img)
	if err != nil {
		log.Fatalf("Analysis failed: %v", err)
	}
	
	fmt.Printf("Detected %d subjects:\n", len(analysis.Subjects))
	for i, subject := range analysis.Subjects {
		centerX, centerY := subject.Center()
		fmt.Printf("  Subject %d: %dx%d at (%d,%d) center=(%d,%d) score=%.3f\n",
			i+1, subject.Width, subject.Height, subject.X, subject.Y, 
			centerX, centerY, subject.Score)
	}
	
	fmt.Printf("\nGenerated %d optimal crops:\n", len(analysis.Crops))
	for name, crop := range analysis.Crops {
		bounds := crop.Image.Bounds()
		fmt.Printf("  %s: %dx%d quality=%.3f\n", 
			name, bounds.Dx(), bounds.Dy(), crop.Quality)
	}
	
	// Test specific aspect ratios
	fmt.Println("\nTesting specific crops:")
	
	ratios := []cropper.AspectRatio{
		cropper.Square,
		cropper.Portrait,
		cropper.Landscape,
		cropper.Widescreen,
	}
	
	for _, ratio := range ratios {
		result, err := analyzer.CropToAspectRatio(img, ratio)
		if err != nil {
			fmt.Printf("  %s: Failed - %v\n", ratio.Name, err)
			continue
		}
		
		bounds := result.Image.Bounds()
		actualRatio := float64(bounds.Dx()) / float64(bounds.Dy())
		fmt.Printf("  %s (%d:%d): %dx%d ratio=%.3f quality=%.3f\n",
			ratio.Name, ratio.Width, ratio.Height,
			bounds.Dx(), bounds.Dy(), actualRatio, result.Quality)
	}
	
	// Test custom ratio
	fmt.Println("\nTesting custom ratio:")
	customRatio := cropper.AspectRatio{
		Width:  21,
		Height: 9,
		Name:   "ultrawide",
	}
	
	result, err := analyzer.CropToAspectRatio(img, customRatio)
	if err != nil {
		fmt.Printf("  Custom ratio failed: %v\n", err)
	} else {
		bounds := result.Image.Bounds()
		actualRatio := float64(bounds.Dx()) / float64(bounds.Dy())
		fmt.Printf("  %s (%d:%d): %dx%d ratio=%.3f quality=%.3f\n",
			customRatio.Name, customRatio.Width, customRatio.Height,
			bounds.Dx(), bounds.Dy(), actualRatio, result.Quality)
	}
	
	// Save sample crop (if we had a real filesystem)
	fmt.Println("\nExample complete! In a real application, you would save the cropped images:")
	fmt.Println("  analyzer.SaveImage(result.Image, \"crop_square.jpg\")")
	
	fmt.Printf("\nLibrary version: %s\n", imageanalyzer.GetVersion())
}