// Package migration imports panel backups exported by other proxy panels into
// the current installation. It never replaces the live database: it parses a
// snapshot into a vendor-neutral representation, then appends users, listeners
// and bindings in one transaction.
package migration

import "time"

// Source identifies which panel produced the uploaded backup.
type Source string

const (
	SourceUnknown Source = ""
	SourceSelf    Source = "self"
	SourceXUI     Source = "x-ui"
	SourceSUI     Source = "s-ui"
)

// User is a client/subscription holder extracted from a foreign backup.
type User struct {
	// Ref is a stable key inside this snapshot (source email or client name).
	Ref string `json:"-"`
	// Username is the candidate login name; collisions are resolved at import.
	Username string `json:"username"`
	// UUID carries VLESS/VMess credentials; Password carries Trojan/SS/QUIC ones.
	UUID     string `json:"uuid,omitempty"`
	Password string `json:"password,omitempty"`
	// Flow is the XTLS flow hint for VLESS clients.
	Flow string `json:"flow,omitempty"`

	TrafficLimit  int64      `json:"traffic_limit"`
	UploadBytes   int64      `json:"upload_bytes"`
	DownloadBytes int64      `json:"download_bytes"`
	IPLimit       int        `json:"ip_limit"`
	ExpireAt      *time.Time `json:"expire_at,omitempty"`
	Enabled       bool       `json:"enabled"`
	Remark        string     `json:"remark,omitempty"`
	Group         string     `json:"group,omitempty"`
	TelegramID    int64      `json:"telegram_id,omitempty"`
	// SourceSubID is kept only for the report: subscription URLs are regenerated.
	SourceSubID string `json:"-"`
	// numericID is the source row id, used internally to rebuild split-layout
	// bindings. It is never serialised.
	numericID int64
}

// Node is a listener/endpoint extracted from a foreign backup.
type Node struct {
	Ref      string `json:"-"`
	Name     string `json:"name"`
	Protocol string `json:"protocol"`
	Port     string `json:"port"`
	Listen   string `json:"listen,omitempty"`
	// Config is a Mihomo-schema JSON fragment (never contains users; bindings
	// inject credentials at config-generation time).
	Config string `json:"config,omitempty"`
	TLS    bool   `json:"tls,omitempty"`
	UDP    bool   `json:"udp,omitempty"`

	PublicHost        string `json:"public_host,omitempty"`
	AccessSNI         string `json:"access_sni,omitempty"`
	AccessALPN        string `json:"access_alpn,omitempty"`
	ClientFingerprint string `json:"client_fingerprint,omitempty"`

	// SourceProtocol records the original protocol name for the report.
	SourceProtocol string `json:"source_protocol,omitempty"`
	// Notes lists per-node caveats the operator must review before enabling.
	Notes []string `json:"notes,omitempty"`
}

// Binding links a user to a node inside this snapshot.
type Binding struct {
	UserRef string `json:"user_ref"`
	NodeRef string `json:"node_ref"`
}

// Snapshot is the vendor-neutral result of parsing a foreign backup.
type Snapshot struct {
	Source   Source    `json:"source"`
	Users    []User    `json:"users"`
	Nodes    []Node    `json:"nodes"`
	Bindings []Binding `json:"bindings"`
	// Warnings are non-fatal caveats about fields that cannot be reproduced.
	Warnings []string `json:"warnings,omitempty"`
	// Skipped records entries that were deliberately not imported, with a reason.
	Skipped []string `json:"skipped,omitempty"`
	// Tables lists the source tables found, for diagnostics.
	Tables []string `json:"tables,omitempty"`
}

// Counts summarises what a preview or import would do.
type Counts struct {
	Users    int `json:"users"`
	Nodes    int `json:"nodes"`
	Bindings int `json:"bindings"`
	Warnings int `json:"warnings"`
	Skipped  int `json:"skipped"`
}

func (s *Snapshot) Counts() Counts {
	return Counts{
		Users:    len(s.Users),
		Nodes:    len(s.Nodes),
		Bindings: len(s.Bindings),
		Warnings: len(s.Warnings),
		Skipped:  len(s.Skipped),
	}
}

// Preview is returned by the dry-run endpoint; it never writes anything.
type Preview struct {
	Source   Source   `json:"source"`
	Counts   Counts   `json:"counts"`
	Users    []User   `json:"users"`
	Nodes    []Node   `json:"nodes"`
	Warnings []string `json:"warnings,omitempty"`
	Skipped  []string `json:"skipped,omitempty"`
	Tables   []string `json:"tables,omitempty"`
	// BackupFile is empty for a preview; set on a real import.
	BackupFile string `json:"backup_file,omitempty"`
}

// Result is returned after an import. Renamed lists collision resolutions.
type Result struct {
	Source          Source   `json:"source"`
	UsersCreated    int      `json:"users_created"`
	NodesCreated    int      `json:"nodes_created"`
	BindingsCreated int      `json:"bindings_created"`
	Renamed         []string `json:"renamed,omitempty"`
	Skipped         []string `json:"skipped,omitempty"`
	Warnings        []string `json:"warnings,omitempty"`
	BackupFile      string   `json:"backup_file,omitempty"`
}
