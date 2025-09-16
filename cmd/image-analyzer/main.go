package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/menta2k/image-analyzer/pkg/client"
	"github.com/menta2k/image-analyzer/pkg/detection"
	"github.com/menta2k/image-analyzer/pkg/llamacpp"
	"github.com/menta2k/image-analyzer/pkg/ollama"
	"github.com/menta2k/image-analyzer/pkg/processing"
	"github.com/menta2k/image-analyzer/pkg/types"
)

// Default target sizes for cropping
var defaultTargetSizes = [][2]int{
	{1200, 675},
	{1200, 800},
	{400, 250},
	{600, 400},
	{1200, 630},
}

func main() {
	var in, outDir, model, url, ext string
	var backend string
	var quality int
	var lossless bool
	var sendFmt string
	var sendSize int
	var sendQ int
	var zoom float64
	var debug bool

	// Debug overlay format (separate from crop ext)
	var dbgext string
	var dbgquality int
	var dbglossless bool

	flag.StringVar(&in, "in", "", "input image path or URL (jpg/png/webp)")
	flag.StringVar(&outDir, "out", "out", "output directory")
	flag.StringVar(&model, "model", "openbmb/minicpm-v4.5", "model name")
	flag.StringVar(&backend, "backend", "llamacpp", "backend to use: ollama or llamacpp")
	flag.StringVar(&url, "url", "", "server URL (defaults: ollama=http://localhost:11435/api/chat, llamacpp=http://localhost:8080)")

	flag.StringVar(&ext, "ext", "jpg", "output format for crops: jpg|png|webp")
	flag.IntVar(&quality, "quality", 90, "JPEG/WebP output quality for crops (1-100)")
	flag.BoolVar(&lossless, "lossless", false, "WebP output lossless mode for crops")

	flag.StringVar(&dbgext, "dbgext", "png", "debug overlay format: png|jpg|webp")
	flag.IntVar(&dbgquality, "dbgquality", 92, "debug overlay quality (for jpg/webp)")
	flag.BoolVar(&dbglossless, "dbglossless", false, "debug overlay WebP lossless mode")

	flag.StringVar(&sendFmt, "sendfmt", "jpg", "format sent to Ollama: jpg|png")
	flag.IntVar(&sendSize, "sendsize", 1536, "max long side sent to Ollama (px), 0=original")
	flag.IntVar(&sendQ, "sendq", 85, "JPEG quality for image sent to Ollama (1-100)")

	flag.Float64Var(&zoom, "zoom", 1.0, "shrink factor for crop size (0.01..1.0)")
	flag.BoolVar(&debug, "debug", false, "create debug overlay images")

	flag.Parse()
	if in == "" {
		log.Fatalf("usage: %s -in input.jpg|URL [-backend ollama|llamacpp] [-url server_url] [-out outdir] [-ext jpg|png|webp] [-zoom 0.95] [-sendfmt jpg|png]", filepath.Base(os.Args[0]))
	}
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		log.Fatal(err)
	}

	// Initialize components
	processor := processing.NewProcessor()

	// Create appropriate client based on backend
	var visionClient client.VisionClient
	var err error

	switch backend {
	case "ollama":
		if url == "" {
			url = "http://localhost:11435/api/chat"
		}
		visionClient, err = ollama.NewClient(url)
		if err != nil {
			log.Fatalf("Failed to create Ollama client: %v", err)
		}
	case "llamacpp":
		if url == "" {
			url = "http://localhost:8080"
		}
		visionClient, err = llamacpp.NewClient(url)
		if err != nil {
			log.Fatalf("Failed to create llama.cpp client: %v", err)
		}
	default:
		log.Fatalf("Unknown backend: %s (use 'ollama' or 'llamacpp')\n", backend)
	}

	detector := detection.NewDetector(visionClient)

	// Load input image (from file or URL)
	img, err := processor.LoadImageSmart(in)
	if err != nil {
		log.Fatal(err)
	}
	bounds := img.Bounds()
	imgW, imgH := bounds.Dx(), bounds.Dy()

	// Prepare image for model
	imgB64, err := processor.PrepareImageForModel(img, sendFmt, sendSize, sendQ)
	if err != nil {
		log.Fatal(err)
	}

	// Detect subject in image
	result, err := detector.DetectSubject(context.Background(), model, imgB64)
	if err != nil {
		log.Fatal(err)
	}

	// Find the nearest point to center within the detected box
	cx, cy := processor.FindNearestPointToCenter(result.Primary.Box)

	log.Printf("primary=%q conf=%.2f modelBox=%.3fx%.3f@%.3f,%.3f  -> crop center=%.3f,%.3f",
		result.Primary.Label, result.Primary.Confidence, result.Primary.Box.W, result.Primary.Box.H,
		result.Primary.Box.X, result.Primary.Box.Y, cx, cy)
	log.Printf("description: %s", result.Description)
	log.Printf("tags: %v", result.Tags)

	// Create debug overlay for original image (if debug enabled)
	if debug {
		baseOverlay := processor.CreateDebugOverlay(img, result.Primary.Box, types.Box{X: 0, Y: 0, W: 0, H: 0}, cx, cy)
		baseDbgPath := filepath.Join(outDir, fmt.Sprintf("000_original_with_box.%s", strings.ToLower(dbgext)))
		if err := processor.SaveImage(baseOverlay, baseDbgPath, dbgext, dbgquality, dbglossless); err != nil {
			log.Printf("debug overlay save failed: %v", err)
		} else {
			log.Printf("wrote %s", baseDbgPath)
		}
	}

	// Process each target size
	seen := map[string]int{}
	for i, sz := range defaultTargetSizes {
		w, h := sz[0], sz[1]
		key := fmt.Sprintf("%dx%d", w, h)
		seen[key]++
		variant := "A"
		if seen[key] > 1 {
			variant = "B"
		}

		// Calculate optimal crop box
		cropBox := processor.CalculateOptimalCropBox(cx, cy, w, h, imgW, imgH, zoom)

		// Crop and save the image
		croppedImg, err := processor.CropImageToBox(img, cropBox, w, h)
		if err != nil {
			log.Printf("crop %s failed: %v", key, err)
			continue
		}

		cropPath := filepath.Join(outDir, fmt.Sprintf("%03d_%s_%s.%s", i+1, key, variant, strings.ToLower(ext)))
		if err := processor.SaveImage(croppedImg, cropPath, ext, quality, lossless); err != nil {
			log.Printf("save %s failed: %v", cropPath, err)
		} else {
			log.Printf("wrote %s", cropPath)
		}

		// Create debug overlay for this crop (if debug enabled)
		if debug {
			dbg := processor.CreateDebugOverlay(img, result.Primary.Box, cropBox, cx, cy)
			dbgPath := filepath.Join(outDir, fmt.Sprintf("%03d_debug_%s_%s.%s", i+1, key, variant, strings.ToLower(dbgext)))
			if err := processor.SaveImage(dbg, dbgPath, dbgext, dbgquality, dbglossless); err != nil {
				log.Printf("debug save %s failed: %v", dbgPath, err)
			} else {
				log.Printf("wrote %s", dbgPath)
			}
		}
	}

	// Save raw model JSON output
	js, _ := json.MarshalIndent(result, "", "  ")
	_ = os.WriteFile(filepath.Join(outDir, "model_output.json"), js, 0o644)
}
