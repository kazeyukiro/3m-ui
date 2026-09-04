package certstore

import (
	"encoding/json"
	"log"
	"os"
	"strings"

	"github.com/kazeyukiro/3m-ui/backend/internal/database/models"
	"gopkg.in/yaml.v3"
	"gorm.io/gorm"
)

// HydrateListenersFromDisk copies on-disk PEMs into listener.Config when the DB
// row has no complete certificate pair. Runs at panel startup so a binary-only
// update can restore TLS identity without the operator re-saving nodes.
func HydrateListenersFromDisk(db *gorm.DB) {
	if db == nil {
		return
	}
	var list []models.Listener
	if err := db.Find(&list).Error; err != nil {
		log.Printf("[certstore] hydrate list: %v", err)
		return
	}
	n := 0
	for i := range list {
		l := &list[i]
		if l.ID == 0 {
			continue
		}
		cfg := map[string]interface{}{}
		raw := strings.TrimSpace(l.Config)
		if raw != "" {
			_ = json.Unmarshal([]byte(raw), &cfg)
		}
		if cfg == nil {
			cfg = map[string]interface{}{}
		}
		if hasCompleteCert(cfg) {
			// Keep disk mirror in sync for next update.
			c, _ := cfg["certificate"].(string)
			k, _ := cfg["private-key"].(string)
			if k == "" {
				k, _ = cfg["private_key"].(string)
			}
			_ = Save(l.ID, c, k)
			continue
		}
		c, k, ok := Load(l.ID)
		if !ok {
			continue
		}
		cfg["certificate"] = c
		cfg["private-key"] = k
		delete(cfg, "private_key")
		b, err := json.Marshal(cfg)
		if err != nil {
			continue
		}
		if err := db.Model(&models.Listener{}).Where("id = ?", l.ID).Update("config", string(b)).Error; err != nil {
			log.Printf("[certstore] hydrate listener %d: %v", l.ID, err)
			continue
		}
		n++
	}
	if n > 0 {
		log.Printf("[certstore] restored TLS material from disk for %d listener(s)", n)
	}
}

// HydrateFromMihomoYAML recovers PEMs from an existing mihomo config.yaml by
// matching listener name → id when both DB and certstore lack a pair.
// This covers installs that never wrote /var/lib/3m-ui/listener-certs.
func HydrateFromMihomoYAML(db *gorm.DB, configPath string) {
	if db == nil || strings.TrimSpace(configPath) == "" {
		return
	}
	data, err := os.ReadFile(configPath)
	if err != nil {
		return
	}
	var root struct {
		Listeners []map[string]interface{} `yaml:"listeners"`
	}
	if err := yaml.Unmarshal(data, &root); err != nil || len(root.Listeners) == 0 {
		return
	}
	byName := map[string]map[string]interface{}{}
	for _, item := range root.Listeners {
		name, _ := item["name"].(string)
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		byName[name] = item
	}
	var list []models.Listener
	if err := db.Find(&list).Error; err != nil {
		return
	}
	n := 0
	for i := range list {
		l := &list[i]
		if l.ID == 0 {
			continue
		}
		cfg := map[string]interface{}{}
		if strings.TrimSpace(l.Config) != "" {
			_ = json.Unmarshal([]byte(l.Config), &cfg)
		}
		if cfg == nil {
			cfg = map[string]interface{}{}
		}
		if hasCompleteCert(cfg) {
			continue
		}
		if _, _, ok := Load(l.ID); ok {
			continue // disk wins; HydrateListenersFromDisk handles it
		}
		item := byName[strings.TrimSpace(l.Name)]
		if item == nil {
			continue
		}
		c, _ := item["certificate"].(string)
		k, _ := item["private-key"].(string)
		if strings.TrimSpace(c) == "" || strings.TrimSpace(k) == "" {
			continue
		}
		cfg["certificate"] = c
		cfg["private-key"] = k
		delete(cfg, "private_key")
		b, err := json.Marshal(cfg)
		if err != nil {
			continue
		}
		if err := db.Model(&models.Listener{}).Where("id = ?", l.ID).Update("config", string(b)).Error; err != nil {
			continue
		}
		_ = Save(l.ID, c, k)
		n++
	}
	if n > 0 {
		log.Printf("[certstore] recovered TLS material from mihomo config.yaml for %d listener(s)", n)
	}
}

func hasCompleteCert(cfg map[string]interface{}) bool {
	if cfg == nil {
		return false
	}
	c, _ := cfg["certificate"].(string)
	k, _ := cfg["private-key"].(string)
	if k == "" {
		k, _ = cfg["private_key"].(string)
	}
	return strings.TrimSpace(c) != "" && strings.TrimSpace(k) != ""
}
