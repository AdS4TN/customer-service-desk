package runtime

import (
	"reflect"
	"sync"
	"testing"
	"time"

	"agent-desk/internal/pkg/enums"
)

func waitReplySignal(t *testing.T, ch <-chan struct{}) {
	t.Helper()
	select {
	case <-ch:
	case <-time.After(3 * time.Second):
		t.Fatal("reply queue did not make progress")
	}
}

func TestReplyQueueSerializesEachConversationAndKeepsOtherCustomersIndependent(t *testing.T) {
	var q replyQueue
	started, release, other, finished := make(chan struct{}), make(chan struct{}), make(chan struct{}), make(chan struct{})
	var once sync.Once
	t.Cleanup(func() { once.Do(func() { close(release) }) })
	var mu sync.Mutex
	order := []int{}
	record := func(id int) { mu.Lock(); defer mu.Unlock(); order = append(order, id) }
	q.enqueue(15, 1, func() { close(started); <-release; record(1) })
	waitReplySignal(t, started)
	q.enqueue(15, 3, func() { record(3); close(finished) })
	q.enqueue(15, 2, func() { record(2) })
	q.enqueue(15, 2, func() { record(99) })
	q.enqueue(15, 1, func() { record(99) })
	q.enqueue(16, 4, func() { close(other) })
	waitReplySignal(t, other)
	mu.Lock()
	empty := len(order) == 0
	mu.Unlock()
	if !empty {
		t.Fatal("later reply ran while the first reply was active")
	}
	once.Do(func() { close(release) })
	waitReplySignal(t, finished)
	mu.Lock()
	defer mu.Unlock()
	if !reflect.DeepEqual(order, []int{1, 2, 3}) {
		t.Fatalf("wrong reply order: %v", order)
	}
}

func TestReplyQueueRecoversWorkerAndReleasesIdleConversation(t *testing.T) {
	var q replyQueue
	finished := make(chan struct{})
	q.enqueue(15, 1, func() { panic("synthetic") })
	q.enqueue(15, 2, func() { close(finished) })
	waitReplySignal(t, finished)
	deadline := time.Now().Add(3 * time.Second)
	for {
		q.mu.Lock()
		remaining := len(q.conversations)
		q.mu.Unlock()
		if remaining == 0 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("idle conversation retained by queue")
		}
		time.Sleep(time.Millisecond)
	}
	next := make(chan struct{})
	q.enqueue(15, 3, func() { close(next) })
	waitReplySignal(t, next)
}

func TestQueuedReplyEligibilityRejectsStaleOrHistoricalInputs(t *testing.T) {
	e := newReplyEligibility()
	c, m, agent := newConversationFixture(), newCustomerMessageFixture("hello"), newAIAgentFixture()
	for _, status := range []enums.IMMessageStatus{enums.IMMessageStatusSending, enums.IMMessageStatusFailed, enums.IMMessageStatusRecalled} {
		m.SendStatus = status
		if e.CanReply(c, m, agent) {
			t.Fatalf("accepted message status %d", status)
		}
	}
	m.SendStatus = enums.IMMessageStatusSent
	m.IsHistorical = true
	if e.CanReply(c, m, agent) {
		t.Fatal("imported history triggered reply")
	}
	m.IsHistorical = false
	c.Status = enums.IMConversationStatusClosed
	if e.CanReply(c, m, agent) {
		t.Fatal("closed conversation triggered reply")
	}
}
