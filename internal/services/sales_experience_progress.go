package services

import (
	"context"
	"errors"
	"strings"
	"sync"
	"time"

	"agent-desk/internal/ai"
	"agent-desk/internal/models"
	sx "agent-desk/internal/pkg/salesexperience"
	"github.com/mlogclub/simple/sqls"
)

func (s *salesExperienceService) callDistill(job *models.SalesExperienceJob, cfg models.AIConfig, prompt string, input any) (string, error) {
	// Background extraction has no fixed completion deadline. Cancellation comes
	// from the task state; interactive replies keep their existing timeout policy.
	cfg.TimeoutMS, cfg.MaxRetryCount = 0, 0
	ctx, cancel := context.WithCancelCause(context.Background())
	defer cancel(nil)
	now := time.Now()
	if err := experienceRepo.UpdateRunningJob(sqls.DB(), job.ID, map[string]any{"call_started_at": now, "heartbeat_at": now, "last_response_at": nil, "output_chars": 0, "call_phase": "waiting"}); err != nil {
		return "", err
	}
	var mu sync.Mutex
	progress := ai.ChatProgress{Phase: "waiting"}
	update := func() error {
		mu.Lock()
		p := progress
		mu.Unlock()
		fields := map[string]any{"heartbeat_at": time.Now(), "output_chars": p.OutputChars, "call_phase": p.Phase}
		if !p.LastResponseAt.IsZero() {
			fields["last_response_at"] = p.LastResponseAt
		}
		return experienceRepo.UpdateRunningJob(sqls.DB(), job.ID, fields)
	}
	finished, stopped := make(chan struct{}), make(chan struct{})
	go func() {
		defer close(stopped)
		ticker := time.NewTicker(2 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-finished:
				return
			case <-ticker.C:
				if err := update(); err != nil {
					cancel(err)
					return
				}
			}
		}
	}()
	var result *ai.ChatCompletionResult
	var err error
	if s.stream != nil {
		result, err = s.stream(ctx, cfg, prompt, sx.Encode(input), func(p ai.ChatProgress) { mu.Lock(); progress = p; mu.Unlock() })
	} else {
		result, err = s.complete(ctx, cfg, prompt, sx.Encode(input))
	}
	close(finished)
	<-stopped
	if cause := context.Cause(ctx); cause != nil {
		return "", cause
	}
	if e := update(); e != nil {
		return "", e
	}
	if err != nil {
		return "", errors.New("modelCall")
	}
	if result == nil || strings.TrimSpace(result.Content) == "" {
		return "", errors.New("format")
	}
	if result.FinishReason == "length" {
		return "", errors.New("truncated")
	}
	if err := experienceRepo.UpdateRunningJob(sqls.DB(), job.ID, map[string]any{"call_phase": "validating"}); err != nil {
		return "", err
	}
	return result.Content, nil
}
