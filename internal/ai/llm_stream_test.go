package ai

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"agent-desk/internal/models"
)

func TestChatWithProgress(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body["stream"] != true {
			t.Error("missing streaming request")
		}
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, "data: {\"choices\":[{\"index\":0,\"delta\":{\"reasoning_content\":\"private reasoning\"}}]}\n\n")
		fmt.Fprint(w, "data: {\"choices\":[{\"index\":0,\"delta\":{\"content\":\"中文\"}}]}\n\n")
		fmt.Fprint(w, "data: {\"choices\":[{\"index\":0,\"delta\":{\"content\":\" Skill\"},\"finish_reason\":\"stop\"}]}\n\ndata: [DONE]\n\n")
	}))
	defer server.Close()
	var progress []ChatProgress
	result, err := LLM.ChatWithProgress(context.Background(), models.AIConfig{BaseURL: server.URL, ModelName: "synthetic"}, "system", "input", func(p ChatProgress) { progress = append(progress, p) })
	if err != nil {
		t.Fatal(err)
	}
	if result.Content != "中文 Skill" || result.FinishReason != "stop" || len(progress) != 3 {
		t.Fatalf("incorrect stream result: %+v", result)
	}
	if progress[0].Phase != "thinking" || progress[0].OutputChars != 0 || progress[2].OutputChars != 8 || progress[2].Phase != "generating" {
		t.Fatalf("incorrect activity counters: %+v", progress)
	}
	encoded, _ := json.Marshal(progress)
	if strings.Contains(string(encoded), "private reasoning") || progress[0].LastResponseAt.IsZero() {
		t.Fatal("reasoning leaked or activity missing")
	}
}

func TestChatWithProgressRejectsIncompleteAndUpstreamFailure(t *testing.T) {
	for _, status := range []int{http.StatusOK, http.StatusBadRequest} {
		t.Run(fmt.Sprint(status), func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "text/event-stream")
				w.WriteHeader(status)
				fmt.Fprint(w, "data: {\"choices\":[{\"index\":0,\"delta\":{\"content\":\"partial\"}}]}\n\n")
			}))
			defer server.Close()
			_, err := LLM.ChatWithProgress(context.Background(), models.AIConfig{BaseURL: server.URL}, "", "", nil)
			if err == nil || (status == http.StatusOK && !errors.Is(err, io.ErrUnexpectedEOF)) {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestChatWithProgressCancellation(t *testing.T) {
	started := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		w.(http.Flusher).Flush()
		close(started)
		<-r.Context().Done()
	}))
	defer server.Close()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() {
		_, err := LLM.ChatWithProgress(ctx, models.AIConfig{BaseURL: server.URL}, "", "", nil)
		done <- err
	}()
	select {
	case <-started:
	case <-time.After(3 * time.Second):
		t.Fatal("stream did not open")
	}
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("unexpected cancellation error: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("stream did not stop")
	}
}
