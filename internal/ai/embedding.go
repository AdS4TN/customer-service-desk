package ai

import (
	"context"
	"fmt"

	openai "github.com/openai/openai-go/v3"

	"agent-desk/internal/models"
	"agent-desk/internal/pkg/enums"
	"agent-desk/internal/pkg/errorsx"
)

type EmbeddingResult struct {
	Vector     []float32
	TokensUsed int
	ModelName  string
	Dimension  int
}

type embedding struct{}

var Embedding = &embedding{}

func (s *embedding) GetModel(ctx context.Context) (*models.AIConfig, error) {
	config, err := GetEnabledAIConfig(enums.AIModelTypeEmbedding)
	if err != nil {
		return nil, errorsx.BusinessErrorI18n(2001, "error.embeddingModel.noneEnabled")
	}
	return config, nil
}

func (s *embedding) GenerateEmbedding(ctx context.Context, text string) (*EmbeddingResult, error) {
	if text == "" {
		return nil, errorsx.InvalidParamI18n("error.e0215")
	}

	result, err := s.callEmbeddingAPI(ctx, text)
	if err != nil {
		return nil, err
	}

	return result, nil
}

func (s *embedding) GenerateBatchEmbeddings(ctx context.Context, texts []string) ([]EmbeddingResult, error) {
	if len(texts) == 0 {
		return nil, errorsx.InvalidParamI18n("error.e0216")
	}

	for _, text := range texts {
		if text == "" {
			return nil, errorsx.InvalidParamI18n("error.e0215")
		}
	}
	config, err := GetEnabledAIConfig(enums.AIModelTypeEmbedding)
	if err != nil {
		return nil, err
	}
	client := newOpenAIClient(*config)
	response, err := client.Embeddings.New(ctx, openai.EmbeddingNewParams{
		Input: openai.EmbeddingNewParamsInputUnion{OfArrayOfStrings: texts},
		Model: openai.EmbeddingModel(config.ModelName),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to call batch embedding api: %w", err)
	}
	if len(response.Data) != len(texts) {
		return nil, fmt.Errorf("embedding api returned %d vectors for %d inputs", len(response.Data), len(texts))
	}
	results := make([]EmbeddingResult, len(texts))
	for _, data := range response.Data {
		if data.Index < 0 || int(data.Index) >= len(results) {
			return nil, fmt.Errorf("embedding api returned invalid index %d", data.Index)
		}
		vector := make([]float32, len(data.Embedding))
		for index, value := range data.Embedding {
			vector[index] = float32(value)
		}
		results[data.Index] = EmbeddingResult{
			Vector: vector, TokensUsed: int(response.Usage.TotalTokens),
			ModelName: response.Model, Dimension: len(vector),
		}
	}
	return results, nil
}

func (s *embedding) callEmbeddingAPI(ctx context.Context, text string) (*EmbeddingResult, error) {
	config, err := GetEnabledAIConfig(enums.AIModelTypeEmbedding)
	if err != nil {
		return nil, err
	}
	client := newOpenAIClient(*config)
	embeddingResp, err := client.Embeddings.New(ctx, openai.EmbeddingNewParams{
		Input: openai.EmbeddingNewParamsInputUnion{
			OfString: openai.String(text),
		},
		Model: openai.EmbeddingModel(config.ModelName),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to call embedding api: %w", err)
	}

	if len(embeddingResp.Data) == 0 {
		return nil, fmt.Errorf("no embedding data in response")
	}
	vector := make([]float32, 0, len(embeddingResp.Data[0].Embedding))
	for _, item := range embeddingResp.Data[0].Embedding {
		vector = append(vector, float32(item))
	}

	return &EmbeddingResult{
		Vector:     vector,
		TokensUsed: int(embeddingResp.Usage.TotalTokens),
		ModelName:  embeddingResp.Model,
		Dimension:  len(vector),
	}, nil
}

func (s *embedding) GetDimension(ctx context.Context) (int, error) {
	model, err := s.GetModel(ctx)
	if err != nil {
		return 0, err
	}
	return model.Dimension, nil
}
