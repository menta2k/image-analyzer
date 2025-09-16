package llamacpp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/menta2k/image-analyzer/pkg/types"
)

type Client struct {
	baseURL    string
	httpClient *http.Client
}

// OpenAI-compatible message format
type Message struct {
	Role    string      `json:"role"`
	Content interface{} `json:"content"` // Can be string or []ContentPart
}

type ContentPart struct {
	Type     string    `json:"type"`
	Text     string    `json:"text,omitempty"`
	ImageURL *ImageURL `json:"image_url,omitempty"`
}

type ImageURL struct {
	URL string `json:"url"`
}

// OpenAI-compatible chat completion request
type ChatCompletionRequest struct {
	Model       string    `json:"model"`
	Messages    []Message `json:"messages"`
	Temperature float64   `json:"temperature,omitempty"`
	MaxTokens   int       `json:"max_tokens,omitempty"`
	TopP        float64   `json:"top_p,omitempty"`
	Stream      bool      `json:"stream"`
}

// OpenAI-compatible chat completion response
type ChatCompletionResponse struct {
	ID      string   `json:"id"`
	Object  string   `json:"object"`
	Created int64    `json:"created"`
	Model   string   `json:"model"`
	Choices []Choice `json:"choices"`
	Usage   Usage    `json:"usage,omitempty"`
}

type Choice struct {
	Index        int     `json:"index"`
	Message      Message `json:"message"`
	FinishReason string  `json:"finish_reason,omitempty"`
}

type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

func NewClient(serverURL string) (*Client, error) {
	if serverURL == "" {
		serverURL = "http://localhost:8080"
	}

	return &Client{
		baseURL: strings.TrimSuffix(serverURL, "/"),
		httpClient: &http.Client{
			Timeout: 5 * time.Minute,
		},
	}, nil
}

func (c *Client) SimpleQuery(ctx context.Context, model, prompt, imgB64 string) (string, error) {
	if _, hasDeadline := ctx.Deadline(); !hasDeadline {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, 300*time.Second)
		defer cancel()
	}

	content := []ContentPart{
		{
			Type: "text",
			Text: prompt,
		},
	}

	if imgB64 != "" {
		content = append(content, ContentPart{
			Type: "image_url",
			ImageURL: &ImageURL{
				URL: "data:image/jpeg;base64," + imgB64,
			},
		})
	}

	req := ChatCompletionRequest{
		Model: model,
		Messages: []Message{
			{
				Role:    "user",
				Content: content,
			},
		},
		Temperature: 0.7,
		MaxTokens:   2048,
		TopP:        0.9,
		Stream:      false,
	}

	respBody, err := c.sendRequest(ctx, "/v1/chat/completions", req)
	if err != nil {
		return "", fmt.Errorf("request failed: %v", err)
	}

	var resp ChatCompletionResponse
	if err := json.Unmarshal(respBody, &resp); err != nil {
		return "", fmt.Errorf("failed to parse response: %v", err)
	}

	if len(resp.Choices) == 0 {
		return "", fmt.Errorf("no choices in response")
	}

	// Extract text from the response (handle both string and array formats)
	switch content := resp.Choices[0].Message.Content.(type) {
	case string:
		return content, nil
	case []interface{}:
		for _, item := range content {
			if partMap, ok := item.(map[string]interface{}); ok {
				if text, ok := partMap["text"].(string); ok && text != "" {
					return text, nil
				}
			}
		}
	}

	return "", fmt.Errorf("no text content in response")
}

func (c *Client) AnalyzeImage(ctx context.Context, model, prompt, imgB64 string) (*types.AnalysisResult, error) {
	if _, hasDeadline := ctx.Deadline(); !hasDeadline {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, 300*time.Second)
		defer cancel()
	}

	content := []ContentPart{
		{
			Type: "text",
			Text: prompt,
		},
	}

	if imgB64 != "" {
		content = append(content, ContentPart{
			Type: "image_url",
			ImageURL: &ImageURL{
				URL: "data:image/jpeg;base64," + imgB64,
			},
		})
	}

	req := ChatCompletionRequest{
		Model: model,
		Messages: []Message{
			{
				Role:    "user",
				Content: content,
			},
		},
		Temperature: 0.7,
		MaxTokens:   4096,
		TopP:        0.8,
		Stream:      false,
	}

	respBody, err := c.sendRequest(ctx, "/v1/chat/completions", req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %v", err)
	}

	var resp ChatCompletionResponse
	if err := json.Unmarshal(respBody, &resp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %v", err)
	}

	if len(resp.Choices) == 0 {
		return nil, fmt.Errorf("no choices in response")
	}

	// Extract text content from the response (handle both string and array formats)
	var responseText string
	switch content := resp.Choices[0].Message.Content.(type) {
	case string:
		responseText = content
	case []interface{}:
		for _, item := range content {
			if partMap, ok := item.(map[string]interface{}); ok {
				if text, ok := partMap["text"].(string); ok && text != "" {
					responseText = text
					break
				}
			}
		}
	}

	if responseText == "" {
		return nil, fmt.Errorf("empty response from llama.cpp server")
	}

	return parseAnalysisResult(responseText)
}

func (c *Client) sendRequest(ctx context.Context, endpoint string, payload interface{}) ([]byte, error) {
	jsonData, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %v", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+endpoint, bytes.NewReader(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %v", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("server returned status %d: %s", resp.StatusCode, string(body))
	}

	return body, nil
}

func parseAnalysisResult(raw string) (*types.AnalysisResult, error) {
	raw = sanitizeModelJSON(raw)

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
		// Try to extract JSON from the response
		start := strings.Index(raw, "{")
		end := strings.LastIndex(raw, "}")
		if start >= 0 && end > start {
			extracted := raw[start : end+1]
			if err2 := json.Unmarshal([]byte(extracted), &result); err2 != nil {
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

	// Check if result is empty and provide fallback values
	if result.Primary.Label == "" && result.Primary.Confidence == 0 {
		if result.Primary.Cx == 0 && result.Primary.Cy == 0 {
			result.Primary.Cx = 0.5
			result.Primary.Cy = 0.5
		}
		if result.Primary.Box.W == 0 && result.Primary.Box.H == 0 {
			result.Primary.Box = types.Box{X: 0.25, Y: 0.25, W: 0.5, H: 0.5}
		}
	}

	return &result, nil
}

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