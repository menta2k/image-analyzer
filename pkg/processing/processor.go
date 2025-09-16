package processing

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"io"
	"math"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/chai2010/webp"
	"github.com/disintegration/imaging"
	_ "golang.org/x/image/webp"

	"github.com/menta2k/image-analyzer/pkg/types"
)

// Processor handles image processing operations
type Processor struct{}

// NewProcessor creates a new image processor
func NewProcessor() *Processor {
	return &Processor{}
}

// LoadImageFromURL downloads and loads an image from a URL
func (p *Processor) LoadImageFromURL(imageURL string) (image.Image, error) {
	// Validate URL
	parsedURL, err := url.Parse(imageURL)
	if err != nil {
		return nil, fmt.Errorf("invalid URL: %v", err)
	}
	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		return nil, fmt.Errorf("unsupported URL scheme: %s (only http and https are supported)", parsedURL.Scheme)
	}

	// Create HTTP client with timeout
	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	// Create request with User-Agent header
	req, err := http.NewRequest("GET", imageURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %v", err)
	}
	req.Header.Set("User-Agent", "Image-Analyzer/1.0 (+https://github.com/sko/image-analyzer)")

	// Make request
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to download image: %v", err)
	}
	defer resp.Body.Close()

	// Check response status
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to download image: HTTP %d %s", resp.StatusCode, resp.Status)
	}

	// Check content type
	contentType := resp.Header.Get("Content-Type")
	if !strings.HasPrefix(contentType, "image/") {
		return nil, fmt.Errorf("URL does not point to an image (Content-Type: %s)", contentType)
	}

	// Read response body
	imageData, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read image data: %v", err)
	}

	// Decode image from bytes
	return p.decodeImageFromBytes(imageData)
}

// LoadImage loads an image from a file path with WebP support
func (p *Processor) LoadImage(path string) (image.Image, error) {
	// Try imaging.Open (registered decoders)
	if img, err := imaging.Open(path); err == nil {
		return img, nil
	}

	// Fallback: explicit WebP decode
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	low := strings.ToLower(path)
	if strings.HasSuffix(low, ".webp") || strings.Contains(low, ".webp") {
		if img, err := webp.Decode(f); err == nil {
			return img, nil
		}
		if _, err := f.Seek(0, 0); err == nil {
			if img, _, err := image.Decode(f); err == nil {
				return img, nil
			}
		}
	} else {
		if _, err := f.Seek(0, 0); err == nil {
			if img, _, err := image.Decode(f); err == nil {
				return img, nil
			}
		}
	}
	return nil, fmt.Errorf("image: unknown format for %s", path)
}

// LoadImageSmart loads an image from either a file path or URL
func (p *Processor) LoadImageSmart(source string) (image.Image, error) {
	// Check if it's a URL
	if strings.HasPrefix(source, "http://") || strings.HasPrefix(source, "https://") {
		return p.LoadImageFromURL(source)
	}
	// Otherwise treat as file path
	return p.LoadImage(source)
}

// decodeImageFromBytes decodes an image from byte data with WebP support
func (p *Processor) decodeImageFromBytes(data []byte) (image.Image, error) {
	// Try standard image.Decode first
	reader := bytes.NewReader(data)
	if img, _, err := image.Decode(reader); err == nil {
		return img, nil
	}

	// Try WebP decode
	reader = bytes.NewReader(data)
	if img, err := webp.Decode(reader); err == nil {
		return img, nil
	}

	return nil, fmt.Errorf("image: unknown or unsupported format")
}

// PrepareImageForModel converts an image to base64 for sending to vision models
func (p *Processor) PrepareImageForModel(img image.Image, format string, maxDim int, quality int) (string, error) {
	if maxDim > 0 {
		b := img.Bounds()
		w, h := b.Dx(), b.Dy()
		if w > maxDim || h > maxDim {
			if w >= h {
				img = imaging.Resize(img, maxDim, 0, imaging.Lanczos)
			} else {
				img = imaging.Resize(img, 0, maxDim, imaging.Lanczos)
			}
		}
	}

	var buf bytes.Buffer
	switch strings.ToLower(format) {
	case "png":
		enc := png.Encoder{CompressionLevel: png.BestCompression}
		if err := enc.Encode(&buf, img); err != nil {
			return "", err
		}
	default: // jpg
		if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: quality}); err != nil {
			return "", err
		}
	}
	return base64.StdEncoding.EncodeToString(buf.Bytes()), nil
}

// CropImageToBox crops an image to the specified normalized box
func (p *Processor) CropImageToBox(img image.Image, box types.Box, targetWidth, targetHeight int) (image.Image, error) {
	bounds := img.Bounds()
	fw, fh := float64(bounds.Dx()), float64(bounds.Dy())

	// Convert normalized box to pixel coordinates
	x0 := int(clamp(box.X, 0, 1)*fw + 0.5)
	y0 := int(clamp(box.Y, 0, 1)*fh + 0.5)
	x1 := int(clamp(box.X+box.W, 0, 1)*fw + 0.5)
	y1 := int(clamp(box.Y+box.H, 0, 1)*fh + 0.5)

	rect := image.Rect(x0, y0, x1, y1).Intersect(bounds)
	if rect.Empty() {
		return nil, fmt.Errorf("empty crop rectangle")
	}

	cropped := imaging.Crop(img, rect)

	// Resize to exact target dimensions while preserving aspect ratio
	if targetWidth > 0 && targetHeight > 0 {
		cropped = imaging.Fill(cropped, targetWidth, targetHeight, imaging.Center, imaging.Lanczos)
	}

	return cropped, nil
}

// CalculateOptimalCropBox calculates the optimal crop box for given aspect ratio centered at a point
func (p *Processor) CalculateOptimalCropBox(centerX, centerY float64, targetWidth, targetHeight, imgWidth, imgHeight int, zoom float64) types.Box {
	if zoom <= 0 {
		zoom = 1
	}

	r := float64(targetWidth) / float64(targetHeight) // target aspect W/H

	// Center in pixels
	cx := centerX * float64(imgWidth)
	cy := centerY * float64(imgHeight)

	// Max half extents allowed by image bounds
	halfWMax := math.Min(cx, float64(imgWidth)-cx)
	halfHMax := math.Min(cy, float64(imgHeight)-cy)

	// Width is limited by horizontal bounds AND by vertical bounds scaled by aspect
	maxWidthPx := math.Min(2*halfWMax, r*(2*halfHMax))
	widthPx := maxWidthPx * clamp(zoom, 0.01, 1.0)
	heightPx := widthPx / r

	// Top-left in pixels, clamped
	x0 := clamp(cx-widthPx/2, 0, float64(imgWidth)-widthPx)
	y0 := clamp(cy-heightPx/2, 0, float64(imgHeight)-heightPx)

	return types.Box{
		X: x0 / float64(imgWidth),
		Y: y0 / float64(imgHeight),
		W: widthPx / float64(imgWidth),
		H: heightPx / float64(imgHeight),
	}
}

// FindNearestPointToCenter finds the nearest point in a box to the image center
func (p *Processor) FindNearestPointToCenter(box types.Box) (float64, float64) {
	cx := clamp(0.5, box.X, box.X+box.W)
	cy := clamp(0.5, box.Y, box.Y+box.H)
	return cx, cy
}

// SaveImage saves an image to a file with the specified format and quality
func (p *Processor) SaveImage(img image.Image, path, format string, quality int, lossless bool) error {
	switch strings.ToLower(format) {
	case "webp":
		f, err := os.Create(path)
		if err != nil {
			return err
		}
		defer f.Close()
		opts := &webp.Options{Lossless: lossless, Quality: float32(quality)}
		return webp.Encode(f, img, opts)
	case "png":
		return imaging.Save(img, path)
	default: // jpg/jpeg
		return imaging.Save(img, path, imaging.JPEGQuality(quality))
	}
}

// CreateDebugOverlay creates an overlay image showing detection and crop boxes
func (p *Processor) CreateDebugOverlay(img image.Image, modelBox, cropBox types.Box, cropCx, cropCy float64) image.Image {
	nrgba := imaging.Clone(img)
	w := nrgba.Bounds().Dx()
	h := nrgba.Bounds().Dy()

	// Colors
	green := color.NRGBA{0, 255, 0, 255}                  // model box
	gold := color.NRGBA{255, 204, 0, 255}                 // crop box
	red := color.NRGBA{255, 0, 0, 255}                    // crop center
	blue := color.NRGBA{0, 170, 255, 255}                 // image center
	stroke := int(math.Max(2, 0.004*float64(minInt(w, h)))) // ~0.4% of min side
	cross := int(math.Max(4, 0.01*float64(minInt(w, h))))   // ~1% of min side

	// Draw model box
	drawBox(nrgba, modelBox, w, h, green, stroke)

	// Draw crop box if valid
	if cropBox.W > 0 && cropBox.H > 0 {
		drawBox(nrgba, cropBox, w, h, gold, stroke)
	}

	// Draw crop center crosshair
	px := int(clamp(cropCx, 0, 1)*float64(w) + 0.5)
	py := int(clamp(cropCy, 0, 1)*float64(h) + 0.5)
	drawHLine(nrgba, py, px-cross, px+cross, red)
	drawVLine(nrgba, px, py-cross, py+cross, red)

	// Draw image center marker
	ix, iy := w/2, h/2
	drawHLine(nrgba, iy, ix-6, ix+6, blue)
	drawVLine(nrgba, ix, iy-6, iy+6, blue)

	return nrgba
}

// Helper functions
func clamp(v, lo, hi float64) float64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func boxToPixels(box types.Box, w, h int) (int, int, int, int) {
	x0 := int(clamp(box.X, 0, 1)*float64(w) + 0.5)
	y0 := int(clamp(box.Y, 0, 1)*float64(h) + 0.5)
	x1 := int(clamp(box.X+box.W, 0, 1)*float64(w) + 0.5)
	y1 := int(clamp(box.Y+box.H, 0, 1)*float64(h) + 0.5)
	if x1 <= x0 {
		x1 = x0 + 1
	}
	if y1 <= y0 {
		y1 = y0 + 1
	}
	return x0, y0, x1, y1
}

func drawBox(img *image.NRGBA, box types.Box, w, h int, color color.NRGBA, stroke int) {
	x0, y0, x1, y1 := boxToPixels(box, w, h)
	for s := 0; s < stroke; s++ {
		drawHLine(img, y0+s, x0, x1, color)
		drawHLine(img, y1-1-s, x0, x1, color)
		drawVLine(img, x0+s, y0, y1, color)
		drawVLine(img, x1-1-s, y0, y1, color)
	}
}

func drawHLine(img *image.NRGBA, y, x0, x1 int, c color.NRGBA) {
	if y < 0 || y >= img.Bounds().Dy() {
		return
	}
	if x0 > x1 {
		x0, x1 = x1, x0
	}
	if x1 <= 0 || x0 >= img.Bounds().Dx() {
		return
	}
	if x0 < 0 {
		x0 = 0
	}
	if x1 > img.Bounds().Dx() {
		x1 = img.Bounds().Dx()
	}
	i := y*img.Stride + x0*4
	for x := x0; x < x1; x++ {
		img.Pix[i+0] = c.R
		img.Pix[i+1] = c.G
		img.Pix[i+2] = c.B
		img.Pix[i+3] = c.A
		i += 4
	}
}

func drawVLine(img *image.NRGBA, x, y0, y1 int, c color.NRGBA) {
	if x < 0 || x >= img.Bounds().Dx() {
		return
	}
	if y0 > y1 {
		y0, y1 = y1, y0
	}
	if y1 <= 0 || y0 >= img.Bounds().Dy() {
		return
	}
	if y0 < 0 {
		y0 = 0
	}
	if y1 > img.Bounds().Dy() {
		y1 = img.Bounds().Dy()
	}
	i := y0*img.Stride + x*4
	for y := y0; y < y1; y++ {
		img.Pix[i+0] = c.R
		img.Pix[i+1] = c.G
		img.Pix[i+2] = c.B
		img.Pix[i+3] = c.A
		i += img.Stride
	}
}