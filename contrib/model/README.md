# Image Analyzer

A Go module for intelligent image analysis and cropping using Ollama vision models. This tool detects subjects in images and creates optimally cropped versions for different aspect ratios.

## Architecture

The module is organized into separate packages for maximum flexibility:

- **`pkg/ollama`** - Ollama API client wrapper
- **`pkg/detection`** - Vision-based subject detection
- **`pkg/processing`** - Image processing and cropping operations
- **`pkg/types`** - Shared data structures
- **`cmd/image-analyzer`** - CLI application
- **`example/`** - Example usage demonstrating the API

## Features

- **Subject Detection**: Uses Ollama vision models to detect and locate primary subjects
- **Smart Cropping**: Creates optimal crops centered on detected subjects
- **Multiple Formats**: Supports JPG, PNG, and WebP input/output
- **URL Support**: Load images directly from HTTP/HTTPS URLs
- **Debug Overlays**: Visual debugging with bounding boxes and crop indicators
- **Flexible API**: Separate detection and processing for custom workflows

## Installation

```bash
go get github.com/sko/image-analyzer
```

## Requirements

- Go 1.19+
- Running Ollama instance with a vision model (e.g., `openbmb/minicpm-v4.5`, `minicpm-v`, `llava`)

## CLI Usage

```bash
# Build the CLI tool
go build -o image-analyzer cmd/image-analyzer/main.go

# Basic usage with local file
./image-analyzer -in photo.jpg -out crops/ -model openbmb/minicpm-v4.5

# Using a URL
./image-analyzer -in "https://picsum.photos/1200/800" -out crops/ -model openbmb/minicpm-v4.5

# Advanced options with URL
./image-analyzer \
  -in "https://picsum.photos/1200/800" \
  -out crops/ \
  -model minicpm-v \
  -ext webp \
  -quality 95 \
  -zoom 0.9 \
  -debug \
  -sendfmt png \
  -sendsize 2048

# Wikipedia images work too (includes User-Agent header)
./image-analyzer -in "https://upload.wikimedia.org/wikipedia/commons/6/66/Mayotte-mamoudzou-1800x1000-d259bf1d.jpg" -out crops/ -model minicpm-v
```

### CLI Options

- `-in`: Input image path or URL (required)
- `-out`: Output directory (default: "out")
- `-model`: Ollama model name (default: "openbmb/minicpm-v4.5")
- `-url`: Ollama API endpoint (default: "http://localhost:11434/api/chat")
- `-ext`: Output format: jpg|png|webp (default: "jpg")
- `-quality`: JPEG/WebP quality 1-100 (default: 90)
- `-zoom`: Crop zoom factor 0.01-1.0 (default: 1.0)
- `-debug`: Create debug overlay images (default: false)
- `-sendfmt`: Format sent to model: jpg|png (default: "jpg")
- `-sendsize`: Max dimension sent to model (default: 1536)

## API Usage

### Basic Example

```go
package main

import (
    "context"
    "log"

    "ollama-image-analyzer/pkg/detection"
    "ollama-image-analyzer/pkg/ollama"
    "ollama-image-analyzer/pkg/processing"
)

func main() {
    // Initialize components
    processor := processing.NewProcessor()
    client, _ := ollama.NewClient("http://localhost:11434/api/chat")
    detector := detection.NewDetector(client)

    // Load image (file or URL)
    img, _ := processor.LoadImageSmart("photo.jpg")
    // or from URL:
    // img, _ := processor.LoadImageSmart("https://picsum.photos/800/600")

    // Prepare for vision model
    imgB64, _ := processor.PrepareImageForModel(img, "jpg", 1536, 85)

    // Detect subject
    result, _ := detector.DetectSubject(context.Background(), "minicpm-v", imgB64)

    // Find optimal crop center
    cx, cy := processor.FindNearestPointToCenter(result.Primary.Box)

    // Create crop
    cropBox := processor.CalculateOptimalCropBox(cx, cy, 800, 600,
        img.Bounds().Dx(), img.Bounds().Dy(), 1.0)
    croppedImg, _ := processor.CropImageToBox(img, cropBox, 800, 600)

    // Save result
    processor.SaveImage(croppedImg, "output.jpg", "jpg", 90, false)
}
```

### Advanced Workflow

```go
// Custom detection prompt
customPrompt := `Find faces in this image and return the main face location...`
result, _ := detector.DetectSubjectWithPrompt(ctx, "minicpm-v", imgB64, customPrompt)

// Multiple crops with different aspect ratios
configs := []struct{ w, h int }{
    {1200, 675}, // 16:9 landscape
    {600, 800},  // 3:4 portrait
    {400, 400},  // 1:1 square
}

for _, cfg := range configs {
    cropBox := processor.CalculateOptimalCropBox(cx, cy, cfg.w, cfg.h, imgW, imgH, 0.9)
    cropped, _ := processor.CropImageToBox(img, cropBox, cfg.w, cfg.h)
    processor.SaveImage(cropped, fmt.Sprintf("crop_%dx%d.jpg", cfg.w, cfg.h), "jpg", 90, false)
}

// Debug visualization
debugImg := processor.CreateDebugOverlay(img, result.Primary.Box, cropBox, cx, cy)
processor.SaveImage(debugImg, "debug.png", "png", 92, false)
```

## Package APIs

### `pkg/types`

Core data structures:
- `Box` - Normalized bounding box (coordinates in [0,1])
- `Primary` - Detected subject with confidence and location
- `AnalysisResult` - Complete detection result with tags and description

### `pkg/ollama`

Ollama client wrapper:
- `NewClient(url)` - Create client
- `AnalyzeImage(ctx, model, prompt, imageB64)` - Analyze image with vision model

### `pkg/detection`

Subject detection:
- `NewDetector(client)` - Create detector
- `DetectSubject(ctx, model, imageB64)` - Detect with default prompt
- `DetectSubjectWithPrompt(ctx, model, imageB64, prompt)` - Custom prompt

### `pkg/processing`

Image operations:
- `LoadImage(path)` - Load image from file path with WebP support
- `LoadImageFromURL(url)` - Download and load image from URL
- `LoadImageSmart(source)` - Load from file path or URL automatically
- `PrepareImageForModel(img, fmt, maxDim, quality)` - Encode for vision model
- `CropImageToBox(img, box, w, h)` - Crop to normalized box
- `CalculateOptimalCropBox(cx, cy, w, h, imgW, imgH, zoom)` - Calculate crop
- `FindNearestPointToCenter(box)` - Find center point in box
- `SaveImage(img, path, fmt, quality, lossless)` - Save with format options
- `CreateDebugOverlay(img, modelBox, cropBox, cx, cy)` - Debug visualization

## Default Crop Sizes

The CLI generates these crops by default:
- 1200×675 (16:9 landscape)
- 1200×800 (3:2 landscape)
- 400×250 (8:5 small)
- 600×400 (3:2 medium, variants A & B)
- 1200×630 (social media)
- 1200×675 (variant B)

## Output Files

The CLI creates:
- `001_1200x675_A.jpg` - Cropped versions (always created)
- `model_output.json` - Raw detection results (always created)

When `-debug` flag is used, additional files are created:
- `000_original_with_box.png` - Original with detected subject box
- `001_debug_1200x675_A.png` - Debug overlays showing boxes and centers

## License

MIT