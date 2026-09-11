package runtime

import (
	"log/slog"
	"sort"
	"sync"
)

type queuedReply struct {
	messageID int64
	run       func()
}

type conversationReplyQueue struct {
	activeID int64
	pending  []queuedReply
}

// One worker per conversation; independent customers remain concurrent.
// Jobs are not merged: a queued message may contain a handoff or confirmation.
type replyQueue struct {
	mu            sync.Mutex
	conversations map[int64]*conversationReplyQueue
}

func (q *replyQueue) enqueue(conversationID, messageID int64, run func()) {
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.conversations == nil {
		q.conversations = make(map[int64]*conversationReplyQueue)
	}
	queue, exists := q.conversations[conversationID]
	if !exists {
		queue = &conversationReplyQueue{}
		q.conversations[conversationID] = queue
	}
	if queue.activeID == messageID {
		return
	}
	index := sort.Search(len(queue.pending), func(i int) bool { return queue.pending[i].messageID >= messageID })
	if index < len(queue.pending) && queue.pending[index].messageID == messageID {
		return
	}
	queue.pending = append(queue.pending, queuedReply{})
	copy(queue.pending[index+1:], queue.pending[index:])
	queue.pending[index] = queuedReply{messageID: messageID, run: run}
	if !exists {
		go q.drain(conversationID, queue)
	}
}

func (q *replyQueue) drain(conversationID int64, queue *conversationReplyQueue) {
	for {
		q.mu.Lock()
		if len(queue.pending) == 0 {
			delete(q.conversations, conversationID)
			q.mu.Unlock()
			return
		}
		job := queue.pending[0]
		queue.pending[0] = queuedReply{}
		queue.pending = queue.pending[1:]
		queue.activeID = job.messageID
		q.mu.Unlock()
		func() {
			defer func() {
				if recover() != nil {
					slog.Error("queued ai reply panicked", "conversation_id", conversationID, "message_id", job.messageID)
				}
			}()
			job.run()
		}()
	}
}
