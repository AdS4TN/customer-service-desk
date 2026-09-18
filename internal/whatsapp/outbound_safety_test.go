package whatsapp

import (
	"testing"
)

func TestProductionAccountOutboundEnabled(t *testing.T) {
	if OutboundMessagesDisabled {
		t.Fatal("WhatsApp outbound must remain enabled after owner approval")
	}
}
