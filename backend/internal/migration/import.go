package migration

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"github.com/kazeyukiro/3m-ui/backend/internal/database/models"
	"github.com/kazeyukiro/3m-ui/backend/internal/security"
	"gorm.io/gorm"
)

// Options controls an import. Backup is invoked before the transaction starts;
// if it fails the import is aborted and nothing is written.
type Options struct {
	// Backup, when set, must produce a snapshot of the current database and
	// return a display name for it. It is called at most once.
	Backup func() (string, error)
	// ConflictSuffix is appended on name/username/uuid collisions.
	ConflictSuffix string
}

// Import appends the snapshot into the live database. Existing rows are never
// modified or deleted. All imported listeners are disabled.
func Import(db *gorm.DB, snap *Snapshot, opts Options) (*Result, error) {
	if db == nil {
		return nil, fmt.Errorf("database is not initialised")
	}
	if snap == nil || len(snap.Users) == 0 && len(snap.Nodes) == 0 {
		return nil, fmt.Errorf("backup contains no users or listeners to import")
	}
	suffix := strings.TrimSpace(opts.ConflictSuffix)
	if suffix == "" {
		suffix = "_imported"
	}

	res := &Result{Source: snap.Source}
	if opts.Backup != nil {
		name, err := opts.Backup()
		if err != nil {
			return nil, fmt.Errorf("pre-import backup failed; nothing was imported: %w", err)
		}
		res.BackupFile = name
	}

	// Resolve collisions before the transaction so the operation is atomic and
	// short. Unscoped queries include soft-deleted rows, which still occupy the
	// unique indexes.
	users, userRenames, err := resolveUsers(db, snap.Users, suffix)
	if err != nil {
		return nil, err
	}
	nodes, nodeRenames, err := resolveNodes(db, snap.Nodes, suffix)
	if err != nil {
		return nil, err
	}
	res.Renamed = append(res.Renamed, userRenames...)
	res.Renamed = append(res.Renamed, nodeRenames...)

	refToUser := make(map[string]uint, len(users))
	for i := range users {
		refToUser[snap.Users[i].Ref] = users[i].ID
	}
	refToNode := make(map[string]uint, len(nodes))
	for i := range nodes {
		refToNode[snap.Nodes[i].Ref] = nodes[i].ID
	}

	err = db.Transaction(func(tx *gorm.DB) error {
		for i := range users {
			if err := tx.Create(&users[i]).Error; err != nil {
				return fmt.Errorf("create user %q: %w", users[i].Username, err)
			}
			refToUser[snap.Users[i].Ref] = users[i].ID
		}
		for i := range nodes {
			if err := tx.Create(&nodes[i]).Error; err != nil {
				return fmt.Errorf("create listener %q: %w", nodes[i].Name, err)
			}
			refToNode[snap.Nodes[i].Ref] = nodes[i].ID
		}
		seen := make(map[string]struct{}, len(snap.Bindings))
		for _, b := range snap.Bindings {
			uid, uok := refToUser[b.UserRef]
			nid, nok := refToNode[b.NodeRef]
			if !uok || !nok || uid == 0 || nid == 0 {
				continue
			}
			key := fmt.Sprintf("%d:%d", nid, uid)
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}
			link := models.ListenerUser{ListenerID: nid, ProxyUserID: uid}
			if err := tx.Create(&link).Error; err != nil {
				return fmt.Errorf("bind user %d to listener %d: %w", uid, nid, err)
			}
			res.BindingsCreated++
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	res.UsersCreated = len(users)
	res.NodesCreated = len(nodes)
	res.Skipped = append([]string{}, snap.Skipped...)
	res.Warnings = append([]string{}, snap.Warnings...)
	return res, nil
}

func existingUsernames(db *gorm.DB) (map[string]struct{}, error) {
	var names []string
	if err := db.Unscoped().Model(&models.ProxyUser{}).Pluck("username", &names).Error; err != nil {
		return nil, err
	}
	out := make(map[string]struct{}, len(names))
	for _, n := range names {
		out[strings.ToLower(n)] = struct{}{}
	}
	return out, nil
}

func existingUUIDs(db *gorm.DB) (map[string]struct{}, error) {
	var ids []string
	if err := db.Unscoped().Model(&models.ProxyUser{}).Pluck("uuid", &ids).Error; err != nil {
		return nil, err
	}
	out := make(map[string]struct{}, len(ids))
	for _, id := range ids {
		if id != "" {
			out[strings.ToLower(id)] = struct{}{}
		}
	}
	return out, nil
}

func existingNodeNames(db *gorm.DB) (map[string]struct{}, error) {
	var names []string
	if err := db.Unscoped().Model(&models.Listener{}).Pluck("name", &names).Error; err != nil {
		return nil, err
	}
	out := make(map[string]struct{}, len(names))
	for _, n := range names {
		out[strings.ToLower(n)] = struct{}{}
	}
	return out, nil
}

func uniqueName(base string, taken map[string]struct{}, suffix string) (string, bool) {
	base = strings.TrimSpace(base)
	if base == "" {
		base = "imported"
	}
	key := strings.ToLower(base)
	if _, ok := taken[key]; !ok {
		taken[key] = struct{}{}
		return base, false
	}
	for i := 0; i < 1000; i++ {
		candidate := base + suffix
		if i > 0 {
			candidate = fmt.Sprintf("%s%s%d", base, suffix, i+1)
		}
		if _, ok := taken[strings.ToLower(candidate)]; !ok {
			taken[strings.ToLower(candidate)] = struct{}{}
			return candidate, true
		}
	}
	return "", false
}

func resolveUsers(db *gorm.DB, src []User, suffix string) ([]models.ProxyUser, []string, error) {
	names, err := existingUsernames(db)
	if err != nil {
		return nil, nil, fmt.Errorf("read existing users: %w", err)
	}
	uuids, err := existingUUIDs(db)
	if err != nil {
		return nil, nil, fmt.Errorf("read existing credentials: %w", err)
	}
	out := make([]models.ProxyUser, 0, len(src))
	var renamed []string
	for _, u := range src {
		name, changed := uniqueName(u.Username, names, suffix)
		if name == "" {
			return nil, nil, fmt.Errorf("could not allocate a unique name for %q", u.Username)
		}
		if changed {
			renamed = append(renamed, fmt.Sprintf("user %q -> %q", u.Username, name))
		}
		uuid := strings.TrimSpace(u.UUID)
		if uuid == "" {
			uuid, err = newUUID()
			if err != nil {
				return nil, nil, err
			}
		}
		if _, ok := uuids[strings.ToLower(uuid)]; ok {
			newID, e := newUUID()
			if e != nil {
				return nil, nil, e
			}
			renamed = append(renamed, fmt.Sprintf("user %q credential UUID regenerated (collision)", name))
			uuid = newID
		}
		uuids[strings.ToLower(uuid)] = struct{}{}

		password := u.Password
		if password == "" {
			password = uuid // keep credentials usable for password-based protocols
		}
		encrypted, err := security.Encrypt(password)
		if err != nil {
			return nil, nil, fmt.Errorf("encrypt credential for %q: %w", name, err)
		}
		subToken, err := newToken(16)
		if err != nil {
			return nil, nil, err
		}
		expire := time.Time{}
		if u.ExpireAt != nil {
			expire = u.ExpireAt.UTC()
		}
		row := models.ProxyUser{
			Username:          name,
			PasswordEncrypted: encrypted,
			UUID:              uuid,
			TrafficLimit:      u.TrafficLimit,
			TrafficUsed:       u.UploadBytes + u.DownloadBytes,
			UploadBytes:       u.UploadBytes,
			DownloadBytes:     u.DownloadBytes,
			IPLimit:           u.IPLimit,
			ExpireTime:        expire,
			Enabled:           u.Enabled,
			Remark:            strings.TrimSpace(u.Remark),
			Group:             strings.TrimSpace(u.Group),
			Tags:              "migration",
			TelegramID:        u.TelegramID,
			SubToken:          subToken,
		}
		out = append(out, row)
	}
	return out, renamed, nil
}

func resolveNodes(db *gorm.DB, src []Node, suffix string) ([]models.Listener, []string, error) {
	names, err := existingNodeNames(db)
	if err != nil {
		return nil, nil, fmt.Errorf("read existing listeners: %w", err)
	}
	out := make([]models.Listener, 0, len(src))
	var renamed []string
	for _, n := range src {
		name, changed := uniqueName(n.Name, names, suffix)
		if name == "" {
			return nil, nil, fmt.Errorf("could not allocate a unique name for %q", n.Name)
		}
		if changed {
			renamed = append(renamed, fmt.Sprintf("listener %q -> %q", n.Name, name))
		}
		bind := "0.0.0.0"
		if strings.TrimSpace(n.Listen) != "" {
			bind = strings.TrimSpace(n.Listen)
		}
		row := models.Listener{
			Name:              name,
			Protocol:          n.Protocol,
			Type:              n.Protocol,
			Port:              strings.TrimSpace(n.Port),
			BindAddress:       bind,
			Config:            n.Config,
			TLS:               n.TLS,
			UDP:               n.UDP,
			Enabled:           false,
			Status:            "inactive",
			TrafficMultiplier: 1,
			PublicHost:        n.PublicHost,
			PublicPort:        "",
			AccessSNI:         n.AccessSNI,
			AccessALPN:        n.AccessALPN,
			ClientFingerprint: n.ClientFingerprint,
		}
		// An empty port is invalid for a listener; keep the record but flag it.
		if row.Port == "" {
			row.Port = "0"
		}
		out = append(out, row)
	}
	return out, renamed, nil
}

func newUUID() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	h := hex.EncodeToString(b)
	return fmt.Sprintf("%s-%s-%s-%s-%s", h[0:8], h[8:12], h[12:16], h[16:20], h[20:32]), nil
}
