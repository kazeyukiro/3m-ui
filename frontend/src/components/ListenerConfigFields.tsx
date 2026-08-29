import React from 'react';
import {
  Form, Input, InputNumber, Select, Switch, Divider, Alert, Space, Typography, Button, Card, Radio,
} from 'antd';
import { generateMaterial } from '../api/listeners';
import { MinusCircleOutlined, PlusOutlined } from '@ant-design/icons';
import { useI18n } from '../i18n';

const { Text } = Typography;

/** Official SS ciphers from https://wiki.metacubex.one/config/inbound/listeners/ss/ */
export const SS_CIPHERS = [
  '2022-blake3-aes-128-gcm',
  '2022-blake3-aes-256-gcm',
  '2022-blake3-chacha20-poly1305',
  'aes-128-gcm',
  'aes-192-gcm',
  'aes-256-gcm',
  'chacha20-ietf-poly1305',
  'xchacha20-ietf-poly1305',
  'none',
];

const TLS_PROTOCOLS = new Set([
  'vmess', 'vless', 'trojan', 'hysteria2', 'tuic', 'anytls', 'trusttunnel',
]);
const REALITY_PROTOCOLS = new Set(['vmess', 'vless', 'trojan']);
const TRANSPORT_PROTOCOLS = new Set(['vmess', 'vless', 'trojan']);
const UDP_PROTOCOLS = new Set(['shadowsocks', 'snell', 'vmess', 'vless', 'trojan']);
/** Protocols that support shadow-tls / res-tls / jls-config wrappers */
const WRAPPER_TLS_PROTOCOLS = new Set([
  'shadowsocks', 'snell', 'vmess', 'vless', 'trojan', 'anytls',
]);
const MUX_PROTOCOLS = new Set(['shadowsocks', 'vmess', 'vless', 'trojan']);
const SIMPLE_OBFS_PROTOCOLS = new Set(['shadowsocks']);
const KCP_TUN_PROTOCOLS = new Set(['shadowsocks']);
const XHTTP_PROTOCOLS = new Set(['vless']);
const MKCP_PROTOCOLS = new Set(['vmess']);
const MEKYA_PROTOCOLS = new Set(['vmess']);
/** Protocols that support allow-insecure (plain TLS offload behind nginx/caddy). */
const ALLOW_INSECURE_PROTOCOLS = new Set(['vmess', 'vless', 'trojan', 'anytls']);

export function protocolSupportsUDP(protocol: string): boolean {
  return UDP_PROTOCOLS.has(protocol);
}

export function protocolSupportsTLS(protocol: string): boolean {
  return TLS_PROTOCOLS.has(protocol);
}

function asArray(v: any): any[] {
  if (!v) return [];
  return Array.isArray(v) ? v : [v];
}

function asStringList(v: any): string[] {
  if (!v) return [];
  if (Array.isArray(v)) return v.map(String);
  return [String(v)];
}

/** Parse stored config JSON into form field values. */
export function configToFormValues(raw: string | undefined | null): Record<string, any> {
  let cfg: Record<string, any> = {};
  if (raw) {
    try {
      cfg = typeof raw === 'string' ? JSON.parse(raw) : raw;
    } catch {
      cfg = {};
    }
  }
  // Only copy scalar / array top-level fields the form understands.
  // Nested objects are expanded explicitly below to avoid leaking raw objects into Form state.
  const values: Record<string, any> = {};
  for (const [k, v] of Object.entries(cfg)) {
    if (v !== null && typeof v === 'object' && !Array.isArray(v)) continue;
    values[k] = v;
  }

  // Reality
  if (cfg['reality-config'] && typeof cfg['reality-config'] === 'object') {
    const r = cfg['reality-config'];
    values.reality_enabled = true;
    values.reality_dest = r.dest;
    values.reality_private_key = r['private-key'];
    values.reality_short_id = asStringList(r['short-id']);
    values.reality_server_names = asStringList(r['server-names']);
  } else {
    values.reality_enabled = false;
  }

  // Trojan ss-option
  if (cfg['ss-option'] && typeof cfg['ss-option'] === 'object') {
    values.ss_option_enabled = !!cfg['ss-option'].enabled;
    values.ss_option_method = cfg['ss-option'].method;
    values.ss_option_password = cfg['ss-option'].password;
  }

  // simple-obfs
  if (cfg['simple-obfs'] && typeof cfg['simple-obfs'] === 'object') {
    values.simple_obfs_enabled = !!cfg['simple-obfs'].enable;
    values.simple_obfs_mode = cfg['simple-obfs'].mode;
  }

  // shadow-tls
  if (cfg['shadow-tls'] && typeof cfg['shadow-tls'] === 'object') {
    const s = cfg['shadow-tls'];
    values.shadow_tls_enabled = !!s.enable;
    values.shadow_tls_version = s.version;
    values.shadow_tls_password = s.password;
    values.shadow_tls_handshake_dest = s.handshake?.dest;
    values.shadow_tls_handshake_proxy = s.handshake?.proxy;
    values.shadow_tls_users = asArray(s.users).map((u: any) => ({
      name: u?.name ?? u?.username ?? '',
      password: u?.password ?? '',
    }));
  }

  // res-tls
  if (cfg['res-tls'] && typeof cfg['res-tls'] === 'object') {
    const r = cfg['res-tls'];
    values.res_tls_enabled = !!r.enable;
    values.res_tls_dest = r.dest;
    values.res_tls_password = r.password;
    values.res_tls_restls_script = r['restls-script'];
    values.res_tls_min_record_len = r['min-record-len'];
    values.res_tls_proxy = r.proxy;
    values.res_tls_rate_limit = r['rate-limit'];
  }

  // jls-config
  if (cfg['jls-config'] && typeof cfg['jls-config'] === 'object') {
    const j = cfg['jls-config'];
    values.jls_enabled = !!j.enable;
    values.jls_dest = j.dest;
    values.jls_sni = j.sni;
    values.jls_alpn = asStringList(j.alpn);
    values.jls_proxy = j.proxy;
    values.jls_rate_limit = j['rate-limit'];
    values.jls_users = asArray(j.users).map((u: any) => ({
      username: u?.username ?? u?.name ?? '',
      password: u?.password ?? '',
    }));
  }

  // mux-option
  if (cfg['mux-option'] && typeof cfg['mux-option'] === 'object') {
    const m = cfg['mux-option'];
    values.mux_enabled = m.enable === true || m.padding === true || m.brutal != null || m['brutal-opts'] != null;
    values.mux_padding = !!m.padding;
    values.mux_protocol = m.protocol;
    values.mux_max_connections = m['max-connections'];
    values.mux_min_streams = m['min-streams'];
    values.mux_max_streams = m['max-streams'];
    values.mux_statistic = !!m.statistic;
    values.mux_only_tcp = !!m['only-tcp'];
    const brutal = m.brutal || m['brutal-opts'] || {};
    values.mux_brutal_enabled = !!brutal.enabled || !!brutal.enable;
    values.mux_brutal_up = brutal.up;
    values.mux_brutal_down = brutal.down;
  }

  // kcp-tun (SS)
  if (cfg['kcp-tun'] && typeof cfg['kcp-tun'] === 'object') {
    const k = cfg['kcp-tun'];
    values.kcp_tun_enabled = !!k.enable;
    values.kcp_tun_key = k.key;
    values.kcp_tun_crypt = k.crypt;
    values.kcp_tun_mode = k.mode;
    values.kcp_tun_conn = k.conn;
    values.kcp_tun_mtu = k.mtu;
    values.kcp_tun_sndwnd = k.sndwnd;
    values.kcp_tun_rcvwnd = k.rcvwnd;
    values.kcp_tun_nocomp = !!k.nocomp;
  }

  // xhttp-config (VLESS)
  if (cfg['xhttp-config'] && typeof cfg['xhttp-config'] === 'object') {
    const x = cfg['xhttp-config'];
    values.xhttp_enabled = true;
    values.xhttp_path = x.path;
    values.xhttp_host = x.host;
    values.xhttp_mode = x.mode;
  }

  // mkcp-config (VMess)
  if (cfg['mkcp-config'] && typeof cfg['mkcp-config'] === 'object') {
    const k = cfg['mkcp-config'];
    values.mkcp_enabled = k.enable !== false;
    values.mkcp_mtu = k.mtu;
    values.mkcp_tti = k.tti;
    values.mkcp_uplink = k['uplink-capacity'];
    values.mkcp_downlink = k['downlink-capacity'];
    values.mkcp_congestion = !!k.congestion;
    values.mkcp_write_buffer = k['write-buffer'];
    values.mkcp_read_buffer = k['read-buffer'];
    values.mkcp_seed = k.seed;
    values.mkcp_header = k.header;
  }

  // mekya-config (VMess) — incompatible with mkcp/ws/grpc per official docs
  if (cfg['mekya-config'] && typeof cfg['mekya-config'] === 'object') {
    const m = cfg['mekya-config'];
    values.mekya_enabled = m.enable !== false;
    values.mekya_max_write_size = m['max-write-size'];
    values.mekya_max_write_duration_ms = m['max-write-duration-ms'];
    values.mekya_max_simultaneous_write_connection = m['max-simultaneous-write-connection'];
    values.mekya_packet_writing_buffer = m['packet-writing-buffer'];
    const kcp = m.kcp && typeof m.kcp === 'object' ? m.kcp : {};
    values.mekya_kcp_mtu = kcp.mtu;
    values.mekya_kcp_tti = kcp.tti;
    values.mekya_kcp_uplink = kcp['uplink-capacity'];
    values.mekya_kcp_downlink = kcp['downlink-capacity'];
    values.mekya_kcp_congestion = !!kcp.congestion;
    values.mekya_kcp_write_buffer = kcp['write-buffer'];
    values.mekya_kcp_read_buffer = kcp['read-buffer'];
    values.mekya_kcp_seed = kcp.seed;
    values.mekya_kcp_header = kcp.header;
  }

  // snell obfs-opts
  if (cfg['obfs-opts'] && typeof cfg['obfs-opts'] === 'object') {
    values.obfs_opts_mode = cfg['obfs-opts'].mode;
    values.obfs_opts_host = cfg['obfs-opts'].host;
  }

  // shadowquic jls-upstream
  if (cfg['jls-upstream'] && typeof cfg['jls-upstream'] === 'object') {
    const j = cfg['jls-upstream'];
    values.jls_upstream_enabled = true;
    values.jls_upstream_addr = j.addr;
    values.jls_upstream_sni = j.sni;
    values.jls_upstream_proxy = j.proxy;
    values.jls_upstream_rate_limit = j['rate-limit'];
  }

  // hysteria2 realm-opts
  if (cfg['realm-opts'] && typeof cfg['realm-opts'] === 'object') {
    const r = cfg['realm-opts'];
    values.realm_enabled = !!r.enable;
    values.realm_server_url = r['server-url'];
    values.realm_token = r.token;
    values.realm_id = r['realm-id'];
    values.realm_stun = asStringList(r['stun-servers']);
    values.realm_proxy = r.proxy;
    values.realm_skip_cert = !!r['skip-cert-verify'];
  }

  // sudoku extras
  if (cfg['padding-min'] != null) values['padding-min'] = cfg['padding-min'];
  if (cfg['padding-max'] != null) values['padding-max'] = cfg['padding-max'];
  if (cfg['table-type']) values['table-type'] = cfg['table-type'];
  if (cfg['handshake-timeout'] != null) values['handshake-timeout'] = cfg['handshake-timeout'];
  if (cfg['enable-pure-downlink'] != null) values['enable-pure-downlink'] = cfg['enable-pure-downlink'];

  // trusttunnel
  if (cfg.network) values.network = cfg.network;
  if (cfg['bbr-profile']) values['bbr-profile'] = cfg['bbr-profile'];
  if (cfg['quic-versions']) values['quic-versions'] = asStringList(cfg['quic-versions']);
  if (cfg.cwnd != null) values.cwnd = cfg.cwnd;

  // ALPN
  if (cfg.alpn && !Array.isArray(cfg.alpn)) {
    values.alpn = [cfg.alpn];
  }

  // token
  if (Array.isArray(cfg.token)) {
    values.token = cfg.token.join(',');
  }

  // users managed elsewhere
  delete values.users;
  delete values['shadow-tls'];
  delete values['res-tls'];
  delete values['jls-config'];
  delete values['simple-obfs'];
  delete values['mux-option'];
  delete values['kcp-tun'];
  delete values['xhttp-config'];
  delete values['mkcp-config'];
  delete values['mekya-config'];
  delete values['obfs-opts'];
  delete values['jls-upstream'];
  delete values['realm-opts'];
  delete values['reality-config'];
  delete values['ss-option'];

  if (cfg['ws-path']) values.transport_layer = 'ws';
  else if (cfg['grpc-service-name']) values.transport_layer = 'grpc';
  else if (cfg['xhttp-config']) values.transport_layer = 'xhttp';
  else values.transport_layer = 'raw';
  if (cfg['reality-config']) values.security_layer = 'reality';
  else if (cfg.certificate || cfg['private-key']) values.security_layer = 'tls';
  else values.security_layer = 'none';

  return values;
}

/** Keys the visual form fully owns. */
const FORM_OWNED_KEYS = new Set([
  'cipher', 'password', 'psk', 'version', 'alterId', 'flow', 'decryption', 'encryption',
  'ws-path', 'grpc-service-name', 'ss-option',
  'up', 'down', 'ignore-client-bandwidth', 'obfs', 'obfs-password', 'obfs-min-packet-size', 'obfs-max-packet-size',
  'masquerade', 'alpn', 'max-idle-time', 'handshake-timeout', 'token', 'congestion-controller',
  'authentication-timeout', 'max-udp-relay-packet-size', 'zero-rtt', 'padding-scheme', 'transport',
  'key', 'aead-method', 'padding-min', 'padding-max', 'table-type', 'enable-pure-downlink',
  'certificate', 'private-key', 'client-auth-type', 'client-auth-cert', 'ech-key', 'allow-insecure',
  'reality-config', 'users', 'simple-obfs', 'shadow-tls', 'res-tls', 'jls-config', 'mux-option',
  'kcp-tun', 'xhttp-config', 'mkcp-config', 'mekya-config', 'obfs-opts', 'jls-upstream', 'realm-opts',
  'network', 'bbr-profile', 'quic-versions', 'cwnd', 'traffic-pattern', 'user-hint-is-mandatory',
]);

function cleanObj(obj: Record<string, any>): Record<string, any> | undefined {
  const out: Record<string, any> = {};
  for (const [k, v] of Object.entries(obj)) {
    if (v === undefined || v === null || v === '') continue;
    if (typeof v === 'number' && Number.isNaN(v)) continue;
    if (Array.isArray(v) && v.length === 0) continue;
    out[k] = v;
  }
  return Object.keys(out).length ? out : undefined;
}

/** Coerce form value to a finite number, or undefined if empty/invalid. */
function toNum(v: any): number | undefined {
  if (v === undefined || v === null || v === '') return undefined;
  const n = typeof v === 'number' ? v : Number(v);
  return Number.isFinite(n) ? n : undefined;
}

/** Coerce to int when finite. */
function toInt(v: any): number | undefined {
  const n = toNum(v);
  return n === undefined ? undefined : Math.trunc(n);
}

/** Build official Mihomo listener config from form values. */
export function formValuesToConfig(
  protocol: string,
  values: Record<string, any>,
  previousConfig?: Record<string, any> | null,
): Record<string, any> {
  const cfg: Record<string, any> = {};

  if (previousConfig && typeof previousConfig === 'object') {
    for (const [k, v] of Object.entries(previousConfig)) {
      if (!FORM_OWNED_KEYS.has(k) && k !== 'name' && k !== 'type' && k !== 'port' && k !== 'listen') {
        cfg[k] = v;
      }
    }
  }

  const set = (key: string, v: any) => {
    if (v === undefined || v === null || v === '') return;
    if (typeof v === 'number' && Number.isNaN(v)) return;
    if (Array.isArray(v) && v.length === 0) return;
    cfg[key] = v;
  };

  switch (protocol) {
    case 'shadowsocks':
      set('cipher', values.cipher);
      set('password', values.password);
      break;
    case 'snell':
      set('psk', values.psk);
      {
        const ver = toInt(values.version);
        if (ver !== undefined) set('version', ver);
      }
      if (values.obfs_opts_mode || values.obfs_opts_host) {
        const o = cleanObj({ mode: values.obfs_opts_mode, host: values.obfs_opts_host });
        if (o) cfg['obfs-opts'] = o;
      }
      break;
    case 'vmess':
      {
        const aid = toInt(values.alterId);
        if (aid !== undefined) set('alterId', aid);
      }
      set('ws-path', values['ws-path']);
      set('grpc-service-name', values['grpc-service-name']);
      break;
    case 'vless':
      set('flow', values.flow);
      set('ws-path', values['ws-path']);
      set('grpc-service-name', values['grpc-service-name']);
      set('decryption', values.decryption);
      set('encryption', values.encryption);
      break;
    case 'trojan':
      set('ws-path', values['ws-path']);
      set('grpc-service-name', values['grpc-service-name']);
      if (values.ss_option_enabled) {
        cfg['ss-option'] = cleanObj({
          enabled: true,
          method: values.ss_option_method,
          password: values.ss_option_password,
        });
      }
      break;
    case 'hysteria2':
      set('up', values.up);
      set('down', values.down);
      if (values['ignore-client-bandwidth'] === true) cfg['ignore-client-bandwidth'] = true;
      set('obfs', values.obfs);
      set('obfs-password', values['obfs-password']);
      set('obfs-min-packet-size', values['obfs-min-packet-size']);
      set('obfs-max-packet-size', values['obfs-max-packet-size']);
      set('masquerade', values.masquerade);
      set('alpn', values.alpn);
      set('max-idle-time', values['max-idle-time']);
      set('handshake-timeout', values['handshake-timeout']);
      set('bbr-profile', values['bbr-profile']);
      break;
    case 'tuic':
      if (values.token) {
        const tokens = String(values.token).split(',').map((s: string) => s.trim()).filter(Boolean);
        set('token', tokens.length === 1 ? tokens[0] : tokens);
      }
      set('congestion-controller', values['congestion-controller']);
      set('alpn', values.alpn);
      set('max-idle-time', values['max-idle-time']);
      set('authentication-timeout', values['authentication-timeout']);
      set('max-udp-relay-packet-size', values['max-udp-relay-packet-size']);
      set('bbr-profile', values['bbr-profile']);
      break;
    case 'shadowquic':
      set('alpn', values.alpn);
      set('congestion-controller', values['congestion-controller']);
      if (values['zero-rtt'] === true) cfg['zero-rtt'] = true;
      set('up', values.up);
      set('down', values.down);
      if (values['ignore-client-bandwidth'] === true) cfg['ignore-client-bandwidth'] = true;
      set('max-idle-time', values['max-idle-time']);
      set('cwnd', values.cwnd);
      set('bbr-profile', values['bbr-profile']);
      set('quic-versions', values['quic-versions']);
      break;
    case 'anytls':
      set('padding-scheme', values['padding-scheme']);
      break;
    case 'mieru':
      set('transport', values.transport);
      set('traffic-pattern', values['traffic-pattern']);
      if (values['user-hint-is-mandatory'] === true) cfg['user-hint-is-mandatory'] = true;
      break;
    case 'sudoku':
      set('key', values.key);
      set('aead-method', values['aead-method']);
      set('padding-min', values['padding-min']);
      set('padding-max', values['padding-max']);
      set('table-type', values['table-type']);
      set('handshake-timeout', values['handshake-timeout']);
      if (values['enable-pure-downlink'] === true) cfg['enable-pure-downlink'] = true;
      break;
    case 'trusttunnel':
      set('network', values.network);
      set('congestion-controller', values['congestion-controller']);
      set('bbr-profile', values['bbr-profile']);
      break;
    default:
      break;
  }

  // Reality vs certificate/private-key are mutually exclusive per official docs.
  if (values.security_layer === 'reality') {
    values.reality_enabled = true;
  } else if (values.security_layer === 'none' || values.security_layer === 'tls') {
    values.reality_enabled = false;
  }
  if (values.transport_layer === 'ws') {
    values['grpc-service-name'] = undefined;
    values.xhttp_enabled = false;
  } else if (values.transport_layer === 'grpc') {
    values['ws-path'] = undefined;
    values.xhttp_enabled = false;
  } else if (values.transport_layer === 'xhttp') {
    values['ws-path'] = undefined;
    values['grpc-service-name'] = undefined;
    values.xhttp_enabled = true;
  } else if (values.transport_layer === 'raw') {
    values['ws-path'] = undefined;
    values['grpc-service-name'] = undefined;
    values.xhttp_enabled = false;
  }
  const realityOn = REALITY_PROTOCOLS.has(protocol) && !!values.reality_enabled;
  if (realityOn) {
    const reality = cleanObj({
      dest: values.reality_dest,
      'private-key': values.reality_private_key,
      'short-id': values.reality_short_id,
      'server-names': values.reality_server_names,
    });
    if (reality) cfg['reality-config'] = reality;
  } else if (TLS_PROTOCOLS.has(protocol)) {
    set('certificate', values.certificate);
    set('private-key', values['private-key']);
    set('client-auth-type', values['client-auth-type']);
    set('client-auth-cert', values['client-auth-cert']);
    set('ech-key', values['ech-key']);
    if (ALLOW_INSECURE_PROTOCOLS.has(protocol) && values['allow-insecure'] === true) cfg['allow-insecure'] = true;
  }

  // simple-obfs
  if (SIMPLE_OBFS_PROTOCOLS.has(protocol) && values.simple_obfs_enabled) {
    cfg['simple-obfs'] = cleanObj({
      enable: true,
      mode: values.simple_obfs_mode,
    }) || { enable: true };
  }

  // shadow-tls
  if (WRAPPER_TLS_PROTOCOLS.has(protocol) && values.shadow_tls_enabled) {
    const users = asArray(values.shadow_tls_users)
      .filter((u: any) => u?.name || u?.password)
      .map((u: any) => cleanObj({ name: u.name, password: u.password }))
      .filter(Boolean);
    const handshake = cleanObj({
      dest: values.shadow_tls_handshake_dest,
      proxy: values.shadow_tls_handshake_proxy,
    });
    cfg['shadow-tls'] = cleanObj({
      enable: true,
      version: toInt(values.shadow_tls_version),
      password: values.shadow_tls_password,
      users: users.length ? users : undefined,
      handshake,
    }) || { enable: true };
  }

  // res-tls
  if (WRAPPER_TLS_PROTOCOLS.has(protocol) && values.res_tls_enabled) {
    cfg['res-tls'] = cleanObj({
      enable: true,
      dest: values.res_tls_dest,
      password: values.res_tls_password,
      'restls-script': values.res_tls_restls_script,
      'min-record-len': values.res_tls_min_record_len,
      proxy: values.res_tls_proxy,
      'rate-limit': values.res_tls_rate_limit,
    }) || { enable: true };
  }

  // jls-config
  if (WRAPPER_TLS_PROTOCOLS.has(protocol) && values.jls_enabled) {
    const users = asArray(values.jls_users)
      .filter((u: any) => u?.username || u?.password)
      .map((u: any) => cleanObj({ username: u.username, password: u.password }))
      .filter(Boolean);
    cfg['jls-config'] = cleanObj({
      enable: true,
      dest: values.jls_dest,
      sni: values.jls_sni,
      alpn: values.jls_alpn,
      proxy: values.jls_proxy,
      'rate-limit': values.jls_rate_limit,
      users: users.length ? users : undefined,
    }) || { enable: true };
  }

  // mux-option
  if (MUX_PROTOCOLS.has(protocol) && values.mux_enabled) {
    const brutal = values.mux_brutal_enabled
      ? cleanObj({ enabled: true, up: values.mux_brutal_up, down: values.mux_brutal_down })
      : undefined;
    cfg['mux-option'] = cleanObj({
      enable: true,
      protocol: values.mux_protocol,
      'max-connections': values.mux_max_connections,
      'min-streams': values.mux_min_streams,
      'max-streams': values.mux_max_streams,
      padding: values.mux_padding === true ? true : undefined,
      statistic: values.mux_statistic === true ? true : undefined,
      'only-tcp': values.mux_only_tcp === true ? true : undefined,
      brutal,
    }) || { enable: true };
  }

  // kcp-tun
  if (KCP_TUN_PROTOCOLS.has(protocol) && values.kcp_tun_enabled) {
    cfg['kcp-tun'] = cleanObj({
      enable: true,
      key: values.kcp_tun_key,
      crypt: values.kcp_tun_crypt,
      mode: values.kcp_tun_mode,
      conn: values.kcp_tun_conn,
      mtu: values.kcp_tun_mtu,
      sndwnd: values.kcp_tun_sndwnd,
      rcvwnd: values.kcp_tun_rcvwnd,
      nocomp: values.kcp_tun_nocomp === true ? true : undefined,
    }) || { enable: true };
  }

  // xhttp-config
  if (XHTTP_PROTOCOLS.has(protocol) && values.xhttp_enabled) {
    const xhttp = cleanObj({
      path: values.xhttp_path,
      host: values.xhttp_host,
      mode: values.xhttp_mode,
    });
    // Official schema: non-empty xhttp-config enables the transport; keep at least path when toggled on.
    cfg['xhttp-config'] = xhttp || { path: '/' };
  }

  // mkcp-config
  if (MKCP_PROTOCOLS.has(protocol) && values.mkcp_enabled) {
    cfg['mkcp-config'] = cleanObj({
      enable: true,
      mtu: values.mkcp_mtu,
      tti: values.mkcp_tti,
      'uplink-capacity': values.mkcp_uplink,
      'downlink-capacity': values.mkcp_downlink,
      congestion: values.mkcp_congestion === true ? true : undefined,
      'write-buffer': values.mkcp_write_buffer,
      'read-buffer': values.mkcp_read_buffer,
      seed: values.mkcp_seed,
      header: values.mkcp_header,
    }) || { enable: true };
  }

  // mekya-config (VMess)
  if (MEKYA_PROTOCOLS.has(protocol) && values.mekya_enabled) {
    const kcp = cleanObj({
      mtu: values.mekya_kcp_mtu,
      tti: values.mekya_kcp_tti,
      'uplink-capacity': values.mekya_kcp_uplink,
      'downlink-capacity': values.mekya_kcp_downlink,
      congestion: values.mekya_kcp_congestion === true ? true : undefined,
      'write-buffer': values.mekya_kcp_write_buffer,
      'read-buffer': values.mekya_kcp_read_buffer,
      seed: values.mekya_kcp_seed,
      header: values.mekya_kcp_header,
    });
    cfg['mekya-config'] = cleanObj({
      enable: true,
      'max-write-size': values.mekya_max_write_size,
      'max-write-duration-ms': values.mekya_max_write_duration_ms,
      'max-simultaneous-write-connection': values.mekya_max_simultaneous_write_connection,
      'packet-writing-buffer': values.mekya_packet_writing_buffer,
      kcp,
    }) || { enable: true };
  }

  // jls-upstream (shadowquic)
  if (protocol === 'shadowquic' && values.jls_upstream_enabled) {
    const upstream = cleanObj({
      addr: values.jls_upstream_addr,
      sni: values.jls_upstream_sni,
      proxy: values.jls_upstream_proxy,
      'rate-limit': values.jls_upstream_rate_limit,
    });
    if (upstream) cfg['jls-upstream'] = upstream;
  }

  // realm-opts (hysteria2)
  if (protocol === 'hysteria2' && values.realm_enabled) {
    cfg['realm-opts'] = cleanObj({
      enable: true,
      'server-url': values.realm_server_url,
      token: values.realm_token,
      'realm-id': values.realm_id,
      'stun-servers': values.realm_stun,
      proxy: values.realm_proxy,
      'skip-cert-verify': values.realm_skip_cert === true ? true : undefined,
    }) || { enable: true };
  }

  return cfg;
}

/** Collapsible-style section with enable switch driving nested fields. */
const EnableSection: React.FC<{
  name: string;
  label: string;
  hint?: string;
  children: React.ReactNode;
}> = ({ name, label, hint, children }) => (
  <>
    <Divider titlePlacement="start" plain>{label}</Divider>
    {hint && (
      <Text type="secondary" style={{ display: 'block', marginBottom: 8 }}>{hint}</Text>
    )}
    <Form.Item name={name} label={label} valuePropName="checked">
      <Switch />
    </Form.Item>
    <Form.Item noStyle shouldUpdate={(prev, cur) => prev[name] !== cur[name]}>
      {({ getFieldValue }) => (getFieldValue(name) ? <Card size="small" style={{ marginBottom: 16 }}>{children}</Card> : null)}
    </Form.Item>
  </>
);

type Props = { protocol?: string };

const ListenerConfigFields: React.FC<Props> = ({ protocol }) => {
  const { t } = useI18n();
  const form = Form.useFormInstance();
  const gen = async (kind: string, cipher?: string) => {
    try {
      const data = await generateMaterial({ kind, cipher });
      if (kind === 'reality') {
        form.setFieldsValue({
          reality_private_key: data.private_key,
          reality_short_id: data.short_id ? [data.short_id] : undefined,
        });
      } else if (kind === 'uuid' && data.uuid) {
        form.setFieldsValue({ uuid: data.uuid });
      } else if ((kind === 'password' || kind === 'ss-password') && data.password) {
        form.setFieldsValue({ password: data.password });
      } else if (kind === 'short-id' && data.short_id) {
        form.setFieldsValue({ reality_short_id: [data.short_id] });
      }
    } catch (e: any) {
      // keep silent-ish; antd message may not be imported
      console.error(e);
    }
  };
  if (!protocol) {
    return (
      <Alert type="info" showIcon message={t('listeners.selectProtocolFirst')} style={{ marginBottom: 16 }} />
    );
  }

  return (
    <Space direction="vertical" style={{ width: '100%' }} size="middle">
      <Alert type="info" showIcon message={t('listeners.usersHint')} />

      {TRANSPORT_PROTOCOLS.has(protocol) && (
        <>
          <Divider titlePlacement="start" plain>{t('listeners.sectionTransport')}</Divider>
          <Form.Item name="transport_layer" label={t('listeners.transportLayer') || 'Transport'} initialValue="raw">
            <Radio.Group optionType="button" buttonStyle="solid">
              <Radio.Button value="raw">TCP</Radio.Button>
              <Radio.Button value="ws">WebSocket</Radio.Button>
              <Radio.Button value="grpc">gRPC</Radio.Button>
              <Radio.Button value="xhttp">XHTTP</Radio.Button>
            </Radio.Group>
          </Form.Item>
        </>
      )}
      {(TLS_PROTOCOLS.has(protocol) || REALITY_PROTOCOLS.has(protocol)) && (
        <>
          <Divider titlePlacement="start" plain>{t('listeners.sectionSecurity') || 'Security'}</Divider>
          <Form.Item name="security_layer" label={t('listeners.securityLayer') || 'Security'} initialValue="none">
            <Radio.Group optionType="button" buttonStyle="solid">
              <Radio.Button value="none">None</Radio.Button>
              <Radio.Button value="tls">TLS</Radio.Button>
              {REALITY_PROTOCOLS.has(protocol) && <Radio.Button value="reality">Reality</Radio.Button>}
            </Radio.Group>
          </Form.Item>
        </>
      )}

      {/* ---- Protocol core options ---- */}
      {protocol === 'shadowsocks' && (
        <>
          <Divider titlePlacement="start" plain>{t('listeners.sectionProtocol')}</Divider>
          <Form.Item name="cipher" label={t('listeners.cipher')} rules={[{ required: true }]}>
            <Select options={SS_CIPHERS.map((c) => ({ value: c, label: c }))} showSearch />
          </Form.Item>
          <Form.Item name="password" label={t('listeners.password')} tooltip={t('listeners.passwordOptionalHint') || 'Leave empty to auto-generate'}>
            <Input.Password placeholder={t('listeners.passwordOptionalPlaceholder') || 'auto'} addonAfter={<Button type="link" size="small" onClick={() => gen('ss-password', form.getFieldValue('cipher'))}>{t('common.generate') || 'Generate'}</Button>} />
          </Form.Item>
        </>
      )}

      {protocol === 'snell' && (
        <>
          <Divider titlePlacement="start" plain>{t('listeners.sectionProtocol')}</Divider>
          <Form.Item name="psk" label={t('listeners.psk')} rules={[{ required: true }]}>
            <Input.Password />
          </Form.Item>
          <Form.Item name="version" label={t('listeners.snellVersion')} initialValue={3}>
            <Select options={[1, 2, 3].map((v) => ({ value: v, label: String(v) }))} />
          </Form.Item>
          <Form.Item name="obfs_opts_mode" label={t('listeners.obfsOptsMode')}>
            <Select allowClear options={[{ value: 'http', label: 'http' }, { value: 'tls', label: 'tls' }]} />
          </Form.Item>
          <Form.Item name="obfs_opts_host" label={t('listeners.obfsOptsHost')}>
            <Input placeholder="www.example.com" />
          </Form.Item>
        </>
      )}

      {protocol === 'vmess' && (
        <>
          <Divider titlePlacement="start" plain>{t('listeners.sectionProtocol')}</Divider>
          <Form.Item name="alterId" label={t('listeners.alterId')} tooltip={t('listeners.alterIdHint')}>
            <InputNumber min={0} max={65535} style={{ width: '100%' }} placeholder="0" />
          </Form.Item>
        </>
      )}

      {protocol === 'vless' && (
        <>
          <Divider titlePlacement="start" plain>{t('listeners.sectionProtocol')}</Divider>
          <Form.Item name="flow" label={t('listeners.flow')} tooltip={t('listeners.flowHint')}>
            <Select allowClear options={[{ value: 'xtls-rprx-vision', label: 'xtls-rprx-vision' }]} />
          </Form.Item>
          <Form.Item name="decryption" label={t('listeners.decryption')} tooltip={t('listeners.decryptionHint')}>
            <Input.TextArea rows={2} placeholder="mlkem768x25519plus...." />
          </Form.Item>
          <Form.Item name="encryption" label={t('listeners.encryption')} tooltip={t('listeners.encryptionHint')}>
            <Input.TextArea rows={2} placeholder="mlkem768x25519plus...." />
          </Form.Item>
        </>
      )}

      {TRANSPORT_PROTOCOLS.has(protocol) && (
        <Form.Item noStyle shouldUpdate={(a, b) => a.transport_layer !== b.transport_layer}>
          {({ getFieldValue }) => {
            const layer = getFieldValue('transport_layer') || 'raw';
            return (
              <>
                {layer === 'ws' && (
                  <Form.Item name="ws-path" label={t('listeners.wsPath')} tooltip={t('listeners.wsPathHint')} rules={[{ required: true }]}>
                    <Input placeholder="/" />
                  </Form.Item>
                )}
                {layer === 'grpc' && (
                  <Form.Item name="grpc-service-name" label={t('listeners.grpcServiceName')} tooltip={t('listeners.grpcHint')} rules={[{ required: true }]}>
                    <Input placeholder="GunService" />
                  </Form.Item>
                )}
              </>
            );
          }}
        </Form.Item>
      )}

      {protocol === 'trojan' && (
        <EnableSection name="ss_option_enabled" label={t('listeners.sectionSSOption')}>
          <Form.Item name="ss_option_method" label={t('listeners.ssOptionMethod')}>
            <Select
              allowClear
              options={SS_CIPHERS.filter((c) => !c.startsWith('2022')).map((c) => ({ value: c, label: c }))}
            />
          </Form.Item>
          <Form.Item name="ss_option_password" label={t('listeners.ssOptionPassword')}>
            <Input.Password />
          </Form.Item>
        </EnableSection>
      )}

      {protocol === 'hysteria2' && (
        <>
          <Divider titlePlacement="start" plain>{t('listeners.sectionProtocol')}</Divider>
          <Form.Item name="up" label={t('listeners.up')} tooltip={t('listeners.bandwidthHint')}>
            <Input placeholder="100 Mbps" />
          </Form.Item>
          <Form.Item name="down" label={t('listeners.down')} tooltip={t('listeners.bandwidthHint')}>
            <Input placeholder="100 Mbps" />
          </Form.Item>
          <Form.Item name="ignore-client-bandwidth" label={t('listeners.ignoreClientBandwidth')} valuePropName="checked">
            <Switch />
          </Form.Item>
          <Form.Item name="obfs" label={t('listeners.obfs')}>
            <Select allowClear options={[{ value: 'salamander', label: 'salamander' }]} />
          </Form.Item>
          <Form.Item name="obfs-password" label={t('listeners.obfsPassword')}>
            <Input.Password />
          </Form.Item>
          <Form.Item name="obfs-min-packet-size" label={t('listeners.obfsMinPacketSize')}>
            <InputNumber min={0} style={{ width: '100%' }} />
          </Form.Item>
          <Form.Item name="obfs-max-packet-size" label={t('listeners.obfsMaxPacketSize')}>
            <InputNumber min={0} style={{ width: '100%' }} />
          </Form.Item>
          <Form.Item name="masquerade" label={t('listeners.masquerade')}>
            <Input placeholder="https://www.example.com" />
          </Form.Item>
          <Form.Item name="alpn" label={t('listeners.alpn')}>
            <Select mode="tags" placeholder="h3" tokenSeparators={[',']} />
          </Form.Item>
          <Form.Item name="max-idle-time" label={t('listeners.maxIdleTime')}>
            <InputNumber min={0} style={{ width: '100%' }} placeholder="15000" />
          </Form.Item>
          <Form.Item name="handshake-timeout" label={t('listeners.handshakeTimeout')}>
            <InputNumber min={0} style={{ width: '100%' }} />
          </Form.Item>
          <Form.Item name="bbr-profile" label={t('listeners.bbrProfile')}>
            <Input />
          </Form.Item>
        </>
      )}

      {protocol === 'tuic' && (
        <>
          <Divider titlePlacement="start" plain>{t('listeners.sectionProtocol')}</Divider>
          <Form.Item name="token" label={t('listeners.token')} tooltip={t('listeners.tokenHint')}>
            <Input placeholder={t('listeners.tokenPlaceholder')} />
          </Form.Item>
          <Form.Item name="congestion-controller" label={t('listeners.congestionController')}>
            <Select allowClear options={['bbr', 'cubic', 'new_reno'].map((v) => ({ value: v, label: v }))} />
          </Form.Item>
          <Form.Item name="alpn" label={t('listeners.alpn')}>
            <Select mode="tags" placeholder="h3" tokenSeparators={[',']} />
          </Form.Item>
          <Form.Item name="max-idle-time" label={t('listeners.maxIdleTime')}>
            <InputNumber min={0} style={{ width: '100%' }} placeholder="15000" />
          </Form.Item>
          <Form.Item name="authentication-timeout" label={t('listeners.authenticationTimeout')}>
            <InputNumber min={0} style={{ width: '100%' }} placeholder="1000" />
          </Form.Item>
          <Form.Item name="max-udp-relay-packet-size" label={t('listeners.maxUdpRelayPacketSize')}>
            <InputNumber min={0} style={{ width: '100%' }} placeholder="1500" />
          </Form.Item>
          <Form.Item name="bbr-profile" label={t('listeners.bbrProfile')}>
            <Input />
          </Form.Item>
        </>
      )}

      {protocol === 'shadowquic' && (
        <>
          <Divider titlePlacement="start" plain>{t('listeners.sectionProtocol')}</Divider>
          <Form.Item name="alpn" label={t('listeners.alpn')}>
            <Select mode="tags" placeholder="h3" tokenSeparators={[',']} />
          </Form.Item>
          <Form.Item name="congestion-controller" label={t('listeners.congestionController')}>
            <Select allowClear options={['bbr', 'cubic', 'new_reno'].map((v) => ({ value: v, label: v }))} />
          </Form.Item>
          <Form.Item name="zero-rtt" label={t('listeners.zeroRtt')} valuePropName="checked">
            <Switch />
          </Form.Item>
          <Form.Item name="up" label={t('listeners.up')}>
            <Input placeholder="100 Mbps" />
          </Form.Item>
          <Form.Item name="down" label={t('listeners.down')}>
            <Input placeholder="100 Mbps" />
          </Form.Item>
          <Form.Item name="ignore-client-bandwidth" label={t('listeners.ignoreClientBandwidth')} valuePropName="checked">
            <Switch />
          </Form.Item>
          <Form.Item name="max-idle-time" label={t('listeners.maxIdleTime')}>
            <InputNumber min={0} style={{ width: '100%' }} />
          </Form.Item>
          <Form.Item name="cwnd" label={t('listeners.cwnd')}>
            <InputNumber min={0} style={{ width: '100%' }} />
          </Form.Item>
          <Form.Item name="bbr-profile" label={t('listeners.bbrProfile')}>
            <Input />
          </Form.Item>
          <Form.Item name="quic-versions" label={t('listeners.quicVersions')}>
            <Select mode="tags" tokenSeparators={[',']} />
          </Form.Item>
        </>
      )}

      {protocol === 'anytls' && (
        <>
          <Divider titlePlacement="start" plain>{t('listeners.sectionProtocol')}</Divider>
          <Form.Item name="padding-scheme" label={t('listeners.paddingScheme')}>
            <Input.TextArea rows={2} />
          </Form.Item>
        </>
      )}

      {protocol === 'mieru' && (
        <>
          <Divider titlePlacement="start" plain>{t('listeners.sectionProtocol')}</Divider>
          <Form.Item name="transport" label={t('listeners.transport')} rules={[{ required: true }]}>
            <Select options={[{ value: 'TCP', label: 'TCP' }, { value: 'UDP', label: 'UDP' }]} />
          </Form.Item>
          <Form.Item name="traffic-pattern" label={t('listeners.trafficPattern')}>
            <Input />
          </Form.Item>
          <Form.Item name="user-hint-is-mandatory" label={t('listeners.userHintMandatory')} valuePropName="checked">
            <Switch />
          </Form.Item>
        </>
      )}

      {protocol === 'sudoku' && (
        <>
          <Divider titlePlacement="start" plain>{t('listeners.sectionProtocol')}</Divider>
          <Form.Item name="key" label={t('listeners.sudokuKey')} rules={[{ required: true }]}>
            <Input.Password />
          </Form.Item>
          <Form.Item name="aead-method" label={t('listeners.aeadMethod')}>
            <Select
              allowClear
              options={['chacha20-poly1305', 'aes-128-gcm', 'aes-256-gcm'].map((v) => ({ value: v, label: v }))}
            />
          </Form.Item>
          <Form.Item name="padding-min" label={t('listeners.paddingMin')}>
            <InputNumber min={0} style={{ width: '100%' }} />
          </Form.Item>
          <Form.Item name="padding-max" label={t('listeners.paddingMax')}>
            <InputNumber min={0} style={{ width: '100%' }} />
          </Form.Item>
          <Form.Item name="table-type" label={t('listeners.tableType')}>
            <Input />
          </Form.Item>
          <Form.Item name="handshake-timeout" label={t('listeners.handshakeTimeout')}>
            <InputNumber min={0} style={{ width: '100%' }} />
          </Form.Item>
          <Form.Item name="enable-pure-downlink" label={t('listeners.enablePureDownlink')} valuePropName="checked">
            <Switch />
          </Form.Item>
        </>
      )}

      {protocol === 'trusttunnel' && (
        <>
          <Divider titlePlacement="start" plain>{t('listeners.sectionProtocol')}</Divider>
          <Form.Item name="network" label={t('listeners.network')}>
            <Select allowClear options={['tcp', 'udp'].map((v) => ({ value: v, label: v }))} />
          </Form.Item>
          <Form.Item name="congestion-controller" label={t('listeners.congestionController')}>
            <Select allowClear options={['bbr', 'cubic', 'new_reno'].map((v) => ({ value: v, label: v }))} />
          </Form.Item>
          <Form.Item name="bbr-profile" label={t('listeners.bbrProfile')}>
            <Input />
          </Form.Item>
        </>
      )}

      {/* ---- TLS certificates ---- */}
      {TLS_PROTOCOLS.has(protocol) && (
        <>
          <Divider titlePlacement="start" plain>{t('listeners.sectionTLS')}</Divider>
          <Text type="secondary" style={{ display: 'block', marginBottom: 8 }}>
            {t('listeners.tlsPairHint')}
          </Text>
          <Form.Item name="certificate" label={t('listeners.certificate')}>
            <Input.TextArea rows={2} placeholder="./server.crt" />
          </Form.Item>
          <Form.Item name="private-key" label={t('listeners.privateKey')}>
            <Input.TextArea rows={2} placeholder="./server.key" />
          </Form.Item>
          <Form.Item name="client-auth-type" label={t('listeners.clientAuthType')}>
            <Select
              allowClear
              options={['request', 'require-any', 'verify-if-given', 'require-and-verify'].map((v) => ({
                value: v,
                label: v,
              }))}
            />
          </Form.Item>
          <Form.Item name="client-auth-cert" label={t('listeners.clientAuthCert')}>
            <Input.TextArea rows={2} />
          </Form.Item>
          <Form.Item name="ech-key" label={t('listeners.echKey')}>
            <Input.TextArea rows={2} />
          </Form.Item>
          {ALLOW_INSECURE_PROTOCOLS.has(protocol) && (
            <Form.Item
              name="allow-insecure"
              label={t('listeners.allowInsecure')}
              valuePropName="checked"
              tooltip={t('listeners.allowInsecureHint')}
            >
              <Switch />
            </Form.Item>
          )}
        </>
      )}

      {/* ---- Reality ---- */}
      {REALITY_PROTOCOLS.has(protocol) && (
        <EnableSection
          name="reality_enabled"
          label={t('listeners.sectionReality')}
          hint={t('listeners.realityExclusiveHint')}
        >
          <Form.Item name="reality_dest" label={t('listeners.realityDest')} tooltip="Default: www.microsoft.com:443">
            <Input placeholder="www.microsoft.com:443" />
          </Form.Item>
          <Form.Item
            name="reality_private_key"
            label={t('listeners.realityPrivateKey')}
            tooltip={t('listeners.realityKeyHint') || 'Leave empty to auto-generate on save'}
          >
            <Input.Password placeholder="auto" addonAfter={<Button type="link" size="small" onClick={() => gen('reality')}>{t('common.generate') || 'Generate'}</Button>} />
          </Form.Item>
          <Form.Item name="reality_short_id" label={t('listeners.realityShortId')}>
            <Select mode="tags" placeholder="auto" tokenSeparators={[',']} />
          </Form.Item>
          <Form.Item name="reality_server_names" label={t('listeners.realityServerNames')}>
            <Select mode="tags" placeholder="www.example.com" tokenSeparators={[',']} />
          </Form.Item>
        </EnableSection>
      )}

      {/* ---- simple-obfs (SS) ---- */}
      {SIMPLE_OBFS_PROTOCOLS.has(protocol) && (
        <EnableSection name="simple_obfs_enabled" label={t('listeners.sectionSimpleObfs')}>
          <Form.Item name="simple_obfs_mode" label={t('listeners.simpleObfsMode')}>
            <Select options={[{ value: 'http', label: 'http' }, { value: 'tls', label: 'tls' }]} />
          </Form.Item>
        </EnableSection>
      )}

      {/* ---- shadow-tls ---- */}
      {WRAPPER_TLS_PROTOCOLS.has(protocol) && (
        <EnableSection
          name="shadow_tls_enabled"
          label={t('listeners.sectionShadowTLS')}
          hint={t('listeners.shadowTlsHint')}
        >
          <Form.Item name="shadow_tls_version" label={t('listeners.shadowTlsVersion')}>
            <Select options={[1, 2, 3].map((v) => ({ value: v, label: `v${v}` }))} />
          </Form.Item>
          <Form.Item name="shadow_tls_password" label={t('listeners.shadowTlsPassword')} tooltip={t('listeners.shadowTlsPasswordHint')}>
            <Input.Password />
          </Form.Item>
          <Form.Item name="shadow_tls_handshake_dest" label={t('listeners.shadowTlsHandshakeDest')}>
            <Input placeholder="www.example.com:443" />
          </Form.Item>
          <Form.Item name="shadow_tls_handshake_proxy" label={t('listeners.shadowTlsHandshakeProxy')}>
            <Input />
          </Form.Item>
          <Form.List name="shadow_tls_users">
            {(fields, { add, remove }) => (
              <>
                <Text type="secondary">{t('listeners.shadowTlsUsers')} (v3)</Text>
                {fields.map((field) => (
                  <Space key={field.key} align="baseline" style={{ display: 'flex', marginBottom: 8 }}>
                    <Form.Item {...field} name={[field.name, 'name']} rules={[{ required: true }]}>
                      <Input placeholder={t('common.username')} />
                    </Form.Item>
                    <Form.Item {...field} name={[field.name, 'password']} rules={[{ required: true }]}>
                      <Input.Password placeholder={t('common.password')} />
                    </Form.Item>
                    <MinusCircleOutlined onClick={() => remove(field.name)} />
                  </Space>
                ))}
                <Button type="dashed" onClick={() => add()} block icon={<PlusOutlined />}>
                  {t('listeners.addUser')}
                </Button>
              </>
            )}
          </Form.List>
        </EnableSection>
      )}

      {/* ---- res-tls ---- */}
      {WRAPPER_TLS_PROTOCOLS.has(protocol) && (
        <EnableSection name="res_tls_enabled" label={t('listeners.sectionResTLS')} hint={t('listeners.resTlsHint')}>
          <Form.Item name="res_tls_dest" label={t('listeners.resTlsDest')}>
            <Input placeholder="www.example.com:443" />
          </Form.Item>
          <Form.Item name="res_tls_password" label={t('listeners.resTlsPassword')}>
            <Input.Password />
          </Form.Item>
          <Form.Item name="res_tls_restls_script" label={t('listeners.resTlsScript')}>
            <Input />
          </Form.Item>
          <Form.Item name="res_tls_min_record_len" label={t('listeners.resTlsMinRecordLen')}>
            <InputNumber min={0} style={{ width: '100%' }} />
          </Form.Item>
          <Form.Item name="res_tls_proxy" label={t('listeners.resTlsProxy')}>
            <Input />
          </Form.Item>
          <Form.Item name="res_tls_rate_limit" label={t('listeners.rateLimit')} tooltip={t('listeners.rateLimitHint')}>
            <InputNumber min={0} style={{ width: '100%' }} />
          </Form.Item>
        </EnableSection>
      )}

      {/* ---- jls-config ---- */}
      {WRAPPER_TLS_PROTOCOLS.has(protocol) && (
        <EnableSection name="jls_enabled" label={t('listeners.sectionJLS')} hint={t('listeners.jlsHint')}>
          <Form.Item name="jls_dest" label={t('listeners.jlsDest')}>
            <Input placeholder="www.example.com:443" />
          </Form.Item>
          <Form.Item name="jls_sni" label={t('listeners.jlsSni')} tooltip={t('listeners.jlsSniHint')}>
            <Input placeholder="www.example.com" />
          </Form.Item>
          <Form.Item name="jls_alpn" label={t('listeners.alpn')}>
            <Select mode="tags" tokenSeparators={[',']} placeholder="h2, http/1.1" />
          </Form.Item>
          <Form.Item name="jls_proxy" label={t('listeners.jlsProxy')}>
            <Input />
          </Form.Item>
          <Form.Item name="jls_rate_limit" label={t('listeners.rateLimit')} tooltip={t('listeners.rateLimitHint')}>
            <InputNumber min={0} style={{ width: '100%' }} />
          </Form.Item>
          <Form.List name="jls_users">
            {(fields, { add, remove }) => (
              <>
                <Text type="secondary">{t('listeners.jlsUsers')}</Text>
                {fields.map((field) => (
                  <Space key={field.key} align="baseline" style={{ display: 'flex', marginBottom: 8 }}>
                    <Form.Item {...field} name={[field.name, 'username']} rules={[{ required: true }]}>
                      <Input placeholder={t('common.username')} />
                    </Form.Item>
                    <Form.Item {...field} name={[field.name, 'password']} rules={[{ required: true }]}>
                      <Input.Password placeholder={t('common.password')} />
                    </Form.Item>
                    <MinusCircleOutlined onClick={() => remove(field.name)} />
                  </Space>
                ))}
                <Button type="dashed" onClick={() => add()} block icon={<PlusOutlined />}>
                  {t('listeners.addUser')}
                </Button>
              </>
            )}
          </Form.List>
        </EnableSection>
      )}

      {/* ---- mux-option ---- */}
      {MUX_PROTOCOLS.has(protocol) && (
        <EnableSection name="mux_enabled" label={t('listeners.sectionMux')}>
          <Form.Item name="mux_padding" label={t('listeners.muxPadding')} valuePropName="checked">
            <Switch />
          </Form.Item>
          <Form.Item name="mux_protocol" label={t('listeners.muxProtocol')}>
            <Input />
          </Form.Item>
          <Form.Item name="mux_max_connections" label={t('listeners.muxMaxConnections')}>
            <InputNumber min={0} style={{ width: '100%' }} />
          </Form.Item>
          <Form.Item name="mux_min_streams" label={t('listeners.muxMinStreams')}>
            <InputNumber min={0} style={{ width: '100%' }} />
          </Form.Item>
          <Form.Item name="mux_max_streams" label={t('listeners.muxMaxStreams')}>
            <InputNumber min={0} style={{ width: '100%' }} />
          </Form.Item>
          <Form.Item name="mux_statistic" label={t('listeners.muxStatistic')} valuePropName="checked">
            <Switch />
          </Form.Item>
          <Form.Item name="mux_only_tcp" label={t('listeners.muxOnlyTcp')} valuePropName="checked">
            <Switch />
          </Form.Item>
          <Form.Item name="mux_brutal_enabled" label={t('listeners.muxBrutal')} valuePropName="checked">
            <Switch />
          </Form.Item>
          <Form.Item noStyle shouldUpdate={(p, c) => p.mux_brutal_enabled !== c.mux_brutal_enabled}>
            {({ getFieldValue }) =>
              getFieldValue('mux_brutal_enabled') ? (
                <>
                  <Form.Item name="mux_brutal_up" label={t('listeners.muxBrutalUp')}>
                    <InputNumber min={0} style={{ width: '100%' }} placeholder="1000" />
                  </Form.Item>
                  <Form.Item name="mux_brutal_down" label={t('listeners.muxBrutalDown')}>
                    <InputNumber min={0} style={{ width: '100%' }} placeholder="1000" />
                  </Form.Item>
                </>
              ) : null
            }
          </Form.Item>
        </EnableSection>
      )}

      {/* ---- kcp-tun (SS) ---- */}
      {KCP_TUN_PROTOCOLS.has(protocol) && (
        <EnableSection name="kcp_tun_enabled" label={t('listeners.sectionKcpTun')}>
          <Form.Item name="kcp_tun_key" label={t('listeners.kcpTunKey')}>
            <Input.Password />
          </Form.Item>
          <Form.Item name="kcp_tun_crypt" label={t('listeners.kcpTunCrypt')}>
            <Select
              allowClear
              options={[
                'aes', 'aes-128', 'aes-128-gcm', 'aes-192', 'salsa20', 'blowfish', 'twofish',
                'cast5', '3des', 'tea', 'xtea', 'xor', 'none', 'null',
              ].map((v) => ({ value: v, label: v }))}
            />
          </Form.Item>
          <Form.Item name="kcp_tun_mode" label={t('listeners.kcpTunMode')}>
            <Select allowClear options={['fast3', 'fast2', 'fast', 'normal', 'manual'].map((v) => ({ value: v, label: v }))} />
          </Form.Item>
          <Form.Item name="kcp_tun_conn" label={t('listeners.kcpTunConn')}>
            <InputNumber min={1} style={{ width: '100%' }} />
          </Form.Item>
          <Form.Item name="kcp_tun_mtu" label="MTU">
            <InputNumber min={0} style={{ width: '100%' }} placeholder="1350" />
          </Form.Item>
          <Form.Item name="kcp_tun_sndwnd" label={t('listeners.kcpTunSndwnd')}>
            <InputNumber min={0} style={{ width: '100%' }} />
          </Form.Item>
          <Form.Item name="kcp_tun_rcvwnd" label={t('listeners.kcpTunRcvwnd')}>
            <InputNumber min={0} style={{ width: '100%' }} />
          </Form.Item>
          <Form.Item name="kcp_tun_nocomp" label={t('listeners.kcpTunNocomp')} valuePropName="checked">
            <Switch />
          </Form.Item>
        </EnableSection>
      )}

      {/* ---- xhttp (VLESS) ---- */}
      {XHTTP_PROTOCOLS.has(protocol) && (
        <EnableSection name="xhttp_enabled" label={t('listeners.sectionXHTTP')} hint={t('listeners.xhttpHint')}>
          <Form.Item name="xhttp_path" label={t('listeners.xhttpPath')}>
            <Input placeholder="/" />
          </Form.Item>
          <Form.Item name="xhttp_host" label={t('listeners.xhttpHost')}>
            <Input />
          </Form.Item>
          <Form.Item name="xhttp_mode" label={t('listeners.xhttpMode')}>
            <Select
              allowClear
              options={['auto', 'stream-one', 'stream-up', 'packet-up'].map((v) => ({ value: v, label: v }))}
            />
          </Form.Item>
        </EnableSection>
      )}

      {/* ---- mkcp (VMess) ---- */}
      {MKCP_PROTOCOLS.has(protocol) && (
        <EnableSection name="mkcp_enabled" label={t('listeners.sectionMkcp')} hint={t('listeners.mkcpHint')}>
          <Form.Item name="mkcp_mtu" label="MTU">
            <InputNumber min={0} style={{ width: '100%' }} placeholder="1350" />
          </Form.Item>
          <Form.Item name="mkcp_tti" label="TTI">
            <InputNumber min={0} style={{ width: '100%' }} placeholder="50" />
          </Form.Item>
          <Form.Item name="mkcp_uplink" label={t('listeners.mkcpUplink')}>
            <InputNumber min={0} style={{ width: '100%' }} />
          </Form.Item>
          <Form.Item name="mkcp_downlink" label={t('listeners.mkcpDownlink')}>
            <InputNumber min={0} style={{ width: '100%' }} />
          </Form.Item>
          <Form.Item name="mkcp_congestion" label={t('listeners.mkcpCongestion')} valuePropName="checked">
            <Switch />
          </Form.Item>
          <Form.Item name="mkcp_write_buffer" label={t('listeners.mkcpWriteBuffer')}>
            <InputNumber min={0} style={{ width: '100%' }} />
          </Form.Item>
          <Form.Item name="mkcp_read_buffer" label={t('listeners.mkcpReadBuffer')}>
            <InputNumber min={0} style={{ width: '100%' }} />
          </Form.Item>
          <Form.Item name="mkcp_seed" label={t('listeners.mkcpSeed')}>
            <Input />
          </Form.Item>
          <Form.Item name="mkcp_header" label={t('listeners.mkcpHeader')}>
            <Select
              allowClear
              options={['none', 'srtp', 'utp', 'wechat-video', 'dtls', 'wireguard'].map((v) => ({ value: v, label: v }))}
            />
          </Form.Item>
        </EnableSection>
      )}

      {MEKYA_PROTOCOLS.has(protocol) && (
        <EnableSection name="mekya_enabled" label={t('listeners.sectionMekya')} hint={t('listeners.mekyaHint')}>
          <Form.Item name="mekya_max_write_size" label={t('listeners.mekyaMaxWriteSize')} tooltip={t('listeners.mekyaMaxWriteSizeHint')}>
            <InputNumber min={0} style={{ width: '100%' }} placeholder="10485760" />
          </Form.Item>
          <Form.Item name="mekya_max_write_duration_ms" label={t('listeners.mekyaMaxWriteDuration')} tooltip={t('listeners.mekyaMaxWriteDurationHint')}>
            <InputNumber min={0} style={{ width: '100%' }} placeholder="5000" />
          </Form.Item>
          <Form.Item name="mekya_max_simultaneous_write_connection" label={t('listeners.mekyaMaxSimultaneous')} tooltip={t('listeners.mekyaMaxSimultaneousHint')}>
            <InputNumber min={0} style={{ width: '100%' }} placeholder="128" />
          </Form.Item>
          <Form.Item name="mekya_packet_writing_buffer" label={t('listeners.mekyaPacketBuffer')} tooltip={t('listeners.mekyaPacketBufferHint')}>
            <InputNumber min={0} style={{ width: '100%' }} placeholder="65536" />
          </Form.Item>
          <Divider titlePlacement="start" plain style={{ marginTop: 8 }}>{t('listeners.mekyaKcpSection')}</Divider>
          <Form.Item name="mekya_kcp_mtu" label="MTU">
            <InputNumber min={0} style={{ width: '100%' }} placeholder="1350" />
          </Form.Item>
          <Form.Item name="mekya_kcp_tti" label="TTI">
            <InputNumber min={0} style={{ width: '100%' }} placeholder="15" />
          </Form.Item>
          <Form.Item name="mekya_kcp_uplink" label={t('listeners.mkcpUplink')}>
            <InputNumber min={0} style={{ width: '100%' }} placeholder="40" />
          </Form.Item>
          <Form.Item name="mekya_kcp_downlink" label={t('listeners.mkcpDownlink')}>
            <InputNumber min={0} style={{ width: '100%' }} placeholder="2000" />
          </Form.Item>
          <Form.Item name="mekya_kcp_congestion" label={t('listeners.mkcpCongestion')} valuePropName="checked">
            <Switch />
          </Form.Item>
          <Form.Item name="mekya_kcp_write_buffer" label={t('listeners.mkcpWriteBuffer')}>
            <InputNumber min={0} style={{ width: '100%' }} placeholder="67108864" />
          </Form.Item>
          <Form.Item name="mekya_kcp_read_buffer" label={t('listeners.mkcpReadBuffer')}>
            <InputNumber min={0} style={{ width: '100%' }} placeholder="67108864" />
          </Form.Item>
          <Form.Item name="mekya_kcp_seed" label={t('listeners.mkcpSeed')}>
            <Input />
          </Form.Item>
          <Form.Item name="mekya_kcp_header" label={t('listeners.mkcpHeader')}>
            <Select
              allowClear
              options={['none', 'srtp', 'utp', 'wechat-video', 'dtls', 'wireguard'].map((v) => ({ value: v, label: v }))}
            />
          </Form.Item>
        </EnableSection>
      )}

      {/* ---- jls-upstream (ShadowQUIC) ---- */}
      {protocol === 'shadowquic' && (
        <EnableSection name="jls_upstream_enabled" label={t('listeners.sectionJlsUpstream')}>
          <Form.Item name="jls_upstream_addr" label={t('listeners.jlsUpstreamAddr')}>
            <Input placeholder="www.example.com:443" />
          </Form.Item>
          <Form.Item name="jls_upstream_sni" label={t('listeners.jlsSni')}>
            <Input />
          </Form.Item>
          <Form.Item name="jls_upstream_proxy" label={t('listeners.jlsProxy')}>
            <Input />
          </Form.Item>
          <Form.Item name="jls_upstream_rate_limit" label={t('listeners.rateLimit')}>
            <InputNumber min={0} style={{ width: '100%' }} />
          </Form.Item>
        </EnableSection>
      )}

      {/* ---- realm-opts (Hysteria2) ---- */}
      {protocol === 'hysteria2' && (
        <EnableSection name="realm_enabled" label={t('listeners.sectionRealm')}>
          <Form.Item name="realm_server_url" label={t('listeners.realmServerUrl')}>
            <Input />
          </Form.Item>
          <Form.Item name="realm_token" label={t('listeners.realmToken')}>
            <Input.Password />
          </Form.Item>
          <Form.Item name="realm_id" label={t('listeners.realmId')}>
            <Input />
          </Form.Item>
          <Form.Item name="realm_stun" label={t('listeners.realmStun')}>
            <Select mode="tags" tokenSeparators={[',']} />
          </Form.Item>
          <Form.Item name="realm_proxy" label={t('listeners.realmProxy')}>
            <Input />
          </Form.Item>
          <Form.Item name="realm_skip_cert" label={t('listeners.realmSkipCert')} valuePropName="checked">
            <Switch />
          </Form.Item>
        </EnableSection>
      )}
    </Space>
  );
};

export default ListenerConfigFields;
