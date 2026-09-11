package services

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"agent-desk/internal/models"
	"agent-desk/internal/repositories"
	"github.com/mlogclub/simple/sqls"
)

const MemoryMaxAttempts = 3

// Only allowlisted codes cross the log/API boundary; provider errors may contain secrets.
type memoryStageError struct {
	code  string
	cause error
}

func (e *memoryStageError) Error() string { return e.code }
func (e *memoryStageError) Unwrap() error { return e.cause }

func memoryFailureCode(err error) string {
	if errors.Is(err, errMemoryConfig) {
		return "model_unavailable"
	}
	var stage *memoryStageError
	if errors.As(err, &stage) {
		if stage.code == "model_request_failed" && errors.Is(err, context.DeadlineExceeded) {
			return "model_timeout"
		}
		return stage.code
	}
	return "extraction_failed"
}

func (s *conversationMemoryService) finishFailure(job *models.ConversationMemory, err error, elapsed time.Duration) {
	code := memoryFailureCode(err)
	retry := code != "model_unavailable" && code != "worker_interrupted" && job.AttemptCount < MemoryMaxAttempts
	r := repositories.ConversationMemoryRepository
	var saved bool
	var saveErr error
	if retry {
		delay := 10 * time.Second
		if job.AttemptCount >= 2 {
			delay = 30 * time.Second
		}
		saved, saveErr = r.Retry(sqls.DB(), job, code, time.Now().Add(delay))
	} else {
		saved, saveErr = r.Finish(sqls.DB(), job, job.ProcessedMessageID, "failed", code)
	}
	slog.Warn("conversation analysis failed", "conversation_id", job.ConversationID, "revision", job.Revision, "attempt", job.AttemptCount, "stage", code, "duration_ms", elapsed.Milliseconds(), "retry_scheduled", retry && saved, "superseded", !saved && saveErr == nil, "state_write_failed", saveErr != nil)
}

func memoryRetryHint(code string) string {
	switch code {
	case "memory_validation_failed":
		return "\nA previous attempt failed memory validation. Return both entries and leads arrays, with valid kinds and actual source IDs. EVERY customer_tag/customer_profile entry, including unchanged previous entries, needs evidence containing an EXACT quote from a cited customer message; use previousCustomerEvidence for earlier messages. Keep fieldKey empty for tags/profiles and do not return duplicate categories/values or profile topics."
	case "inquiry_validation_failed":
		return "\nA previous attempt failed inquiry validation. Preserve each existing key's kind, topic and fieldKey EXACTLY; update its value/label or create a new key for a different fact. Only use configured field keys, once per purchase topic."
	case "lead_validation_failed":
		return "\nA previous attempt failed lead validation. Return the leads array, use only actual CUSTOMER source IDs, and quote intentEvidence/email/phone exactly from their messages. Omit unsupported facts."
	default:
		return ""
	}
}
