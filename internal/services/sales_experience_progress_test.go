package services

import (
	"context"
	"testing"
	"time"

	"agent-desk/internal/ai"
	"agent-desk/internal/models"
	"agent-desk/internal/pkg/dto/request"
	sx "agent-desk/internal/pkg/salesexperience"
)

func TestSalesExperienceProgressCancelAndResume(t *testing.T) {
	db, s := setupExperience(t)
	snapshot := sx.Snapshot{Messages: []sx.Message{{ID: 1, Role: "buyer", Type: "text", Text: "Price?"}, {ID: 2, Role: "seller", Type: "text", Text: "Budget?"}}}
	c := models.SalesExperienceCase{Snapshot: sx.Encode(snapshot), SourceHash: sx.Hash(snapshot), Outcome: "unknown"}
	if err := db.Create(&c).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&models.AIConfig{}).Where("id = ?", 1).Update("timeout_ms", 1).Error; err != nil {
		t.Fatal(err)
	}
	job, err := s.Distill(request.DistillSalesExperience{CaseIDs: []int64{c.ID}, ModelConfigID: 1}, 1)
	if err != nil {
		t.Fatal(err)
	}
	started := make(chan bool, 1)
	s.stream = func(ctx context.Context, cfg models.AIConfig, _, _ string, progress func(ai.ChatProgress)) (*ai.ChatCompletionResult, error) {
		_, deadline := ctx.Deadline()
		started <- !deadline && cfg.TimeoutMS == 0 && cfg.MaxRetryCount == 0
		progress(ai.ChatProgress{OutputChars: 31, LastResponseAt: time.Now(), Phase: "generating"})
		<-ctx.Done()
		return nil, ctx.Err()
	}
	done := make(chan struct{})
	go func() { defer close(done); s.ProcessPending() }()
	t.Cleanup(func() { _ = s.Cancel(job.ID); <-done })
	select {
	case ok := <-started:
		if !ok {
			t.Fatal("background extraction still has a deadline")
		}
	case <-time.After(3 * time.Second):
		t.Fatal("worker did not start")
	}
	deadline := time.Now().Add(5 * time.Second)
	for {
		row, e := s.Job(job.ID)
		if e != nil {
			t.Fatal(e)
		}
		if row.OutputChars == 31 {
			if row.HeartbeatAt == nil || row.LastResponseAt == nil || row.CallStartedAt == nil || row.CallPhase != "generating" || row.State != "running" || row.Stage != "case:1/1:window:1/1" {
				t.Fatal("missing real progress")
			}
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("worker heartbeat not saved")
		}
		time.Sleep(25 * time.Millisecond)
	}
	if err := s.Cancel(job.ID); err != nil {
		t.Fatal(err)
	}
	row, _ := s.Job(job.ID)
	if row.State != "cancelled" {
		t.Fatal("cancellation not persisted")
	}
	// Immediate resume must not let the cancelled attempt overwrite the queue.
	if err := s.Retry(job.ID); err != nil {
		t.Fatal(err)
	}
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("cancellation did not interrupt model")
	}
	row, _ = s.Job(job.ID)
	if row.State != "queued" || row.HeartbeatAt != nil {
		t.Fatal("cancelled attempt overwrote retry")
	}
	s.stream = func(ctx context.Context, cfg models.AIConfig, _, _ string, progress func(ai.ChatProgress)) (*ai.ChatCompletionResult, error) {
		return &ai.ChatCompletionResult{Content: `{"summary":"客户询价，尚无可提炼事件。","events":[]}`, FinishReason: "stop"}, nil
	}
	s.ProcessPending()
	row, _ = s.Job(job.ID)
	if row.State != "succeeded" || row.Attempts != 2 {
		t.Fatal("resume failed")
	}
	if err := s.Cancel(job.ID); err == nil {
		t.Fatal("completed task cancelled")
	}
	var revisions, outboxes int64
	db.Model(&models.SalesExperienceRevision{}).Count(&revisions)
	db.Model(&models.ChannelMessageOutbox{}).Count(&outboxes)
	if revisions != 3 || outboxes != 0 {
		t.Fatal("cancel/resume created unexpected revisions or sends")
	}
}

func TestSalesExperienceInteractiveTimeoutUnchanged(t *testing.T) {
	_, s := setupExperience(t)
	s.complete = func(ctx context.Context, cfg models.AIConfig, _, _ string) (*ai.ChatCompletionResult, error) {
		if _, ok := ctx.Deadline(); !ok || cfg.TimeoutMS != 10 {
			t.Error("interactive deadline changed")
		}
		<-ctx.Done()
		return nil, ctx.Err()
	}
	if _, err := s.call(models.AIConfig{TimeoutMS: 10}, "", nil); err == nil || err.Error() != "timeout" {
		t.Fatal("interactive timeout not honored")
	}
}

func TestSalesExperienceCancelQueued(t *testing.T) {
	_, s := setupExperience(t)
	job, err := s.enqueue("distill", sx.ExperimentInput{}, 1, 1)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Cancel(job.ID); err != nil {
		t.Fatal(err)
	}
	s.stream = func(context.Context, models.AIConfig, string, string, func(ai.ChatProgress)) (*ai.ChatCompletionResult, error) {
		t.Error("cancelled job ran")
		return nil, nil
	}
	s.ProcessPending()
	row, _ := s.Job(job.ID)
	if row.State != "cancelled" || row.Attempts != 0 {
		t.Fatal("queued cancellation failed")
	}
}
