import React from 'react';
import {
  Form, Input, InputNumber, Select, Switch, Divider, Alert, Space, Typography, Button, Card, Radio,
} from 'antd';
import { MinusCircleOutlined, PlusOutlined } from '@ant-design/icons';
import { useI18n } from '../i18n';

const { Text } = Typography;

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
const WRAPPER_TLS_PROTOCOLS = new Set([
  'shadowsocks', 'snell', 'vmess', 'vless', 'trojan', 'anytls',
]);
const MUX_PROTOCOLS = new Set(['shadowsocks', 'vmess', 'vless', 'trojan']);
const SIMPLE_OBFS_PROTOCOLS = new Set(['shadowsocks']);
const KCP_TUN_PROTOCOLS = new Set(['shadowsocks']);
const XHTTP_PROTOCOLS = new Set(['vless']);
const MKCP_PROTOCOLS = new Set(['vmess']);
const MEKYA_PROTOCOLS = new Set(['vmess']);
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
  const values: Record<string, any> = {};
  for (const [k, v] of Object.entries(cfg)) {
    if (v !== null && typeof v === 'object' && !Array.isArray(v)) continue;
    values[k] = v;
  }

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

  if (cfg['ss-option'] && typeof cfg['ss-option'] === 'object') {
    values.ss_option_enabled = !!cfg['ss-option'].enabled;
    values.ss_option_method = cfg['ss-option'].method;
    values.ss_option_password = cfg['ss-option'].password;
  }

  if (cfg['simple-obfs'] && typeof cfg['simple-obfs'] === 'object') {
    values.simple_obfs_enabled = !!cfg['simple-obfs'].enable;
    values.simple_obfs_mode = cfg['simple-obfs'].mode;
  }

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

  if (cfg['xhttp-config'] && typeof cfg['xhttp-config'] === 'object') {
    const x = cfg['xhttp-config'];
    values.xhttp_enabled = true;
    values.xhttp_path = x.path;
    values.xhttp_host = x.host;
    values.xhttp_mode = x.mode;
  }

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

  if (cfg['obfs-opts'] && typeof cfg['obfs-opts'] === 'object') {
    values.obfs_opts_mode = cfg['obfs-opts'].mode;
    values.obfs_opts_host = cfg['obfs-opts'].host;
  }

  if (cfg['jls-upstream'] && typeof cfg['jls-upstream'] === 'object') {
    const j = cfg['jls-upstream'];
    values.jls_upstream_enabled = true;
    values.jls_upstream_addr = j.addr;
    values.jls_upstream_sni = j.sni;
    values.jls_upstream_proxy = j.proxy;
    values.jls_upstream_rate_limit = j['rate-limit'];
  }

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

  if (cfg['padding-min'] != null) values['padding-min'] = cfg['padding-min'];
  if (cfg['padding-max'] != null) values['padding-max'] = cfg['padding-max'];
  if (cfg['table-type']) values['table-type'] = cfg['table-type'];
  if (cfg['handshake-timeout'] != null) values['handshake-timeout'] = cfg['handshake-timeout'];
  if (cfg['enable-pure-downlink'] != null) values['enable-pure-downlink'] = cfg['enable-pure-downlink'];

  if (cfg.network) values.network = cfg.network;
  if (cfg['bbr-profile']) values['bbr-profile'] = cfg['bbr-profile'];
  if (cfg['quic-versions']) values['quic-versions'] = asStringList(cfg['quic-versions']);
  if (cfg.cwnd != null) values.cwnd = cfg.cwnd;

  if (cfg.alpn && !Array.isArray(cfg.alpn)) {
    values.alpn = [cfg.alpn];
  }

  if (Array.isArray(cfg.token)) {
    values.token = cfg.token.join(',');
  }

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

const FORM_OWNED_KEYS = new Set([
  'cipher', 'password', 'psk', 'version', 'alterId', 'flow', 'decryption',
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

function toNum(v: any): number | undefined {
  if (v === undefined || v === null || v === '') return undefined;
  const n = typeof v === 'number' ? v : Number(v);
  return Number.isFinite(n) ? n : undefined;
}

function toInt(v: any): number | undefined {
  const n = toNum(v);
  return n === undefined ? undefined : Math.trunc(n);
}

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

  if (SIMPLE_OBFS_PROTOCOLS.has(protocol) && values.simple_obfs_enabled) {
    cfg['simple-obfs'] = cleanObj({
      enable: true,
      mode: values.simple_obfs_mode,
    }) || { enable: true };
  }

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

  if (XHTTP_PROTOCOLS.has(protocol) && values.xhttp_enabled) {
    const xhttp = cleanObj({
      path: values.xhttp_path,
      host: values.xhttp_host,
      mode: values.xhttp_mode,
    });
    cfg['xhttp-config'] = xhttp || { path: '/' };
  }

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

  if (protocol === 'shadowquic' && values.jls_upstream_enabled) {
    const upstream = cleanObj({
      addr: values.jls_upstream_addr,
      sni: values.jls_upstream_sni,
      proxy: values.jls_upstream_proxy,
      'rate-limit': values.jls_upstream_rate_limit,
    });
    if (upstream) cfg['jls-upstream'] = upstream;
  }

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
          <Form.Item name="password" label={t('listeners.password')} tooltip={t('listeners.passwordOptionalHint')}>
            <Input.Password placeholder={t('listeners.passwordOptionalPlaceholder')} />
          </Form.Item>
        </>
      )}

      {protocol === 'vless' && (
        <>
          <Divider titlePlacement="start" plain>{t('listeners.sectionProtocol')}</Divider>
          <Form.Item name="flow" label={t('listeners.flow')} tooltip={t('listeners.flowHint')}>
            <Select allowClear options={[{ value: 'xtls-rprx-vision', label: 'xtls-rprx-vision' }]} />
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

      {TLS_PROTOCOLS.has(protocol) && (
        <>
          <Divider titlePlacement="start" plain>{t('listeners.sectionTLS')}</Divider>
          <Form.Item name="certificate" label={t('listeners.certificate')}>
            <Input.TextArea rows={2} placeholder="./server.crt" />
          </Form.Item>
          <Form.Item name="private-key" label={t('listeners.privateKey')}>
            <Input.TextArea rows={2} placeholder="./server.key" />
          </Form.Item>
        </>
      )}

      {REALITY_PROTOCOLS.has(protocol) && (
        <EnableSection
          name="reality_enabled"
          label={t('listeners.sectionReality')}
          hint={t('listeners.realityExclusiveHint')}
        >
          <Form.Item name="reality_dest" label={t('listeners.realityDest')} rules={[{ required: true }]}>
            <Input placeholder="www.example.com:443" />
          </Form.Item>
          <Form.Item name="reality_private_key" label={t('listeners.realityPrivateKey')} rules={[{ required: true }]}>
            <Input.Password />
          </Form.Item>
          <Form.Item name="reality_short_id" label={t('listeners.realityShortId')}>
            <Select mode="tags" placeholder="0123456789abcdef" tokenSeparators={[',']} />
          </Form.Item>
          <Form.Item name="reality_server_names" label={t('listeners.realityServerNames')}>
            <Select mode="tags" placeholder="www.example.com" tokenSeparators={[',']} />
          </Form.Item>
        </EnableSection>
      )}

      {protocol === 'hysteria2' && (
        <>
          <Divider titlePlacement="start" plain>{t('listeners.sectionProtocol')}</Divider>
          <Form.Item name="up" label={t('listeners.up')}><Input placeholder="100 Mbps" /></Form.Item>
          <Form.Item name="down" label={t('listeners.down')}><Input placeholder="100 Mbps" /></Form.Item>
          <Form.Item name="alpn" label={t('listeners.alpn")}><Select mode="tags" placeholder="h3" tokenSeparators={[',']} /></Form.Item>
        </>
      )}

      {protocol === 'shadowquic' && (
        <>
          <Divider titlePlacement="start" plain>{t('listeners.sectionProtocol')}</Divider>
          <Form.Item name="alpn" label={t('listeners.alpn")}><Select mode="tags" placeholder="h3" tokenSeparators={[',']} /></Form.Item>
          <Form.Item name="congestion-controller" label={t('listeners.congestionController')}>
            <Select allowClear options={['bbr', 'cubic', 'new_reno'].map((v) => ({ value: v, label: v }))} />
          </Form.Item>
          <EnableSection name="jls_upstream_enabled" label={t('listeners.sectionJlsUpstream')}>
            <Form.Item name="jls_upstream_addr" label={t('listeners.jlsUpstreamAddr")}><Input placeholder="www.example.com:443" /></Form.Item>
            <Form.Item name="jls_upstream_sni" label={t('listeners.jlsSni")}><Input /></Form.Item>
          </EnableSection>
        </>
      )}
    </Space>
  );
};

export default ListenerConfigFields;
