package detection

import (
	"context"
	"math"
	"strings"

	"github.com/menta2k/image-analyzer/pkg/client"
	"github.com/menta2k/image-analyzer/pkg/types"
)

// SimpleTestPrompt for testing if the model can see images
const SimpleTestPrompt = `What do you see in this image? Describe it briefly.`

// DefaultPrompt is the default prompt for subject detection
const DefaultPrompt = `You are an image subject locator.

Return JSON only:
{
  "primary": {
    "label": "string",
    "confidence": 0.0,
    "box": {"x": 0.0, "y": 0.0, "w": 0.0, "h": 0.0},
    "cx": 0.0,
    "cy": 0.0
  },
  "description": "short neutral sentence (≤ 20 words)",
  "tags": ["tag1", "tag2", "tag3", "tag4", "tag5"]
}

HARD RULES
- All coordinates are normalized to [0,1] (NOT pixels).
- The box center must satisfy: abs(cx - 0.5) <= 0.10 and abs(cy - 0.5) <= 0.10.
- If your best box violates it, ADJUST the box so its center lies on the nearest allowed boundary.
- The box should tightly include the visually dominant subject (prefer people/vehicles/animals; else the most central salient object).
- Description must be brief and factual. Do not guess real identities.
- Tags: lowercase, concise, no punctuation or duplicates.
- If no subject is found, return:
  {
    "primary":{"label":"none","confidence":0.0,"box":{"x":0.25,"y":0.25,"w":0.50,"h":0.50},"cx":0.5,"cy":0.5},
    "description":"centered generic scene",
    "tags":["generic","center","subject","photo","scene"]
  }
- JSON only. No markdown, no code fences, no comments, no trailing commas.`

// Detector handles image subject detection using vision models
type Detector struct {
	client client.VisionClient
}

// NewDetector creates a new detector with a vision client
func NewDetector(client client.VisionClient) *Detector {
	return &Detector{client: client}
}

// DetectSubject analyzes an image and detects the primary subject
func (d *Detector) DetectSubject(ctx context.Context, model, imageB64 string) (*types.AnalysisResult, error) {
	result, err := d.DetectSubjectWithPrompt(ctx, model, imageB64, DefaultPrompt)
	if err != nil {
		return nil, err
	}

	// Validate and adjust result based on confidence and common sense
	result = d.validateAndAdjustResult(result)

	return result, nil
}

// DetectSubjectWithPrompt analyzes an image with a custom prompt
func (d *Detector) DetectSubjectWithPrompt(ctx context.Context, model, imageB64, prompt string) (*types.AnalysisResult, error) {
	result, err := d.client.AnalyzeImage(ctx, model, prompt, imageB64)
	if err != nil {
		return nil, err
	}

	// Post-process the result
	result.Primary.Box = normalizeBox(result.Primary.Box, 1, 1) // Already normalized but ensure bounds
	result.Tags = normalizeTags(result.Tags)

	return result, nil
}

// TestVision tests if the model can actually see the image with a simple prompt
func (d *Detector) TestVision(ctx context.Context, model, imageB64 string) (string, error) {
	// Use the ollama client directly for a simple text response
	return d.client.SimpleQuery(ctx, model, SimpleTestPrompt, imageB64)
}

// validateAndAdjustResult validates the detection result and adjusts for reliability
func (d *Detector) validateAndAdjustResult(result *types.AnalysisResult) *types.AnalysisResult {
	// Check if this is a "none" result from the prompt (which is good)
	if strings.ToLower(result.Primary.Label) == "none" {
		// This is the expected fallback from the prompt, keep as-is
		return result
	}

	// Normalize the bounding box based on the center constraint
	// The prompt requires abs(cx - 0.5) <= 0.10 and abs(cy - 0.5) <= 0.10
	if math.Abs(result.Primary.Cx-0.5) > 0.10 || math.Abs(result.Primary.Cy-0.5) > 0.10 {
		// Adjust to nearest valid center
		result.Primary.Cx = clamp(result.Primary.Cx, 0.4, 0.6)
		result.Primary.Cy = clamp(result.Primary.Cy, 0.4, 0.6)
	}

	// If any fallback indicators are present, ensure it's marked as such
	fallbackIndicators := []string{"unclear", "empty", "parse", "error", "fallback", "non-json", "generic"}
	for _, indicator := range fallbackIndicators {
		if strings.Contains(strings.ToLower(result.Primary.Label), indicator) ||
			strings.Contains(strings.ToLower(result.Description), indicator) {
			if result.Primary.Label != "none" {
				result.Primary.Label = "none"
				result.Primary.Confidence = 0.0
			}
			break
		}
	}

	return result
}

// clamp ensures a value is within the given bounds
func clamp(v, lo, hi float64) float64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

// normalizeBox ensures box coordinates are within [0,1] bounds
func normalizeBox(b types.Box, imgW, imgH int) types.Box {
	if imgW <= 0 || imgH <= 0 {
		// Already normalized case
		return types.Box{
			X: clamp(b.X, 0, 1),
			Y: clamp(b.Y, 0, 1),
			W: clamp(b.W, 0, 1),
			H: clamp(b.H, 0, 1),
		}
	}

	// Convert from pixel coordinates if needed
	if b.X > 1 || b.Y > 1 || b.W > 1 || b.H > 1 {
		return types.Box{
			X: clamp(b.X/float64(imgW), 0, 1),
			Y: clamp(b.Y/float64(imgH), 0, 1),
			W: clamp(b.W/float64(imgW), 0, 1),
			H: clamp(b.H/float64(imgH), 0, 1),
		}
	}

	return types.Box{
		X: clamp(b.X, 0, 1),
		Y: clamp(b.Y, 0, 1),
		W: clamp(b.W, 0, 1),
		H: clamp(b.H, 0, 1),
	}
}

// normalizeTags ensures tags are cleaned and limited to 5 entries
func normalizeTags(tags []string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, 5)
	for _, t := range tags {
		t = strings.ToLower(strings.TrimSpace(t))
		if t == "" {
			continue
		}
		if _, ok := seen[t]; ok {
			continue
		}
		seen[t] = struct{}{}
		out = append(out, t)
		if len(out) == 5 {
			break
		}
	}
	return out
}