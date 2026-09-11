package messenger

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"

	"agent-desk/internal/pkg/linkedchat"
	"go.mau.fi/mautrix-meta/pkg/messagix/table"
)

func TestCredentials(t *testing.T) {
	t.Setenv("AGENT_DESK_MESSENGER_SESSION_DIR", t.TempDir())
	for _, value := range []string{"", "xs=test", "c_user=bad; xs=test; datr=test", "c_user=1; xs=test; datr=test\r\nInjected: yes", strings.Repeat("x", 32769)} {
		if _, err := ParseCookies(value); err == nil {
			t.Fatal("accepted malformed credentials")
		}
	}
	if err := SaveCredentials(1, "c_user=123; xs=local-test-only; datr=device-test-only"); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(credentialPath(1))
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0600 {
		t.Fatal("credentials not private")
	}
	state := SavedStatus(1)
	if !state.HasCredentials || state.Account != "123" || state.State != "disconnected" {
		t.Fatal("incorrect saved state")
	}
	data, _ := json.Marshal(state)
	if strings.Contains(string(data), "local-test-only") || strings.Contains(string(data), "device-test-only") {
		t.Fatal("credential leaked through status")
	}
	if err := SaveCredentials(1, "c_user=456; xs=local-test-only; datr=device-test-only"); err == nil {
		t.Fatal("allowed silent account switch")
	}
	if err := Forget(1); err != nil {
		t.Fatal(err)
	}
	if SavedStatus(1).HasCredentials {
		t.Fatal("forgotten credential still visible")
	}
}

func TestTableImport(t *testing.T) {
	s := &Session{names: make(map[int64]string), threads: make(map[int64]table.ThreadType)}
	now := time.Now()
	tbl := &table.LSTable{
		LSDeleteThenInsertThread: []*table.LSDeleteThenInsertThread{
			{ThreadKey: 2, ThreadType: table.ONE_TO_ONE, ThreadName: "Customer"},
			{ThreadKey: 3, ThreadType: table.GROUP_THREAD},
			{ThreadKey: 4, ThreadType: table.ENCRYPTED_OVER_WA_ONE_TO_ONE},
		},
		LSInsertMessage: []*table.LSInsertMessage{
			{ThreadKey: 2, MessageId: "live", Text: "hello", TimestampMs: now.Add(time.Second).UnixMilli(), SenderId: 2},
			{ThreadKey: 2, MessageId: "old", Text: "old", TimestampMs: now.Add(-time.Hour).UnixMilli(), SenderId: 1, OfflineThreadingId: "otid"},
			{ThreadKey: 3, MessageId: "group", Text: "group"},
			{ThreadKey: 4, MessageId: "encrypted-placeholder", Text: "encrypted"},
		},
	}
	var messages []linkedchat.Incoming
	s.handleTable(tbl, now, 1, func(m linkedchat.Incoming) bool { messages = append(messages, m); return true })
	if len(messages) != 2 {
		t.Fatalf("unexpected messages: %d", len(messages))
	}
	if messages[0].History || messages[0].Name != "Customer" || messages[0].Chat != "plain:2" {
		t.Fatal("live metadata lost")
	}
	if !messages[1].History || !messages[1].FromMe || messages[1].EchoID != "otid" {
		t.Fatal("history direction/echo ID lost")
	}
}

func TestConnectRequiresCredentials(t *testing.T) {
	t.Setenv("AGENT_DESK_MESSENGER_SESSION_DIR", t.TempDir())
	s, err := Open(SessionDir(), 1, func(linkedchat.Incoming) error { return nil })
	if err != nil {
		t.Fatal(err)
	}
	defer s.store.Close()
	if s.Connect() == nil {
		t.Fatal("connected without credentials")
	}
	if s.Status().State != "disconnected" {
		t.Fatal("fake connected state")
	}
	s.Disconnect()
}
