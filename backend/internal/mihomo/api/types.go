package api

type Traffic struct {
	Up   int64 `json:"up"`
	Down int64 `json:"down"`
}

type Connections struct {
	DownloadTotal int64        `json:"downloadTotal"`
	UploadTotal   int64        `json:"uploadTotal"`
	Connections   []Connection `json:"connections"`
}

// ConnectionMetadata mirrors the "metadata" object Mihomo's external
// controller attaches to each entry in GET /connections. Field names match
// Mihomo's own JSON keys so this decodes the real API response directly.
type ConnectionMetadata struct {
	Network         string `json:"network"`
	Type            string `json:"type"`
	SourceIP        string `json:"sourceIP"`
	DestinationIP   string `json:"destinationIP"`
	SourcePort      string `json:"sourcePort"`
	DestinationPort string `json:"destinationPort"`
	Host            string `json:"host"`
	// InboundName is the Mihomo listener "name" (see mihomo config
	// "listeners[].name") the connection came in on. This is the join key
	// back to our Listener model.
	InboundName string `json:"inboundName"`
	// InboundUser, when Mihomo populates it, is the authenticated username
	// for the inbound connection. When present it is the only reliable,
	// non-guessed way to attribute a connection to a specific ProxyUser.
	InboundUser string `json:"inboundUser"`
}

type Connection struct {
	ID       string `json:"id"`
	Network  string `json:"network"` // deprecated: Mihomo reports network under Metadata.Network; kept for backward compatibility
	Upload   int64  `json:"upload"`
	Download int64  `json:"download"`
	// Metadata carries the real per-connection detail Mihomo returns
	// (source/destination/host/inbound). May be nil for older/mocked
	// responses that don't include it.
	Metadata *ConnectionMetadata `json:"metadata,omitempty"`
	Chains   []string            `json:"chains,omitempty"`
	Rule     string              `json:"rule,omitempty"`
	Start    string              `json:"start,omitempty"`
}
