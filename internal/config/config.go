package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// Config holds the application configuration
type Config struct {
	Analyzer AnalyzerConfig `json:"analyzer"`
	Vision   VisionConfig   `json:"vision"`
	Cropper  CropperConfig  `json:"cropper"`
	Output   OutputConfig   `json:"output"`
}

// AnalyzerConfig holds configuration for image analysis
type AnalyzerConfig struct {
	DefaultQuality   int      `json:"default_quality"`
	SupportedFormats []string `json:"supported_formats"`
	MinImageSize     int      `json:"min_image_size"`
}

// VisionConfig holds configuration for subject detection
type VisionConfig struct {
	EdgeThreshold    float64 `json:"edge_threshold"`
	ContrastWeight   float64 `json:"contrast_weight"`
	ColorWeight      float64 `json:"color_weight"`
	SaliencyWeight   float64 `json:"saliency_weight"`
	MinSubjectRatio  float64 `json:"min_subject_ratio"`
}

// CropperConfig holds configuration for smart cropping
type CropperConfig struct {
	PreserveAspectRatio bool    `json:"preserve_aspect_ratio"`
	AllowUpscaling      bool    `json:"allow_upscaling"`
	PaddingRatio        float64 `json:"padding_ratio"`
	QualityThreshold    float64 `json:"quality_threshold"`
}

// OutputConfig holds configuration for output generation
type OutputConfig struct {
	DefaultFormat string `json:"default_format"`
	OutputDir     string `json:"output_dir"`
	Prefix        string `json:"prefix"`
	Suffix        string `json:"suffix"`
}

// Default returns a configuration with default values
func Default() *Config {
	return &Config{
		Analyzer: AnalyzerConfig{
			DefaultQuality:   85,
			SupportedFormats: []string{"jpg", "jpeg", "png"},
			MinImageSize:     100,
		},
		Vision: VisionConfig{
			EdgeThreshold:   0.1,
			ContrastWeight:  0.3,
			ColorWeight:     0.2,
			SaliencyWeight:  0.5,
			MinSubjectRatio: 0.1,
		},
		Cropper: CropperConfig{
			PreserveAspectRatio: true,
			AllowUpscaling:      false,
			PaddingRatio:        0.1,
			QualityThreshold:    0.7,
		},
		Output: OutputConfig{
			DefaultFormat: "jpg",
			OutputDir:     "./output",
			Prefix:        "",
			Suffix:        "_cropped",
		},
	}
}

// LoadFromFile loads configuration from a JSON file
func LoadFromFile(filename string) (*Config, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}
	
	var config Config
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}
	
	return &config, nil
}

// SaveToFile saves configuration to a JSON file
func (c *Config) SaveToFile(filename string) error {
	// Create directory if it doesn't exist
	dir := filepath.Dir(filename)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}
	
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}
	
	if err := os.WriteFile(filename, data, 0644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}
	
	return nil
}

// Validate checks if the configuration is valid
func (c *Config) Validate() error {
	if c.Analyzer.DefaultQuality < 1 || c.Analyzer.DefaultQuality > 100 {
		return fmt.Errorf("analyzer.default_quality must be between 1 and 100")
	}
	
	if c.Analyzer.MinImageSize < 1 {
		return fmt.Errorf("analyzer.min_image_size must be positive")
	}
	
	if len(c.Analyzer.SupportedFormats) == 0 {
		return fmt.Errorf("analyzer.supported_formats cannot be empty")
	}
	
	if c.Vision.EdgeThreshold < 0 || c.Vision.EdgeThreshold > 1 {
		return fmt.Errorf("vision.edge_threshold must be between 0 and 1")
	}
	
	if c.Vision.MinSubjectRatio < 0 || c.Vision.MinSubjectRatio > 1 {
		return fmt.Errorf("vision.min_subject_ratio must be between 0 and 1")
	}
	
	if c.Cropper.PaddingRatio < 0 || c.Cropper.PaddingRatio > 1 {
		return fmt.Errorf("cropper.padding_ratio must be between 0 and 1")
	}
	
	if c.Cropper.QualityThreshold < 0 || c.Cropper.QualityThreshold > 1 {
		return fmt.Errorf("cropper.quality_threshold must be between 0 and 1")
	}
	
	return nil
}

// GetConfigPath returns the default configuration file path
func GetConfigPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return "./config.json"
	}
	return filepath.Join(home, ".config", "image-analyzer", "config.json")
}