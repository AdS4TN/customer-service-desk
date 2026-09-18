package whatsapp

import "errors"

// Emergency hold. Keep false unless the owner explicitly requests another send freeze.
const OutboundMessagesDisabled = false

var ErrOutboundDisabled = errors.New("whatsapp outbound messaging is disabled: production account is receive-only")
