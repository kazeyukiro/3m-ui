package migration

import (
	"database/sql"
	"fmt"
	"sort"
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
	nodeProtoByID := make(map[int64]string)
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
		nodeProtoByID[id] = proto
	}

	clientRows, err := queryRows(db, `SELECT * FROM clients`)
	if err != nil {
		return nil, fmt.Errorf("read clients: %w", err)
	}
	for _, row := range clientRows {
		cid := integer(row["id"])
		name := firstNonEmpty(str(row["name"]), fmt.Sprintf("client-%d", cid))
		cfgMap := decodeObject(row["config"])

		// Restrict credential selection to the protocols of the inbounds this
		// client is actually bound to, so an unused config section cannot win.
		boundIDs := decodeUintArray(row["inbounds"])
		nodeProtocols := make([]string, 0, len(boundIDs))
		for _, iid := range boundIDs {
			if p, ok := nodeProtoByID[int64(iid)]; ok {
				nodeProtocols = append(nodeProtocols, p)
			}
		}
		u, conflict := suiClient(cfgMap, name, cid, nodeProtocols)
		if conflict != "" {
			snap.Warnings = append(snap.Warnings, conflict)
		}

		// Traffic and quota live on the client row itself.
		u.UploadBytes = integer(row["up"])
		u.DownloadBytes = integer(row["down"])
		u.TrafficLimit = integer(row["volume"])
		u.ExpireAt = timeFromSeconds(integer(row["expiry"]))
		u.Enabled = truthy(row["enable"])
		u.Remark = firstNonEmpty(str(row["desc"]), str(row["remark"]))
		u.Group = str(row["group"])
		snap.Users = append(snap.Users, u)

		for _, iid := range boundIDs {
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

// suiCredentialOrder is the fixed visit order for the per-protocol config map.
// Go map iteration order is randomised; iterating the map directly made the
// chosen UUID/Password depend on chance whenever a client carried different
// credentials in several protocol sections. The s-ui subscription generator
// reads one section per inbound ("vless" -> uuid, "trojan" -> password, ...),
// so sections that describe uuid-based protocols come first, then the
// password-based ones. "shadowsocks16" is the official config key for
// 2022-blake3 shadowsocks (util/shadowsocks.go).
var suiCredentialOrder = []string{
	"vless", "vmess", "tuic", "anytls",
	"trojan", "hysteria2", "hysteria", "shadowsocks16", "shadowsocks",
	"naive", "socks", "http", "mixed",
}

// suiCredential is one credential value found in a protocol config section.
// value is never rendered into warnings; only proto is.
type suiCredential struct {
	proto string
	value string
}

// suiClient extracts credentials from the per-protocol config map. The map is
// shaped like {"vless":{"uuid":"...","flow":"..."},"trojan":{"password":"..."}}
// and each section may carry its OWN credential (the source panel keeps them
// independent). The target user model stores a single UUID and a single
// Password for all of a user's bindings, so a client with different per-
// protocol credentials cannot be reproduced losslessly. We resolve that
// deterministically and return a human-readable conflict note (never containing
// any credential value) that the caller surfaces as a warning.
//
// nodeProtocols lists the Mihomo protocols of the inbounds this client is bound
// to; sections for protocols the client does not use are ignored, so a stray
// config section cannot override the credential that actually gets used.
func suiClient(cfgMap map[string]interface{}, name string, cid int64, nodeProtocols []string) (User, string) {
	u := User{
		Ref:         fmt.Sprintf("client:%d:%s", cid, name),
		Username:    name,
		numericID:   cid,
		SourceSubID: name, // client name doubles as the subscription id
	}

	used := make(map[string]bool, len(nodeProtocols))
	for _, p := range nodeProtocols {
		used[strings.ToLower(strings.TrimSpace(p))] = true
	}

	// Collect every candidate credential in a stable order, tagging the source
	// protocol so conflicts can be reported precisely.
	var uuids, passwords []suiCredential
	var flow string

	consider := func(key string, raw interface{}) {
		entry, ok := raw.(map[string]interface{})
		if !ok {
			return
		}
		proto, supported := suiConfigProtocol(key)
		if !supported {
			return
		}
		// Only keep sections whose protocol the client actually uses (when we
		// know the binding set). This keeps, e.g., an unused "vmess" section
		// from overriding the vless UUID on a vless-only client.
		if len(used) > 0 && !used[proto] {
			return
		}
		if id := firstNonEmpty(str(entry["uuid"]), str(entry["id"])); id != "" {
			uuids = append(uuids, suiCredential{key, id})
		}
		if pw := firstNonEmpty(str(entry["password"]), str(entry["auth_str"])); pw != "" {
			passwords = append(passwords, suiCredential{key, pw})
		}
		if flow == "" {
			flow = str(entry["flow"])
		}
	}

	seen := make(map[string]bool, len(cfgMap))
	for _, key := range suiCredentialOrder {
		if raw, ok := cfgMap[key]; ok {
			seen[strings.ToLower(key)] = true
			consider(key, raw)
		}
	}
	// Any protocol section not in the fixed order is still visited, but in a
	// deterministic (sorted) order so it cannot introduce randomness either.
	extra := make([]string, 0, len(cfgMap))
	for key := range cfgMap {
		if !seen[strings.ToLower(key)] {
			extra = append(extra, key)
		}
	}
	sort.Strings(extra)
	for _, key := range extra {
		consider(key, cfgMap[key])
	}

	if len(uuids) > 0 {
		u.UUID = uuids[0].value
	}
	if len(passwords) > 0 {
		u.Password = passwords[0].value
	}
	u.Flow = flow

	return u, suiCredentialConflict(name, uuids, passwords)
}

// suiConfigProtocol maps an s-ui client-config section key to its Mihomo
// listener protocol. The official "shadowsocks16" key (2022-blake3 shadowsocks,
// see util/shadowsocks.go) is not a real inbound type, so protocolMap does not
// know it; it shares the "shadowsocks" listener protocol.
func suiConfigProtocol(key string) (string, bool) {
	if strings.EqualFold(strings.TrimSpace(key), "shadowsocks16") {
		return "shadowsocks", true
	}
	return protocolMap(key)
}

// suiCredentialConflict builds a warning (never embedding credential values)
// when several protocol sections disagree on a UUID or a password. The target
// panel keeps one credential pair per user, so the operator must reconcile the
// affected protocols before enabling those listeners.
func suiCredentialConflict(name string, uuids, passwords []suiCredential) string {
	distinct := func(in []suiCredential) (map[string]bool, bool) {
		vals := make(map[string]bool, len(in))
		for _, c := range in {
			vals[c.value] = true
		}
		return vals, len(vals) > 1
	}
	uVals, uConflict := distinct(uuids)
	pVals, pConflict := distinct(passwords)
	if !uConflict && !pConflict {
		return ""
	}
	var parts []string
	if uConflict {
		protos := conflictProtocols(uuids, uVals)
		parts = append(parts, fmt.Sprintf("%d different UUIDs across %s", len(uVals), strings.Join(protos, "/")))
	}
	if pConflict {
		protos := conflictProtocols(passwords, pVals)
		parts = append(parts, fmt.Sprintf("%d different passwords across %s", len(pVals), strings.Join(protos, "/")))
	}
	return fmt.Sprintf("client %q has %s; this panel keeps one UUID and one Password per user, so only the first protocol's credential was kept — review and re-enter the credential on the other protocols' listeners before enabling them",
		name, strings.Join(parts, " and "))
}

func conflictProtocols(cs []suiCredential, distinct map[string]bool) []string {
	if len(distinct) < 2 {
		return nil
	}
	out := []string{}
	seen := map[string]bool{}
	for _, c := range cs {
		if !seen[c.proto] {
			seen[c.proto] = true
			out = append(out, c.proto)
		}
	}
	return out
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
			// sing-box stores short_id as a list; some panels wrote a bare
			// string. Normalise both into the list Mihomo expects, without
			// stringifying an array (which used to yield "[...]" literally).
			if ids := suiStringList(reality["short_id"]); len(ids) > 0 {
				rc["short-id"] = ids
			}
			hs := nestedObject(reality, "handshake")
			if server := nestedString(hs, "server"); server != "" {
				dest := server
				// sing-box uses server_port; older dumps used port.
				if p := firstPositive(integer(hs["server_port"]), integer(hs["port"])); p > 0 {
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
		// Official s-ui keeps obfs as an object {"type","password"}; older or
		// flattened dumps used a bare string plus a sibling obfs_password.
		if obfs := nestedObject(opts, "obfs"); obfs != nil {
			if t := nestedString(obfs, "type"); t != "" {
				cfg["obfs"] = t
			}
			if pw := nestedString(obfs, "password"); pw != "" {
				cfg["obfs-password"] = pw
			}
		} else {
			if t := nestedString(opts, "obfs"); t != "" {
				cfg["obfs"] = t
			}
			if pw := nestedString(opts, "obfs_password"); pw != "" {
				cfg["obfs-password"] = pw
			}
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

// suiStringList normalises a JSON value that is either a string, a list of
// strings, or nil into a []interface{} suitable for a Mihomo list field. It
// never stringifies a whole array (which would embed literal "[" and "]").
func suiStringList(v interface{}) []interface{} {
	switch x := v.(type) {
	case string:
		if s := strings.TrimSpace(x); s != "" {
			return []interface{}{s}
		}
	case []byte:
		if s := strings.TrimSpace(string(x)); s != "" {
			return []interface{}{s}
		}
	case []interface{}:
		out := make([]interface{}, 0, len(x))
		for _, item := range x {
			if s := str(item); strings.TrimSpace(s) != "" {
				out = append(out, s)
			}
		}
		return out
	case []string:
		out := make([]interface{}, 0, len(x))
		for _, s := range x {
			if strings.TrimSpace(s) != "" {
				out = append(out, s)
			}
		}
		return out
	}
	return nil
}

// firstPositive returns the first argument greater than zero.
func firstPositive(values ...int64) int64 {
	for _, v := range values {
		if v > 0 {
			return v
		}
	}
	return 0
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
