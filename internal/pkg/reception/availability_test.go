package reception

import (
	"agent-desk/internal/models"
	"agent-desk/internal/pkg/enums"
	"testing"
)

func TestAutomaticMessageModes(t *testing.T) {
	for _, aMode := range []enums.IMConversationServiceMode{enums.IMConversationServiceModeAIOnly, enums.IMConversationServiceModeAIFirst, enums.IMConversationServiceModeHumanOnly} {
		for _, cMode := range []enums.IMConversationServiceMode{enums.IMConversationServiceModeAIOnly, enums.IMConversationServiceModeAIFirst, enums.IMConversationServiceModeHumanOnly} {
			a := &models.AIAgent{Status: enums.StatusOk, ServiceMode: aMode}
			c := &models.Conversation{ServiceMode: cMode}
			want := aMode != enums.IMConversationServiceModeHumanOnly && cMode != enums.IMConversationServiceModeHumanOnly
			if AutomaticMessagesAllowed(c, a) != want {
				t.Fatal("mode mismatch")
			}
			a.Status = enums.StatusDisabled
			if AutomaticMessagesAllowed(c, a) {
				t.Fatal("disabled agent allowed")
			}
			a.Status = enums.StatusOk
			c.Status = enums.IMConversationStatusClosed
			if AutomaticMessagesAllowed(c, a) {
				t.Fatal("closed conversation allowed")
			}
		}
	}
}
