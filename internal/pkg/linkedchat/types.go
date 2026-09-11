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
	Kind          string `json:"kind"`
	State         string `json:"state,omitempty"`
	Filename      string `json:"filename,omitempty"`
	MimeType      string `json:"mimeType,omitempty"`
	Size          uint64 `json:"size,omitempty"`
	Seconds       uint32 `json:"seconds,omitempty"`
	TargetID      string `json:"targetId,omitempty"`
	TargetPreview string `json:"targetPreview,omitempty"`
}
