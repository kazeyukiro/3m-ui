package migration

import (
	"database/sql"
	"fmt"
	"strings"
)

// parseSUI reads a sing-box based panel database. Clients store a per-protocol
// config map and an explicit list of inbound ids; inbounds store a type plus
// an options JSON blob; TLS/REALITY material lives in a separate tls table.
func parseSUI(db *sql.DB, s schema) (*Snapshot, error) {
	snap := &Snapshot{Source: SourceSUI, Tables: s.tableNames()}

	tlsByID := loadSUITLS(db, s)

	inboundRows, err := queryRows(db, `SELECT * FROM inbounds`)
	if err != nil {
		return nil, fmt.Errorf("read inbounds: %w", err)
	}
	nodeByID := make(map[int64]string)
	for _, row := range inboundRows {
		id := integer(row["id"])
		srcType := str(row["type"])
		proto, supported := protocolMap(srcType)
		name := firstNonEmpty(str(row["tag"]), fmt.Sprintf("inbound-%d-%s", id, srcType))

		opts := decodeObject(row["options"])
		if opts == nil {
			opts = decodeObject(row["out_json"])
		}
		addrs := decodeObject(row["addrs"])
		port := firstNonEmpty(str(opts["listen_port"]), str(addrs["listen_port"]), str(row["port"]))
		listen := suiListen(firstNonEmpty(str(opts["listen"]), str(addrs["listen"])))

		if !supported {
			snap.Skipped = append(snap.Skipped,
				fmt.Sprintf("inbound %q: type %q has no Mihomo listener equivalent", name, srcType))
			continue
		}
		if port == "" {
			snap.Skipped = append(snap.Skipped,
				fmt.Sprintf("inbound %q (%s): source has no usable port", name, srcType))
			continue
		}

		tlsID := integer(row["tls_id"])
		tls := tlsByID[tlsID]
		cfg, notes := suiInboundConfig(proto, srcType, opts, tls)

		encoded, err := encodeConfig(cfg)
		if err != nil {
			notes = append(notes, "listener config could not be encoded: "+err.Error())
			encoded = ""
		}

		node := Node{
			Ref:            fmt.Sprintf("inbound:%d", id),
			Name:           name,
			Protocol:       proto,
			Port:           port,
			Listen:         listen,
			Config:         encoded,
			SourceProtocol: srcType,
			Notes:          notes,
		}
		if tls != nil {
			node.AccessSNI = firstNonEmpty(str(tls["server_name"]), str(tls["sni"]))
			if alpn, ok := tls["alpn"].([]interface{}); ok && len(alpn) > 0 {
				node.AccessALPN = strings.Join(suiStrings(alpn), ",")
			}
			node.TLS = truthy(tls["enabled"]) && !truthy(nestedObject(tls, "reality")["enabled"])
		}
		snap.Nodes = append(snap.Nodes, node)
		nodeByID[id] = node.Ref
	}

	clientRows, err := queryRows(db, `SELECT * FROM clients`)
	if err != nil {
		return nil, fmt.Errorf("read clients: %w", err)
	}
	for _, row := range clientRows {
		cid := integer(row["id"])
		name := firstNonEmpty(str(row["name"]), fmt.Sprintf("client-%d", cid))
		cfgMap := decodeObject(row["config"])
		u := suiClient(row, cfgMap, name, cid)

		// Traffic and quota live on the client row itself.
		u.UploadBytes = integer(row["up"])
		u.DownloadBytes = integer(row["down"])
		u.TrafficLimit = integer(row["volume"])
		u.ExpireAt = timeFromSeconds(integer(row["expiry"]))
		u.Enabled = truthy(row["enable"])
		u.Remark = firstNonEmpty(str(row["desc"]), str(row["remark"]))
		u.Group = str(row["group"])
		snap.Users = append(snap.Users, u)

		for _, iid := range decodeUintArray(row["inbounds"]) {
			if ref, ok := nodeByID[int64(iid)]; ok {
				snap.Bindings = append(snap.Bindings, Binding{UserRef: u.Ref, NodeRef: ref})
			}
		}
	}

	snap.Warnings = append(snap.Warnings, suiGlobalWarnings(snap)...)
	return snap, nil
}

func loadSUITLS(db *sql.DB, s schema) map[int64]map[string]interface{} {
	out := map[int64]map[string]interface{}{}
	if _, ok := s.Tables["tls"]; !ok {
		return out
	}
	rows, err := queryRows(db, `SELECT * FROM tls`)
	if err != nil {
		return out
	}
	for _, row := range rows {
		id := integer(row["id"])
		srv := decodeObject(row["server"])
		if srv == nil {
			continue
		}
		out[id] = srv
	}
	return out
}

// suiClient extracts credentials from the per-protocol config map. The map is
// shaped like {"vless":{"uuid":"...","flow":"..."},"trojan":{"password":"..."}}.
// We pick credentials for the protocol(s) the client is bound to; when only
// one protocol config exists, that one wins.
func suiClient(row map[string]interface{}, cfgMap map[string]interface{}, name string, cid int64) User {
	u := User{
		Ref:         fmt.Sprintf("client:%d:%s", cid, name),
		Username:    name,
		numericID:   cid,
		SourceSubID: name, // client name doubles as the subscription id
	}
	for key, raw := range cfgMap {
		entry, ok := raw.(map[string]interface{})
		if !ok {
			continue
		}
		proto, supported := protocolMap(key)
		if !supported {
			continue
		}
		if id := firstNonEmpty(str(entry["uuid"]), str(entry["id"])); id != "" {
			if u.UUID == "" || strings.EqualFold(proto, "vless") || strings.EqualFold(proto, "vmess") {
				u.UUID = id
			}
		}
		if pw := firstNonEmpty(str(entry["password"]), str(entry["auth_str"])); pw != "" && u.Password == "" {
			u.Password = pw
		}
		if f := str(entry["flow"]); f != "" && u.Flow == "" {
			u.Flow = f
		}
	}
	return u
}

func suiInboundConfig(proto, srcType string, opts, tls map[string]interface{}) (map[string]interface{}, []string) {
	cfg := map[string]interface{}{}
	var notes []string

	// sing-box transport object: {type:"ws", path:"..."} etc.
	transport := nestedObject(opts, "transport")
	switch nestedString(transport, "type") {
	case "ws":
		if p := nestedString(transport, "path"); p != "" && protoSupportsPath(proto) {
			cfg["ws-path"] = p
		} else if p != "" {
			notes = append(notes, "WebSocket path has no listener field for this protocol")
		}
	case "grpc":
		if n := nestedString(transport, "service_name"); n != "" && protoSupportsPath(proto) {
			cfg["grpc-service-name"] = n
		} else if n != "" {
			notes = append(notes, "gRPC service name has no listener field for this protocol")
		}
	case "httpupgrade":
		notes = append(notes, "HTTPUpgrade transport has no exact listener equivalent; review manually")
	}

	if tls != nil {
		reality := nestedObject(tls, "reality")
		if reality != nil && truthy(reality["enabled"]) && protoSupports(proto, "reality-config") {
			rc := map[string]interface{}{}
			if k := nestedString(reality, "private_key"); k != "" {
				rc["private-key"] = k
			}
			if sid := nestedString(reality, "short_id"); sid != "" {
				rc["short-id"] = []interface{}{sid}
			}
			hs := nestedObject(reality, "handshake")
			if server := nestedString(hs, "server"); server != "" {
				dest := server
				if p := integer(hs["port"]); p > 0 {
					dest = fmt.Sprintf("%s:%d", server, p)
				}
				rc["dest"] = dest
			}
			if sn := nestedString(tls, "server_name"); sn != "" {
				rc["server-names"] = []interface{}{sn}
			}
			if len(rc) > 0 {
				cfg["reality-config"] = rc
			}
		} else if truthy(tls["enabled"]) {
			notes = append(notes, "TLS certificate/key are stored as file paths in the source panel; configure TLS again before enabling")
		}
	}

	if proto == "hysteria2" {
		if up := integer(opts["up_mbps"]); up > 0 {
			cfg["up"] = fmt.Sprintf("%d Mbps", up)
		}
		if down := integer(opts["down_mbps"]); down > 0 {
			cfg["down"] = fmt.Sprintf("%d Mbps", down)
		}
		if obfs := nestedString(opts, "obfs"); obfs != "" {
			cfg["obfs"] = obfs
		}
		if pw := nestedString(opts, "obfs_password"); pw != "" {
			cfg["obfs-password"] = pw
		}
	}
	if proto == "shadowsocks" {
		if m := nestedString(opts, "method"); m != "" {
			cfg["cipher"] = m
		}
		if p := nestedString(opts, "password"); p != "" {
			cfg["password"] = p
		}
	}
	return cfg, notes
}

func protoSupportsPath(protocol string) bool {
	switch protocol {
	case "vless", "vmess", "trojan":
		return true
	default:
		return false
	}
}

func suiListen(v string) string {
	v = strings.TrimSpace(v)
	if v == "" || v == "0.0.0.0" || v == "::" || v == "::1" {
		return ""
	}
	return v
}

func suiStrings(in []interface{}) []string {
	var out []string
	for _, item := range in {
		if s := str(item); s != "" {
			out = append(out, s)
		}
	}
	return out
}

func suiGlobalWarnings(snap *Snapshot) []string {
	var w []string
	w = append(w,
		"Nodes are imported as disabled; review each listener's transport, TLS and access fields before enabling.",
		"Subscription URLs are regenerated by this panel; the source client name is not preserved as a subscription id.",
		"sing-box endpoints, outbounds, routing rules, DNS and custom TLS options have no Mihomo listener equivalent and were not imported.",
	)
	if len(snap.Users) > 0 {
		w = append(w, "Traffic counters are copied as historical totals; the panel's live counters start from these values.")
	}
	return w
}
