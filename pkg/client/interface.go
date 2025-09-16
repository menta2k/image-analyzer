package client

import (
	"context"
	"github.com/menta2k/image-analyzer/pkg/types"
)

type VisionClient interface {
	SimpleQuery(ctx context.Context, model, prompt, imgB64 string) (string, error)
	AnalyzeImage(ctx context.Context, model, prompt, imgB64 string) (*types.AnalysisResult, error)
}