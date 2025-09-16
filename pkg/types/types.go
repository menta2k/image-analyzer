package types

// Box represents a normalized bounding box with coordinates in [0,1] range
type Box struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
	W float64 `json:"w"`
	H float64 `json:"h"`
}

// Primary represents the primary subject detected in an image
type Primary struct {
	Label      string  `json:"label"`
	Confidence float64 `json:"confidence"`
	Box        Box     `json:"box"`
	Cx         float64 `json:"cx"`
	Cy         float64 `json:"cy"`
}

// AnalysisResult contains the complete analysis result from the vision model
type AnalysisResult struct {
	Primary     Primary  `json:"primary"`
	Description string   `json:"description"`
	Tags        []string `json:"tags"`
}

// CropConfig defines the configuration for image cropping
type CropConfig struct {
	Width     int
	Height    int
	Quality   int
	Lossless  bool
	Extension string
}

// ProcessingOptions contains options for image processing
type ProcessingOptions struct {
	OutputDir    string
	Zoom         float64
	TargetSizes  [][2]int
	DebugOverlay bool
}