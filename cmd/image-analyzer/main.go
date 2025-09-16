package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/menta2k/image-analyzer/internal/config"
	"github.com/menta2k/image-analyzer/internal/utils"
	"github.com/menta2k/image-analyzer/pkg/analyzer"
	"github.com/menta2k/image-analyzer/pkg/cropper"
)

var (
	version = "1.0.0"
	
	// Command line flags
	inputFlag      = flag.String("input", "", "Input image file or directory")
	outputFlag     = flag.String("output", "", "Output directory (default: ./output)")
	ratiosFlag     = flag.String("ratios", "", "Comma-separated aspect ratios (e.g., 1:1,4:3,16:9)")
	configFlag     = flag.String("config", "", "Configuration file path")
	qualityFlag    = flag.Int("quality", 85, "JPEG quality (1-100)")
	formatFlag     = flag.String("format", "", "Output format (jpg, png)")
	prefixFlag     = flag.String("prefix", "", "Output filename prefix")
	suffixFlag     = flag.String("suffix", "_cropped", "Output filename suffix")
	verboseFlag    = flag.Bool("verbose", false, "Verbose output")
	versionFlag    = flag.Bool("version", false, "Show version information")
	helpFlag       = flag.Bool("help", false, "Show help information")
	dryRunFlag     = flag.Bool("dry-run", false, "Show what would be done without actually processing")
	recursiveFlag  = flag.Bool("recursive", false, "Process directories recursively")
)

func main() {
	flag.Usage = showUsage
	flag.Parse()
	
	if *helpFlag {
		showUsage()
		return
	}
	
	if *versionFlag {
		showVersion()
		return
	}
	
	if *inputFlag == "" {
		fmt.Fprintf(os.Stderr, "Error: input file or directory is required\n\n")
		showUsage()
		os.Exit(1)
	}
	
	// Load configuration
	cfg, err := loadConfig()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}
	
	// Override config with command line flags
	applyFlagOverrides(cfg)
	
	// Validate configuration
	if err := cfg.Validate(); err != nil {
		log.Fatalf("Invalid configuration: %v", err)
	}
	
	// Process images
	if err := processImages(cfg); err != nil {
		log.Fatalf("Processing failed: %v", err)
	}
}

func loadConfig() (*config.Config, error) {
	if *configFlag != "" {
		return config.LoadFromFile(*configFlag)
	}
	
	// Try to load from default location
	defaultPath := config.GetConfigPath()
	if utils.FileExists(defaultPath) {
		return config.LoadFromFile(defaultPath)
	}
	
	// Use default configuration
	return config.Default(), nil
}

func applyFlagOverrides(cfg *config.Config) {
	if *outputFlag != "" {
		cfg.Output.OutputDir = *outputFlag
	}
	if *qualityFlag != 85 {
		cfg.Analyzer.DefaultQuality = *qualityFlag
	}
	if *formatFlag != "" {
		cfg.Output.DefaultFormat = *formatFlag
	}
	if *prefixFlag != "" {
		cfg.Output.Prefix = *prefixFlag
	}
	if *suffixFlag != "_cropped" {
		cfg.Output.Suffix = *suffixFlag
	}
}

func processImages(cfg *config.Config) error {
	// Ensure output directory exists
	if !*dryRunFlag {
		if err := utils.EnsureDir(cfg.Output.OutputDir); err != nil {
			return fmt.Errorf("failed to create output directory: %w", err)
		}
	}
	
	// Get list of files to process
	files, err := getInputFiles(*inputFlag)
	if err != nil {
		return fmt.Errorf("failed to get input files: %w", err)
	}
	
	if len(files) == 0 {
		return fmt.Errorf("no image files found")
	}
	
	if *verboseFlag {
		fmt.Printf("Found %d image files to process\n", len(files))
	}
	
	// Parse aspect ratios
	aspectRatios, err := parseAspectRatios(*ratiosFlag)
	if err != nil {
		return fmt.Errorf("failed to parse aspect ratios: %w", err)
	}
	
	// If no ratios specified, use common ones
	if len(aspectRatios) == 0 {
		aspectRatios = cropper.CommonAspectRatios()
	}
	
	// Initialize analyzer and cropper
	imageAnalyzer := analyzer.NewWithConfig(analyzer.Config{
		DefaultQuality:   cfg.Analyzer.DefaultQuality,
		SupportedFormats: cfg.Analyzer.SupportedFormats,
		MinImageSize:     cfg.Analyzer.MinImageSize,
	})
	
	smartCropper := cropper.NewWithConfig(cropper.CropConfig{
		PreserveAspectRatio: cfg.Cropper.PreserveAspectRatio,
		AllowUpscaling:      cfg.Cropper.AllowUpscaling,
		PaddingRatio:        cfg.Cropper.PaddingRatio,
		QualityThreshold:    cfg.Cropper.QualityThreshold,
	})
	
	// Process each file
	start := time.Now()
	processed := 0
	failed := 0
	
	for _, file := range files {
		if err := processFile(file, aspectRatios, imageAnalyzer, smartCropper, cfg); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to process %s: %v\n", file, err)
			failed++
		} else {
			processed++
		}
	}
	
	duration := time.Since(start)
	
	if *verboseFlag {
		fmt.Printf("\nCompleted in %v\n", duration)
		fmt.Printf("Processed: %d files\n", processed)
		if failed > 0 {
			fmt.Printf("Failed: %d files\n", failed)
		}
	}
	
	return nil
}

func getInputFiles(input string) ([]string, error) {
	info, err := os.Stat(input)
	if err != nil {
		return nil, err
	}
	
	if info.IsDir() {
		if *recursiveFlag {
			return utils.ListImageFiles(input)
		} else {
			// List only files in the directory (not subdirectories)
			entries, err := os.ReadDir(input)
			if err != nil {
				return nil, err
			}
			
			var files []string
			for _, entry := range entries {
				if !entry.IsDir() {
					fullPath := filepath.Join(input, entry.Name())
					if utils.IsImageFile(fullPath) {
						files = append(files, fullPath)
					}
				}
			}
			return files, nil
		}
	} else {
		if utils.IsImageFile(input) {
			return []string{input}, nil
		} else {
			return nil, fmt.Errorf("file is not a supported image format")
		}
	}
}

func processFile(filename string, aspectRatios []cropper.AspectRatio, 
	imageAnalyzer *analyzer.ImageAnalyzer, smartCropper *cropper.SmartCropper, 
	cfg *config.Config) error {
	
	if *verboseFlag {
		fmt.Printf("Processing: %s\n", filename)
	}
	
	if *dryRunFlag {
		fmt.Printf("Would process: %s\n", filename)
		return nil
	}
	
	// Load image
	img, err := imageAnalyzer.LoadImage(filename)
	if err != nil {
		return fmt.Errorf("failed to load image: %w", err)
	}
	
	// Validate image
	if err := imageAnalyzer.ValidateImage(img); err != nil {
		return fmt.Errorf("image validation failed: %w", err)
	}
	
	// Get image info
	info := imageAnalyzer.GetImageInfo(img)
	if *verboseFlag {
		fmt.Printf("  Image: %dx%d (ratio: %.2f)\n", info.Width, info.Height, info.AspectRatio)
	}
	
	// Process each aspect ratio
	for _, ratio := range aspectRatios {
		result, err := smartCropper.CropToAspectRatio(img, ratio)
		if err != nil {
			fmt.Fprintf(os.Stderr, "  Failed to crop to %s: %v\n", ratio.Name, err)
			continue
		}
		
		// Skip low-quality crops
		if result.Quality < cfg.Cropper.QualityThreshold {
			if *verboseFlag {
				fmt.Printf("  Skipping %s crop (quality: %.2f)\n", ratio.Name, result.Quality)
			}
			continue
		}
		
		// Generate output filename
		ratioSuffix := fmt.Sprintf("_%s", ratio.Name)
		if cfg.Output.Suffix != "" {
			ratioSuffix = cfg.Output.Suffix + ratioSuffix
		}
		
		outputFile := utils.GenerateOutputFilename(
			filename, 
			cfg.Output.OutputDir, 
			cfg.Output.Prefix,
			ratioSuffix,
			cfg.Output.DefaultFormat,
		)
		
		// Save cropped image
		if err := imageAnalyzer.SaveImage(result.Image, outputFile); err != nil {
			fmt.Fprintf(os.Stderr, "  Failed to save %s: %v\n", outputFile, err)
			continue
		}
		
		if *verboseFlag {
			fmt.Printf("  Saved %s (quality: %.2f)\n", filepath.Base(outputFile), result.Quality)
		}
	}
	
	return nil
}

func parseAspectRatios(ratioStr string) ([]cropper.AspectRatio, error) {
	if ratioStr == "" {
		return nil, nil
	}
	
	var ratios []cropper.AspectRatio
	parts := strings.Split(ratioStr, ",")
	
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		
		// Check if it's a named ratio
		commonRatios := cropper.CommonAspectRatios()
		found := false
		for _, common := range commonRatios {
			if strings.EqualFold(part, common.Name) {
				ratios = append(ratios, common)
				found = true
				break
			}
		}
		
		if found {
			continue
		}
		
		// Parse as width:height
		if strings.Contains(part, ":") {
			dimensions := strings.Split(part, ":")
			if len(dimensions) != 2 {
				return nil, fmt.Errorf("invalid aspect ratio format: %s", part)
			}
			
			width, err := strconv.Atoi(strings.TrimSpace(dimensions[0]))
			if err != nil {
				return nil, fmt.Errorf("invalid width in aspect ratio %s: %w", part, err)
			}
			
			height, err := strconv.Atoi(strings.TrimSpace(dimensions[1]))
			if err != nil {
				return nil, fmt.Errorf("invalid height in aspect ratio %s: %w", part, err)
			}
			
			ratios = append(ratios, cropper.AspectRatio{
				Width:  width,
				Height: height,
				Name:   fmt.Sprintf("%d_%d", width, height),
			})
		} else {
			return nil, fmt.Errorf("invalid aspect ratio format: %s (use width:height or ratio name)", part)
		}
	}
	
	return ratios, nil
}

func showUsage() {
	fmt.Printf("Image Analyzer v%s - Intelligent image analysis and cropping\n\n", version)
	fmt.Println("Usage: image-analyzer [options] -input <file|directory>")
	fmt.Println("\nOptions:")
	flag.PrintDefaults()
	fmt.Println("\nAspect Ratios:")
	fmt.Println("  Use ratio names: square, portrait, landscape, widescreen, instagram, story")
	fmt.Println("  Or specify custom ratios: 4:3,16:9,1:1")
	fmt.Println("\nExamples:")
	fmt.Println("  image-analyzer -input photo.jpg")
	fmt.Println("  image-analyzer -input ./photos -recursive -ratios square,instagram")
	fmt.Println("  image-analyzer -input image.png -ratios 4:3,16:9 -output ./crops")
}

func showVersion() {
	fmt.Printf("Image Analyzer v%s\n", version)
	fmt.Println("A Go module for intelligent image analysis and cropping using vision models")
}