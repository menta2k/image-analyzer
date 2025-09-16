# Image Analyzer

An intelligent image analysis and cropping tool that uses vision language models to detect subjects and generate optimal crops for various aspect ratios.

## Features

- **Intelligent Subject Detection**: Automatically detects the primary subject in images using vision models
- **Smart Cropping**: Generates optimally cropped versions in multiple aspect ratios while preserving the main subject
- **Multiple Backend Support**: Works with both Ollama and llama.cpp servers (OpenAI-compatible API)
- **Flexible Output Formats**: Supports JPEG, PNG, and WebP output formats
- **Debug Overlays**: Optional visualization of detected subjects and crop boundaries
- **Batch Processing**: Process multiple target sizes in a single run
- **URL Support**: Load images directly from HTTP/HTTPS URLs

## Installation

### Prerequisites

- Go 1.24.6 or later
- Either:
  - Ollama installed and running with a vision model
  - llama.cpp server with a compatible vision model (e.g., MiniCPM-V)

### Build from Source

```bash
git clone https://github.com/menta2k/image-analyzer.git
cd image-analyzer
go build -o image-analyzer cmd/image-analyzer/main.go
```

## Quick Start

### Using llama.cpp Server (Default)

1. Start llama.cpp server with a vision model:

```bash
# Using Docker Compose (recommended)
docker-compose -f docker-compose.minicpm.yml up

# Or manually with llama.cpp
./llama-server \
  -m models/ggml-model-Q4_K_M.gguf \
  --mmproj models/mmproj-model-f16.gguf \
  -c 8192 \
  --host 0.0.0.0 \
  --port 8080
```

2. Run the analyzer:

```bash
./image-analyzer -in input.jpg
```

### Using Ollama

1. Install Ollama and pull a vision model:

```bash
ollama pull minicpm-v:latest
# or
ollama pull llava
```

2. Run the analyzer with Ollama backend:

```bash
./image-analyzer -in input.jpg -backend ollama
```

## Usage Examples

### Basic Usage

```bash
# Analyze local image with llama.cpp (default)
./image-analyzer -in photo.jpg

# Analyze image from URL
./image-analyzer -in "https://example.com/image.jpg"

# Use Ollama backend with specific model
./image-analyzer -in photo.jpg -backend ollama -model llava

# Custom output directory and format
./image-analyzer -in photo.jpg -out results/ -ext webp -quality 95
```

### Advanced Options

```bash
# Full control over processing
./image-analyzer \
  -in input.jpg \
  -backend llamacpp \
  -url http://localhost:8080 \
  -model openbmb/minicpm-v4.5 \
  -out crops/ \
  -ext webp \
  -quality 95 \
  -lossless false \
  -zoom 0.9 \
  -debug \
  -sendfmt png \
  -sendsize 2048 \
  -sendq 90
```

## Command Line Options

### Core Options

| Flag | Default | Description |
|------|---------|-------------|
| `-in` | (required) | Input image path or URL (jpg/png/webp) |
| `-backend` | `llamacpp` | Backend to use: `ollama` or `llamacpp` |
| `-url` | Auto | Server URL (defaults: ollama=http://localhost:11435/api/chat, llamacpp=http://localhost:8080) |
| `-model` | `openbmb/minicpm-v4.5` | Model name to use |
| `-out` | `out` | Output directory for processed images |

### Output Options

| Flag | Default | Description |
|------|---------|-------------|
| `-ext` | `jpg` | Output format: `jpg`, `png`, or `webp` |
| `-quality` | `90` | JPEG/WebP quality (1-100) |
| `-lossless` | `false` | Enable lossless WebP mode |
| `-zoom` | `1.0` | Zoom factor for crops (0.01-1.0) |
| `-debug` | `false` | Create debug overlay images |

### Model Input Options

| Flag | Default | Description |
|------|---------|-------------|
| `-sendfmt` | `jpg` | Format sent to model: `jpg` or `png` |
| `-sendsize` | `1536` | Max dimension for model input (0=original) |
| `-sendq` | `85` | JPEG quality for model input |

### Debug Overlay Options

| Flag | Default | Description |
|------|---------|-------------|
| `-dbgext` | `png` | Debug overlay format |
| `-dbgquality` | `92` | Debug overlay quality |
| `-dbglossless` | `false` | Debug overlay WebP lossless |

## Output Files

The tool generates:

### Cropped Images
Multiple crops in different aspect ratios:
- `001_1200x675_A.jpg` - 16:9 landscape
- `002_1200x800_A.jpg` - 3:2 landscape
- `003_400x250_A.jpg` - 8:5 small
- `004_600x400_A.jpg` - 3:2 medium
- `005_1200x630_A.jpg` - Social media optimized

### Analysis Results
- `model_output.json` - Detection results with:
  - Primary subject label and confidence
  - Bounding box coordinates (normalized 0-1)
  - Description and tags

### Debug Overlays (with `-debug` flag)
- `000_original_with_box.png` - Original with detected subject (green box)
- `001_debug_1200x675_A.png` - Crop overlays showing:
  - Green: Detected subject box
  - Red: Crop boundary
  - Blue/Cyan: Center points

## API Usage

### Basic Integration

```go
import (
    "context"
    "github.com/menta2k/image-analyzer/pkg/detection"
    "github.com/menta2k/image-analyzer/pkg/llamacpp"
    "github.com/menta2k/image-analyzer/pkg/processing"
)

func main() {
    // Create components
    processor := processing.NewProcessor()
    client, _ := llamacpp.NewClient("http://localhost:8080")
    detector := detection.NewDetector(client)

    // Load and prepare image
    img, _ := processor.LoadImageSmart("photo.jpg")
    imgB64, _ := processor.PrepareImageForModel(img, "jpg", 1536, 85)

    // Detect subject
    result, _ := detector.DetectSubject(context.Background(), "model", imgB64)

    // Generate crop
    cx, cy := processor.FindNearestPointToCenter(result.Primary.Box)
    cropBox := processor.CalculateOptimalCropBox(cx, cy, 1200, 675,
        img.Bounds().Dx(), img.Bounds().Dy(), 1.0)
    cropped, _ := processor.CropImageToBox(img, cropBox, 1200, 675)

    // Save result
    processor.SaveImage(cropped, "output.jpg", "jpg", 90, false)
}
```

### Custom Detection Prompts

```go
// Use custom prompt for specific detection needs
customPrompt := `Detect the main person's face in this image...`
result, _ := detector.DetectSubjectWithPrompt(
    ctx, "model", imgB64, customPrompt
)
```

## Architecture

```
image-analyzer/
├── cmd/
│   └── image-analyzer/      # CLI application
├── pkg/
│   ├── client/              # Backend interface
│   ├── detection/           # Subject detection logic
│   ├── llamacpp/            # llama.cpp client (OpenAI-compatible)
│   ├── ollama/              # Ollama client
│   ├── processing/          # Image processing and cropping
│   └── types/               # Shared data types
├── contrib/
│   └── models/              # Model storage (for Docker)
├── example/                 # Example usage
└── docker-compose.minicpm.yml
```

## Package APIs

### Core Types (`pkg/types`)
- `Box`: Normalized bounding box (0-1 coordinates)
- `Primary`: Detected subject with confidence
- `AnalysisResult`: Complete detection result

### Detection (`pkg/detection`)
- `NewDetector(client)`: Create detector with backend client
- `DetectSubject()`: Detect with default prompt
- `DetectSubjectWithPrompt()`: Custom detection prompt

### Processing (`pkg/processing`)
- `LoadImageSmart()`: Load from file or URL
- `PrepareImageForModel()`: Optimize for model input
- `CalculateOptimalCropBox()`: Smart crop calculation
- `CropImageToBox()`: Execute crop
- `CreateDebugOverlay()`: Visualization

### Backends
- `pkg/llamacpp`: OpenAI-compatible API client
- `pkg/ollama`: Ollama-specific client

## Supported Models

### Via llama.cpp
- MiniCPM-V 4.5 (recommended)
- Any GGUF vision model with multimodal projector
- Models compatible with OpenAI vision API

### Via Ollama
- minicpm-v (all versions)
- llava (all variants)
- Any Ollama-compatible vision model

## Docker Deployment

Use the provided Docker Compose for easy deployment:

```yaml
version: '3.8'

services:
  minicpmv:
    image: ghcr.io/ggml-org/llama.cpp:full-cuda
    command: >
      --server
      -m /models/ggml-model-Q4_K_M.gguf
      --mmproj /models/mmproj-model-f16.gguf
      -c 8192
      -np 2
      -ngl 999
      --host 0.0.0.0
      --port 8080
    ports:
      - "8080:8080"
    volumes:
      - ./contrib/models:/models
```

## Performance Tips

1. **Model Input Size**: Reduce `-sendsize` for faster processing (default 1536px)
2. **Model Selection**: Q4_K_M quantization offers good speed/quality balance
3. **GPU Acceleration**: Use CUDA-enabled builds for 10x+ speedup
4. **Batch Processing**: Tool processes multiple crops efficiently in one run
5. **Image Formats**: JPEG with 85-90 quality is optimal for model input

## Contributing

Contributions are welcome! Please feel free to submit pull requests or open issues.

## License

MIT

## Acknowledgments

- [Ollama](https://github.com/ollama/ollama) - Model serving infrastructure
- [llama.cpp](https://github.com/ggerganov/llama.cpp) - Efficient inference engine
- [MiniCPM-V](https://github.com/OpenBMB/MiniCPM-V) - Vision language model
- [Imaging](https://github.com/disintegration/imaging) - Go image processing