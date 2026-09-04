package models

import "time"

// RemoteNodeMirror caches shareable metadata from a remote 3m-ui panel node
// so the local panel can merge it into a user's subscription.
type RemoteNodeMirror struct {
	BaseModel

	RemoteServerID uint   `gorm:"not null;index;uniqueIndex:uidx_remote_node" json:"remote_server_id"`
	RemoteNodeID   uint   `gorm:"not null;uniqueIndex:uidx_remote_node" json:"remote_node_id"`
	Name           string `gorm:"size:128;not null" json:"name"`
	Protocol       string `gorm:"size:32" json:"protocol"`
	Port           string `gorm:"size:16" json:"port"`
	PublicHost     string `gorm:"size:255" json:"public_host"`
	Enabled        bool   `gorm:"not null;default:true" json:"enabled"`
	// ShareURI is the primary client share link from the remote panel.
	ShareURI string `gorm:"type:text" json:"share_uri,omitempty"`
	// ShareURIsJSON is a JSON array of additional share links.
	ShareURIsJSON string `gorm:"type:text" json:"-"`
	// ClientYAML is a Mihomo client proxy document snippet from the remote export.
	ClientYAML string `gorm:"type:text" json:"client_yaml,omitempty"`
	LastSyncAt *time.Time `json:"last_sync_at"`
	LastError  string     `gorm:"type:text" json:"last_error,omitempty"`

	// Joined for API responses (not persisted).
	RemoteServerName string `gorm:"-" json:"remote_server_name,omitempty"`
}

// ProxyUserRemoteNode binds a local proxy user to a mirrored remote node.
type ProxyUserRemoteNode struct {
	BaseModel
	ProxyUserID      uint `gorm:"not null;uniqueIndex:uidx_user_remote_node" json:"proxy_user_id"`
	RemoteNodeMirrorID uint `gorm:"not null;uniqueIndex:uidx_user_remote_node;index" json:"remote_node_mirror_id"`
}
