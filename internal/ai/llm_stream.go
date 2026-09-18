package ai

import (
	"context"
	"encoding/json"
	"io"
	"strings"
	"time"
	"unicode/utf8"

	"agent-desk/internal/models"
	openai "github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/shared"
)

// ChatProgress reports activity, never reasoning text or partial unvalidated JSON.
type ChatProgress struct {
	OutputChars    int
	LastResponseAt time.Time
	Phase          string
}

func (s *llm) ChatWithProgress(ctx context.Context, cfg models.AIConfig, system, user string, progress func(ChatProgress)) (*ChatCompletionResult, error) {
	params := openai.ChatCompletionNewParams{Model: shared.ChatModel(cfg.ModelName), Messages: []openai.ChatCompletionMessageParamUnion{openai.SystemMessage(system), openai.UserMessage(user)}}
	if cfg.MaxOutputTokens > 0 {
		params.MaxCompletionTokens = openai.Int(int64(cfg.MaxOutputTokens))
	}
	applyProviderSpecificChatParams(&params, cfg)
	client := newOpenAIClient(cfg)
	stream := client.Chat.Completions.NewStreaming(ctx, params)
	defer stream.Close()
	var content strings.Builder
	result := &ChatCompletionResult{ModelName: cfg.ModelName}
	state := ChatProgress{Phase: "waiting"}
	for stream.Next() {
		chunk := stream.Current()
		state.LastResponseAt = time.Now()
		for _, choice := range chunk.Choices {
			if choice.Index != 0 {
				continue
			}
			if choice.Delta.Content != "" {
				content.WriteString(choice.Delta.Content)
				state.OutputChars += utf8.RuneCountInString(choice.Delta.Content)
				state.Phase = "generating"
			} else if state.OutputChars == 0 {
				var extra struct {
					ReasoningContent string `json:"reasoning_content"`
				}
				_ = json.Unmarshal([]byte(choice.Delta.RawJSON()), &extra)
				if extra.ReasoningContent != "" {
					state.Phase = "thinking"
				}
			}
			if choice.FinishReason != "" {
				result.FinishReason = choice.FinishReason
			}
		}
		if progress != nil {
			progress(state)
		}
	}
	if err := stream.Err(); err != nil {
		return nil, err
	}
	if result.FinishReason == "" {
		return nil, io.ErrUnexpectedEOF
	}
	result.Content = strings.TrimSpace(content.String())
	return result, nil
}
