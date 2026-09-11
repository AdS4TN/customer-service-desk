package runtime

import (
	"strings"

	applicationruntime "agent-desk/internal/ai/application/runtime"
	"agent-desk/internal/models"
	"agent-desk/internal/pkg/enums"
	svc "agent-desk/internal/services"
)

type replyStreamPublisher struct {
	conversation models.Conversation
	requestID    string
	raw          []rune
}

func newReplyStreamPublisher(conversation models.Conversation, requestID string) *replyStreamPublisher {
	return &replyStreamPublisher{conversation: conversation, requestID: strings.TrimSpace(requestID)}
}

func (p *replyStreamPublisher) Start() {
	svc.WsService.PublishMessageStream(&p.conversation, enums.IMRealtimeEventMessageStreamStarted, p.requestID, "")
}

func (p *replyStreamPublisher) Handle(event applicationruntime.StreamEvent) {
	if event.Type != applicationruntime.StreamEventOutput || event.Content == "" {
		return
	}
	p.raw = append(p.raw, []rune(event.Content)...)
	svc.WsService.PublishMessageStream(&p.conversation, enums.IMRealtimeEventMessageStreamDelta, p.requestID, string(p.raw))
}

func (p *replyStreamPublisher) Complete(content string) {
	content = strings.TrimSpace(content)
	svc.WsService.PublishMessageStream(&p.conversation, enums.IMRealtimeEventMessageStreamCompleted, p.requestID, content)
}

func (p *replyStreamPublisher) Fail() {
	svc.WsService.PublishMessageStream(&p.conversation, enums.IMRealtimeEventMessageStreamFailed, p.requestID, "")
}
