package ollama

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/ollama/ollama/api"
	"github.com/menta2k/image-analyzer/pkg/types"
)

// Client wraps the Ollama API client
type Client struct {
	client *api.Client
}

// NewClient creates a new Ollama client
func NewClient(ollamaURL string) (*Client, error) {
	// Parse the provided URL
	parsedURL, err := url.Parse(ollamaURL)
	if err != nil {
		return nil, fmt.Errorf("invalid URL: %v", err)
	}

	// Create base URL from the provided URL (removing path like /api/chat)
	baseURL := &url.URL{
		Scheme: parsedURL.Scheme,
		Host:   parsedURL.Host,
	}

	// Create client with the specified URL, ignoring environment
	client := api.NewClient(baseURL, http.DefaultClient)

	return &Client{client: client}, nil
}

// SimpleQuery performs a simple query with an image without expecting JSON
func (c *Client) SimpleQuery(ctx context.Context, model, prompt, imgB64 string) (string, error) {
	// Add timeout if context doesn't have one (longer for MiniCPM-V 4.5 on CPU)
	if _, hasDeadline := ctx.Deadline(); !hasDeadline {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, 300*time.Second) // 5 minutes for CPU processing
		defer cancel()
	}

	// Decode base64 image to raw bytes
	imgBytes, err := base64.StdEncoding.DecodeString(imgB64)
	if err != nil {
		return "", fmt.Errorf("failed to decode base64 image: %v", err)
	}

	// Create chat request without JSON format requirement
	streamFalse := false
	req := &api.ChatRequest{
		Model: model,
		Messages: []api.Message{
			{
				Role:    "user",
				Content: prompt,
				Images:  []api.ImageData{api.ImageData(imgBytes)},
			},
		},
		Stream: &streamFalse,
		// No Format field - let it return natural language
	}

	var responseContent string
	err = c.client.Chat(ctx, req, func(resp api.ChatResponse) error {
		responseContent = resp.Message.Content
		return nil
	})
	if err != nil {
		return "", fmt.Errorf("ollama chat error: %v", err)
	}

	return responseContent, nil
}

// AnalyzeImage analyzes an image and returns the detected subject information
func (c *Client) AnalyzeImage(ctx context.Context, model, prompt, imgB64 string) (*types.AnalysisResult, error) {
	// Add timeout if context doesn't have one (longer for MiniCPM-V 4.5 on CPU)
	if _, hasDeadline := ctx.Deadline(); !hasDeadline {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, 300*time.Second) // 5 minutes for CPU processing
		defer cancel()
	}

	// Decode base64 image to raw bytes
	imgBytes, err := base64.StdEncoding.DecodeString(imgB64)
	if err != nil {
		return nil, fmt.Errorf("failed to decode base64 image: %v", err)
	}

	// Create chat request using SDK types
	streamFalse := false

	// Set model-specific parameters for better performance
	options := map[string]any{}

	// Optimize for MiniCPM-V 4.5 if that's the model being used
	modelLower := strings.ToLower(model)
	if strings.Contains(modelLower, "minicpm-v4") ||
	   strings.Contains(modelLower, "minicpm-v-4") ||
	   strings.Contains(modelLower, "minicpmv4") {
		options["temperature"] = 0.7
		options["top_p"] = 0.8
		options["num_ctx"] = 4096
	}

	req := &api.ChatRequest{
		Model: model,
		Messages: []api.Message{
			{
				Role:    "user",
				Content: prompt,
				Images:  []api.ImageData{api.ImageData(imgBytes)},
			},
		},
		Stream:  &streamFalse,
		Options: options,
		// No Format field - let the prompt guide the format
	}

	var responseContent string
	err = c.client.Chat(ctx, req, func(resp api.ChatResponse) error {
		responseContent = resp.Message.Content
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("ollama chat error: %v", err)
	}

	if responseContent == "" {
		return nil, fmt.Errorf("empty response from ollama")
	}

	// Parse the response
	return parseAnalysisResult(responseContent)
}

// parseAnalysisResult parses the JSON response from the vision model
func parseAnalysisResult(raw string) (*types.AnalysisResult, error) {
	raw = sanitizeModelJSON(raw)

	// If the response doesn't look like JSON, return a conservative fallback
	if !strings.HasPrefix(strings.TrimSpace(raw), "{") {
		return &types.AnalysisResult{
			Primary: types.Primary{
				Label:      "unclear image",
				Confidence: 0.1,
				Box:        types.Box{X: 0.25, Y: 0.25, W: 0.5, H: 0.5},
				Cx:         0.5,
				Cy:         0.5,
			},
			Description: "Model returned non-JSON response",
			Tags:        []string{"unclear", "non-json", "fallback"},
		}, nil
	}

	var result types.AnalysisResult
	if err := json.Unmarshal([]byte(raw), &result); err != nil {
		// Try conservative brace-slice approach
		start := strings.Index(raw, "{")
		end := strings.LastIndex(raw, "}")
		if start >= 0 && end > start {
			if err2 := json.Unmarshal([]byte(raw[start:end+1]), &result); err2 != nil {
				// Return fallback instead of error
				return &types.AnalysisResult{
					Primary: types.Primary{
						Label:      "parse error",
						Confidence: 0.1,
						Box:        types.Box{X: 0.25, Y: 0.25, W: 0.5, H: 0.5},
						Cx:         0.5,
						Cy:         0.5,
					},
					Description: "Failed to parse model response",
					Tags:        []string{"parse-error", "fallback"},
				}, nil
			}
		} else {
			// Return fallback instead of error
			return &types.AnalysisResult{
				Primary: types.Primary{
					Label:      "no json found",
					Confidence: 0.1,
					Box:        types.Box{X: 0.25, Y: 0.25, W: 0.5, H: 0.5},
					Cx:         0.5,
					Cy:         0.5,
				},
				Description: "No valid JSON found in response",
				Tags:        []string{"no-json", "fallback"},
			}, nil
		}
	}

	return &result, nil
}

// sanitizeModelJSON removes code fences, comments, and trailing commas from JSON response
func sanitizeModelJSON(raw string) string {
	raw = strings.TrimSpace(raw)

	// Strip triple-backtick fences if present
	if strings.HasPrefix(raw, "```") {
		if i := strings.Index(raw, "\n"); i >= 0 {
			raw = raw[i+1:]
		}
		if j := strings.LastIndex(raw, "```"); j >= 0 {
			raw = raw[:j]
		}
	}
	raw = strings.TrimSpace(raw)
	raw = strings.Trim(raw, "`")

	// Remove /* ... */ block comments
	reBlock := regexp.MustCompile(`(?s)/\*.*?\*/`)
	raw = reBlock.ReplaceAllString(raw, "")

	// Remove // line/inline comments
	reLine := regexp.MustCompile(`(?m)^\s*//.*$`)
	raw = reLine.ReplaceAllString(raw, "")
	reInline := regexp.MustCompile(`(?m)//.*$`)
	raw = reInline.ReplaceAllString(raw, "")

	// Remove trailing commas before } or ]
	reTrailing := regexp.MustCompile(`,(\s*[}\]])`)
	raw = reTrailing.ReplaceAllString(raw, "$1")

	// Keep only the outermost {...}
	if start := strings.Index(raw, "{"); start >= 0 {
		if end := strings.LastIndex(raw, "}"); end > start {
			raw = raw[start : end+1]
		}
	}
	return strings.TrimSpace(raw)
}