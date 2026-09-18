package linkedchat

import "time"

// Media bytes stay server-side and are uploaded by the channel's official client library.
type OutboundMedia struct {
	Data     []byte
	Filename string
	MimeType string
	Image    bool
}

type Status struct {
	State          string     `json:"state"`
	Account        string     `json:"account"`
	QR             string     `json:"qr,omitempty"`
	QRExpiresAt    *time.Time `json:"qrExpiresAt,omitempty"`
	Error          string     `json:"error,omitempty"`
	HasCredentials bool       `json:"hasCredentials,omitempty"`
	EncryptedState string     `json:"encryptedState,omitempty"`
}

type Incoming struct {
	ID, Account, Chat, Name, Text string
	EchoID                        string
	SentAt                        time.Time
	History, FromMe               bool
	ChatAliases                   []string
	Message                       *Message
	MediaPath                     string // Temporary decrypted file; never serialized or logged.
}

// Message contains display metadata only, never upstream download URLs or keys.
type Message struct {
	Kind          string   `json:"kind"`
	State         string   `json:"state,omitempty"`
	Filename      string   `json:"filename,omitempty"`
	MimeType      string   `json:"mimeType,omitempty"`
	Size          uint64   `json:"size,omitempty"`
	Seconds       uint32   `json:"seconds,omitempty"`
	TargetID      string   `json:"targetId,omitempty"`
	TargetPreview string   `json:"targetPreview,omitempty"`
	RawType       string   `json:"rawType,omitempty"`
	Title         string   `json:"title,omitempty"`
	Footer        string   `json:"footer,omitempty"`
	URL           string   `json:"url,omitempty"`
	MediaKind     string   `json:"mediaKind,omitempty"`
	Options       []Option `json:"options,omitempty"`
	Fields        []Field  `json:"fields,omitempty"`
	StartAt       int64    `json:"startAt,omitempty"`
	EndAt         int64    `json:"endAt,omitempty"`
	Selectable    uint32   `json:"selectable,omitempty"`
	Forwarded     bool     `json:"forwarded,omitempty"`
	Update        string   `json:"update,omitempty"`
	RevisionMS    int64    `json:"revisionMs,omitempty"`
}

type Option struct {
	ID          string `json:"id,omitempty"`
	Label       string `json:"label"`
	Description string `json:"description,omitempty"`
	Count       *int64 `json:"count,omitempty"`
}

type Field struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

// These events describe existing activity, not a new request for an agent.
func (m *Message) Passive() bool {
	if m == nil {
		return false
	}
	return m.Update != "" || m.Kind == "reaction" || m.Kind == "poll_vote" || m.Kind == "poll_update" || m.Kind == "event_response" || m.Kind == "notice" || m.Kind == "revoked"
}

func (m *Message) Textual() bool {
	return m == nil || ((m.Kind == "text" || m.Kind == "reply") && m.Update == "" && m.State == "")
}
