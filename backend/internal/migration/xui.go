package migration

import (
	"database/sql"
	"fmt"
	"strings"
)

// parseXUI reads an Xray-based panel database. Two client layouts are handled:
//
//   - classic: clients live in each inbound's settings JSON ("clients": [...]),
//     with per-client traffic in a separate client_traffics table;
//   - split: clients live in their own client_records table and are attached
//     to inbounds through client_inbounds.
//
// Both layouts are detected from the schema, so older and newer exports work.
func parseXUI(db *sql.DB, s schema) (*Snapshot, error) {
	snap := &Snapshot{Source: SourceXUI, Tables: s.tableNames()}

	rows, err := queryRows(db, `SELECT * FROM inbounds`)
	if err != nil {
		return nil, fmt.Errorf("read inbounds: %w", err)
	}

	traffic := loadXUITraffic(db, s)
	splitClients, splitLinks, splitOK := loadXUISplitClients(db, s)

	nodeByID := make(map[int64]string)
	for _, row := range rows {
		id := integer(row["id"])
		srcProto := str(row["protocol"])
		proto, supported := protocolMap(srcProto)
		remark := firstNonEmpty(str(row["remark"]), str(row["tag"]))
		name := firstNonEmpty(remark, fmt.Sprintf("inbound-%d-%s", id, srcProto))
		port := str(row["port"])
		if port == "" || port == "0" {
			port = ""
		}

		if !supported {
			snap.Skipped = append(snap.Skipped,
				fmt.Sprintf("inbound %q (%s): protocol %q has no Mihomo listener equivalent", name, port, srcProto))
			continue
		}
		if port == "" {
			snap.Skipped = append(snap.Skipped,
				fmt.Sprintf("inbound %q (%s): source has no usable port", name, srcProto))
			continue
		}

		settings := decodeObject(row["settings"])
		stream := decodeObject(row["stream_settings"])
		if stream == nil {
			stream = decodeObject(row["streamsettings"])
		}
		cfg, notes := xuiStreamConfig(proto, srcProto, stream, settings)

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
			Listen:         xuiListen(str(row["listen"])),
			Config:         encoded,
			SourceProtocol: srcProto,
			Notes:          notes,
		}
		snap.Nodes = append(snap.Nodes, node)
		nodeByID[id] = node.Ref

		// Classic layout: clients are embedded in the inbound's settings.
		if !splitOK {
			for _, c := range decodeArray(settings["clients"]) {
				u := xuiClient(c, srcProto, port, traffic)
				snap.Users = append(snap.Users, u)
				snap.Bindings = append(snap.Bindings, Binding{UserRef: u.Ref, NodeRef: node.Ref})
			}
		}
	}

	if splitOK {
		byID := make(map[int64]User, len(splitClients))
		for _, u := range splitClients {
			snap.Users = append(snap.Users, u)
			byID[u.numericID] = u
		}
		for _, link := range splitLinks {
			ref, ok := nodeByID[link.inboundID]
			if !ok {
				continue
			}
			if u, ok := byID[link.clientID]; ok {
				snap.Bindings = append(snap.Bindings, Binding{UserRef: u.Ref, NodeRef: ref})
			}
		}
	}

	snap.Warnings = append(snap.Warnings, xuiGlobalWarnings(snap)...)
	return snap, nil
}

// xuiTraffic holds per-client counters keyed by the client's email/identity.
type xuiTraffic struct {
	up, down int64
	total    int64
	expiry   int64
	enable   bool
}

func loadXUITraffic(db *sql.DB, s schema) map[string]xuiTraffic {
	out := map[string]xuiTraffic{}
	var table string
	for _, candidate := range []string{"client_traffics", "client_traffic", "node_client_traffics"} {
		if _, ok := s.Tables[candidate]; ok {
			table = candidate
			break
		}
	}
	if table == "" {
		return out
	}
	rows, err := queryRows(db, `SELECT * FROM "`+table+`"`)
	if err != nil {
		return out
	}
	for _, row := range rows {
		key := firstNonEmpty(str(row["email"]), str(row["client_email"]), str(row["sub_id"]))
		if key == "" {
			continue
		}
		t := xuiTraffic{
			up:     integer(row["up"]),
			down:   integer(row["down"]),
			total:  integer(row["total"]),
			expiry: integer(row["expiry_time"]),
			enable: truthy(row["enable"]),
		}
		// A later row may carry fresher counters; keep the larger totals.
		if prev, ok := out[key]; !ok || t.up+t.down > prev.up+prev.down {
			out[key] = t
		}
	}
	return out
}

// split client parsing (newer layout). numericID/link IDs are kept internal.
type xuiSplitLink struct {
	clientID  int64
	inboundID int64
}

func loadXUISplitClients(db *sql.DB, s schema) ([]User, []xuiSplitLink, bool) {
	var table string
	for _, candidate := range []string{"client_records", "clients"} {
		if _, ok := s.Tables[candidate]; ok {
			table = candidate
			break
		}
	}
	if table == "" {
		return nil, nil, false
	}
	rows, err := queryRows(db, `SELECT * FROM "`+table+`"`)
	if err != nil || len(rows) == 0 {
		return nil, nil, false
	}
	var users []User
	for _, row := range rows {
		u := xuiSplitUser(row)
		users = append(users, u)
	}

	var links []xuiSplitLink
	for _, candidate := range []string{"client_inbounds", "inbound_clients"} {
		if _, ok := s.Tables[candidate]; !ok {
			continue
		}
		lr, err := queryRows(db, `SELECT * FROM "`+candidate+`"`)
		if err != nil {
			continue
		}
		for _, row := range lr {
			cid := integer(row["client_id"])
			iid := integer(row["inbound_id"])
			if cid == 0 {
				cid = integer(row["client_record_id"])
			}
			if cid > 0 && iid > 0 {
				links = append(links, xuiSplitLink{clientID: cid, inboundID: iid})
			}
		}
		break
	}
	return users, links, true
}

func xuiSplitUser(row map[string]interface{}) User {
	email := firstNonEmpty(str(row["email"]), str(row["name"]))
	id := integer(row["id"])
	u := User{
		Ref:          fmt.Sprintf("client:%d:%s", id, email),
		Username:     email,
		UUID:         firstNonEmpty(str(row["uuid"]), str(row["id_value"])),
		Password:     str(row["password"]),
		Flow:         str(row["flow"]),
		TrafficLimit: gibToBytes(integer(row["total_gb"])),
		IPLimit:      int(integer(row["limit_ip"])),
		ExpireAt:     timeFromMillis(integer(row["expiry_time"])),
		Enabled:      truthy(row["enable"]),
		Remark:       firstNonEmpty(str(row["comment"]), str(row["remark"])),
		Group:        firstNonEmpty(str(row["group_name"]), str(row["group"])),
		TelegramID:   integer(row["tg_id"]),
		SourceSubID:  str(row["sub_id"]),
		numericID:    id,
	}
	if u.Username == "" {
		u.Username = fmt.Sprintf("client-%d", id)
	}
	return u
}

// xuiClient converts one embedded classic-layout client object.
func xuiClient(c map[string]interface{}, srcProto, port string, traffic map[string]xuiTraffic) User {
	email := firstNonEmpty(str(c["email"]), str(c["name"]))
	ref := "client:" + email
	credential := firstNonEmpty(str(c["id"]), str(c["password"]))
	u := User{
		Ref:          ref,
		Username:     email,
		Flow:         str(c["flow"]),
		TrafficLimit: gibToBytes(integer(c["totalGB"])),
		IPLimit:      int(integer(c["limitIp"])),
		ExpireAt:     timeFromMillis(integer(c["expiryTime"])),
		Enabled:      truthy(c["enable"]),
		Remark:       str(c["comment"]),
		TelegramID:   integer(c["tgId"]),
		SourceSubID:  str(c["subId"]),
	}
	if looksLikeUUID(credential) || strings.EqualFold(srcProto, "vless") || strings.EqualFold(srcProto, "vmess") {
		u.UUID = credential
	} else {
		u.Password = credential
	}
	if u.Username == "" {
		u.Username = fmt.Sprintf("client-%s-%s", srcProto, port)
		u.Ref = "client:" + u.Username
	}
	if t, ok := traffic[email]; ok {
		u.UploadBytes = t.up
		u.DownloadBytes = t.down
		if t.total > 0 {
			u.TrafficLimit = t.total
		}
		if t.expiry > 0 {
			u.ExpireAt = timeFromMillis(t.expiry)
		}
	}
	return u
}

func xuiListen(v string) string {
	v = strings.TrimSpace(v)
	if v == "" || v == "0.0.0.0" || v == "::" {
		return ""
	}
	return v
}

// xuiStreamConfig maps the subset of Xray stream settings that has an exact
// Mihomo listener counterpart. Anything else is reported as a review note.
func xuiStreamConfig(proto, srcProto string, stream, settings map[string]interface{}) (map[string]interface{}, []string) {
	cfg := map[string]interface{}{}
	var notes []string

	network := strings.ToLower(nestedString(stream, "network"))
	security := strings.ToLower(nestedString(stream, "security"))

	switch network {
	case "ws":
		ws := nestedObject(stream, "wsSettings")
		if p := nestedString(ws, "path"); p != "" {
			cfg["ws-path"] = p
		}
		if h := nestedString(ws, "host"); h == "" {
			if headers := nestedObject(ws, "headers"); headers != nil {
				_ = headers
			}
		}
	case "grpc", "gun":
		grpc := nestedObject(stream, "grpcSettings")
		if n := nestedString(grpc, "serviceName"); n != "" {
			cfg["grpc-service-name"] = n
		}
	case "xhttp", "splithttp":
		xh := nestedObject(stream, "xhttpSettings")
		if xh != nil {
			xcfg := map[string]interface{}{}
			if p := nestedString(xh, "path"); p != "" {
				xcfg["path"] = p
			}
			if h := nestedString(xh, "host"); h != "" {
				xcfg["host"] = h
			}
			if m := nestedString(xh, "mode"); m != "" {
				xcfg["mode"] = m
			}
			if len(xcfg) > 0 && protoSupports(proto, "xhttp-config") {
				cfg["xhttp-config"] = xcfg
			} else if len(xcfg) > 0 {
				notes = append(notes, "XHTTP transport has no exact listener equivalent; review manually")
			}
		}
	case "kcp", "mkcp":
		notes = append(notes, "mKCP transport is not supported by Mihomo listeners")
	case "quic":
		notes = append(notes, "QUIC transport is not supported by Mihomo listeners")
	case "httpupgrade":
		notes = append(notes, "HTTPUpgrade transport has no exact listener equivalent; review manually")
	}

	switch security {
	case "reality":
		r := nestedObject(stream, "realitySettings")
		if r != nil {
			rc := map[string]interface{}{}
			if d := nestedString(r, "dest"); d != "" {
				rc["dest"] = d
			}
			if k := nestedString(r, "privateKey"); k != "" {
				rc["private-key"] = k
			}
			if ids, ok := r["shortIds"].([]interface{}); ok && len(ids) > 0 {
				rc["short-id"] = ids
			} else if sid := nestedString(r, "shortId"); sid != "" {
				rc["short-id"] = []interface{}{sid}
			}
			if names, ok := r["serverNames"].([]interface{}); ok && len(names) > 0 {
				rc["server-names"] = names
			}
			if len(rc) > 0 && protoSupports(proto, "reality-config") {
				cfg["reality-config"] = rc
			} else if len(rc) > 0 {
				notes = append(notes, "REALITY settings were not imported: protocol has no reality-config field")
			}
		}
	case "tls":
		notes = append(notes, "TLS certificate/key are stored as file paths in the source panel; configure TLS again before enabling")
	}

	// VLESS flow is a listener-level field here; keep the first client hint.
	if proto == "vless" {
		if f := xuiFirstFlow(settings); f != "" {
			cfg["flow"] = f
		}
	}
	// Shadowsocks carries its cipher/method on the inbound, not per client.
	if proto == "shadowsocks" && settings != nil {
		if m := nestedString(settings, "method"); m != "" {
			cfg["cipher"] = m
		}
		if p := nestedString(settings, "password"); p != "" {
			cfg["password"] = p
		}
	}
	// Hysteria2 / TUIC bandwidth hints.
	if proto == "hysteria2" && settings != nil {
		if up := integer(settings["up_mbps"]); up > 0 {
			cfg["up"] = fmt.Sprintf("%d Mbps", up)
		}
		if down := integer(settings["down_mbps"]); down > 0 {
			cfg["down"] = fmt.Sprintf("%d Mbps", down)
		}
	}
	return cfg, notes
}

func xuiFirstFlow(settings map[string]interface{}) string {
	if settings == nil {
		return ""
	}
	for _, c := range decodeArray(settings["clients"]) {
		if f := str(c["flow"]); f != "" {
			return f
		}
	}
	return ""
}

func protoSupports(protocol, field string) bool {
	// The schema validator is the source of truth; this is a cheap pre-check
	// so unsupported fields can be reported instead of silently dropped.
	switch field {
	case "reality-config":
		return protocol == "vless" || protocol == "vmess" || protocol == "trojan"
	case "xhttp-config":
		return protocol == "vless"
	default:
		return false
	}
}

func xuiGlobalWarnings(snap *Snapshot) []string {
	var w []string
	w = append(w,
		"Nodes are imported as disabled; review each listener's transport, TLS and access fields before enabling.",
		"Subscription URLs are regenerated by this panel; clients must re-import their subscription after migration.",
		"Xray stream settings (sniffing, fallbacks, TCP header, external proxy and routing) have no Mihomo listener equivalent and were not imported.",
	)
	if len(snap.Users) > 0 {
		w = append(w, "Traffic counters are copied as historical totals; the panel's live counters start from these values.")
	}
	return w
}
