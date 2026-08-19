/** Declarative listener form schema — mirrors Ant Design ListenerConfigFields field coverage. */
import { SS_CIPHERS, protocolSupportsUDP } from './listenerConfig'

export type FieldType = 'string' | 'text' | 'secret' | 'integer' | 'boolean' | 'select' | 'radio' | 'tags'

export interface FormField {
  name: string
  type: FieldType
  labelKey?: string
  label?: string
  hintKey?: string
  options?: string[]
  optionLabels?: string[]
  required?: boolean
  default?: any
}

export interface FormSection {
  id: string
  titleKey: string
  /** Show when protocol matches (or special tokens) */
  when: string | string[]
  fields: FormField[]
}

const TRANSPORT = new Set(['vmess', 'vless', 'trojan'])
const TLS = new Set(['vmess', 'vless', 'trojan', 'hysteria2', 'tuic', 'anytls', 'trusttunnel'])
const REALITY = new Set(['vmess', 'vless', 'trojan'])
const WRAPPER = new Set(['shadowsocks', 'snell', 'vmess', 'vless', 'trojan', 'anytls'])
const MUX = new Set(['shadowsocks', 'vmess', 'vless', 'trojan'])
const SIMPLE_OBFS = new Set(['shadowsocks'])
const KCP_TUN = new Set(['shadowsocks'])
const XHTTP = new Set(['vless'])
const MKCP = new Set(['vmess'])
const MEKYA = new Set(['vmess'])

export function sectionVisible(section: FormSection, protocol: string, values: Record<string, any>): boolean {
  const when = Array.isArray(section.when) ? section.when : [section.when]
  return when.some((w) => {
    if (w === '*') return true
    if (w === 'transport') return TRANSPORT.has(protocol)
    if (w === 'tls_or_reality') return TLS.has(protocol) || REALITY.has(protocol)
    if (w === 'reality_fields') return REALITY.has(protocol) && (values.security_layer === 'reality' || values.reality_enabled)
    if (w === 'tls_fields') return TLS.has(protocol) && values.security_layer === 'tls'
    if (w === 'ws') return TRANSPORT.has(protocol) && values.transport_layer === 'ws'
    if (w === 'grpc') return TRANSPORT.has(protocol) && values.transport_layer === 'grpc'
    if (w === 'xhttp_transport') return TRANSPORT.has(protocol) && values.transport_layer === 'xhttp'
    if (w === 'wrapper') return WRAPPER.has(protocol)
    if (w === 'mux') return MUX.has(protocol)
    if (w === 'simple_obfs') return SIMPLE_OBFS.has(protocol)
    if (w === 'kcp_tun') return KCP_TUN.has(protocol)
    if (w === 'xhttp') return XHTTP.has(protocol)
    if (w === 'mkcp') return MKCP.has(protocol)
    if (w === 'mekya') return MEKYA.has(protocol)
    if (w.startsWith('flag:')) {
      const key = w.slice(5)
      return !!values[key]
    }
    return protocol === w
  })
}

export const LISTENER_FORM_SECTIONS: FormSection[] = [
  {
    id: 'transport',
    titleKey: 'listeners.sectionTransport',
    when: 'transport',
    fields: [
      { name: 'transport_layer', type: 'radio', labelKey: 'listeners.transportLayer', options: ['raw', 'ws', 'grpc', 'xhttp'], optionLabels: ['TCP', 'WebSocket', 'gRPC', 'XHTTP'], default: 'raw' },
    ],
  },
  {
    id: 'security',
    titleKey: 'listeners.sectionSecurity',
    when: 'tls_or_reality',
    fields: [
      { name: 'security_layer', type: 'radio', labelKey: 'listeners.securityLayer', options: ['none', 'tls', 'reality'], default: 'none' },
    ],
  },
  {
    id: 'shadowsocks',
    titleKey: 'listeners.sectionProtocol',
    when: 'shadowsocks',
    fields: [
      { name: 'cipher', type: 'select', labelKey: 'listeners.cipher', options: SS_CIPHERS as unknown as string[], required: true },
      { name: 'password', type: 'secret', labelKey: 'listeners.password' },
    ],
  },
  {
    id: 'snell',
    titleKey: 'listeners.sectionProtocol',
    when: 'snell',
    fields: [
      { name: 'psk', type: 'secret', labelKey: 'listeners.psk', required: true },
      { name: 'version', type: 'select', labelKey: 'listeners.snellVersion', options: ['1', '2', '3'] },
      { name: 'obfs_opts_mode', type: 'select', labelKey: 'listeners.obfsOptsMode', options: ['http', 'tls'] },
      { name: 'obfs_opts_host', type: 'string', labelKey: 'listeners.obfsOptsHost' },
    ],
  },
  {
    id: 'vmess',
    titleKey: 'listeners.sectionProtocol',
    when: 'vmess',
    fields: [
      { name: 'alterId', type: 'integer', labelKey: 'listeners.alterId', default: 0 },
    ],
  },
  {
    id: 'vless',
    titleKey: 'listeners.sectionProtocol',
    when: 'vless',
    fields: [
      { name: 'flow', type: 'select', labelKey: 'listeners.flow', options: ['', 'xtls-rprx-vision'], optionLabels: ['(none)', 'xtls-rprx-vision'] },
      { name: 'decryption', type: 'text', labelKey: 'listeners.decryption', hintKey: 'listeners.decryptionHint' },
      { name: 'encryption', type: 'text', labelKey: 'listeners.encryption', hintKey: 'listeners.encryptionHint' },
    ],
  },
  {
    id: 'trojan',
    titleKey: 'listeners.sectionProtocol',
    when: 'trojan',
    fields: [
      { name: 'ss_option_method', type: 'select', labelKey: 'listeners.ssOptionMethod', options: SS_CIPHERS as unknown as string[] },
      { name: 'ss_option_password', type: 'secret', labelKey: 'listeners.ssOptionPassword' },
    ],
  },
  {
    id: 'hysteria2',
    titleKey: 'listeners.sectionProtocol',
    when: 'hysteria2',
    fields: [
      { name: 'up', type: 'string', labelKey: 'listeners.up' },
      { name: 'down', type: 'string', labelKey: 'listeners.down' },
      { name: 'ignore-client-bandwidth', type: 'boolean', labelKey: 'listeners.ignoreClientBandwidth' },
      { name: 'obfs', type: 'select', labelKey: 'listeners.obfs', options: ['', 'salamander'] },
      { name: 'obfs-password', type: 'secret', labelKey: 'listeners.obfsPassword' },
      { name: 'obfs-min-packet-size', type: 'integer', labelKey: 'listeners.obfsMinPacketSize' },
      { name: 'obfs-max-packet-size', type: 'integer', labelKey: 'listeners.obfsMaxPacketSize' },
      { name: 'masquerade', type: 'string', labelKey: 'listeners.masquerade' },
      { name: 'alpn', type: 'tags', labelKey: 'listeners.alpn' },
      { name: 'max-idle-time', type: 'integer', labelKey: 'listeners.maxIdleTime' },
      { name: 'handshake-timeout', type: 'integer', labelKey: 'listeners.handshakeTimeout' },
      { name: 'bbr-profile', type: 'string', labelKey: 'listeners.bbrProfile' },
    ],
  },
  {
    id: 'tuic',
    titleKey: 'listeners.sectionProtocol',
    when: 'tuic',
    fields: [
      { name: 'token', type: 'string', labelKey: 'listeners.token' },
      { name: 'congestion-controller', type: 'select', labelKey: 'listeners.congestionController', options: ['cubic', 'bbr', 'new_reno'] },
      { name: 'alpn', type: 'tags', labelKey: 'listeners.alpn' },
      { name: 'max-idle-time', type: 'integer', labelKey: 'listeners.maxIdleTime' },
      { name: 'authentication-timeout', type: 'integer', labelKey: 'listeners.authenticationTimeout' },
      { name: 'max-udp-relay-packet-size', type: 'integer', labelKey: 'listeners.maxUdpRelayPacketSize' },
      { name: 'bbr-profile', type: 'string', labelKey: 'listeners.bbrProfile' },
      { name: 'cwnd', type: 'integer', labelKey: 'listeners.cwnd' },
      { name: 'zero-rtt', type: 'boolean', labelKey: 'listeners.zeroRtt' },
    ],
  },
  {
    id: 'shadowquic',
    titleKey: 'listeners.sectionProtocol',
    when: 'shadowquic',
    fields: [
      { name: 'password', type: 'secret', labelKey: 'listeners.password', required: true },
      { name: 'congestion-controller', type: 'select', labelKey: 'listeners.congestionController', options: ['bbr', 'cubic'] },
      { name: 'zero-rtt', type: 'boolean', labelKey: 'listeners.zeroRtt' },
    ],
  },
  {
    id: 'anytls',
    titleKey: 'listeners.sectionProtocol',
    when: 'anytls',
    fields: [
      { name: 'padding-scheme', type: 'string', labelKey: 'listeners.paddingScheme' },
    ],
  },
  {
    id: 'mieru',
    titleKey: 'listeners.sectionProtocol',
    when: 'mieru',
    fields: [
      { name: 'transport', type: 'select', labelKey: 'listeners.mieruTransport', options: ['UDP', 'TCP'] },
    ],
  },
  {
    id: 'sudoku',
    titleKey: 'listeners.sectionProtocol',
    when: 'sudoku',
    fields: [
      { name: 'table-type', type: 'select', labelKey: 'listeners.tableType', options: ['builtin', 'file'] },
      { name: 'key', type: 'secret', labelKey: 'listeners.key' },
    ],
  },
  {
    id: 'trusttunnel',
    titleKey: 'listeners.sectionProtocol',
    when: 'trusttunnel',
    fields: [
      { name: 'private-key', type: 'secret', labelKey: 'listeners.privateKey' },
    ],
  },
  {
    id: 'ws',
    titleKey: 'listeners.sectionTransport',
    when: 'ws',
    fields: [
      { name: 'ws-path', type: 'string', labelKey: 'listeners.wsPath', default: '/' },
    ],
  },
  {
    id: 'grpc',
    titleKey: 'listeners.sectionTransport',
    when: 'grpc',
    fields: [
      { name: 'grpc-service-name', type: 'string', labelKey: 'listeners.grpcServiceName' },
    ],
  },
  {
    id: 'xhttp_as_transport',
    titleKey: 'listeners.sectionTransport',
    when: 'xhttp_transport',
    fields: [
      { name: 'xhttp_path', type: 'string', labelKey: 'listeners.xhttpPath' },
      { name: 'xhttp_host', type: 'string', labelKey: 'listeners.xhttpHost' },
      { name: 'xhttp_mode', type: 'select', labelKey: 'listeners.xhttpMode', options: ['auto', 'packet-up', 'stream-up', 'stream-one'] },
    ],
  },
  {
    id: 'tls',
    titleKey: 'listeners.sectionTLS',
    when: 'tls_fields',
    fields: [
      { name: 'certificate', type: 'text', labelKey: 'listeners.certificate' },
      { name: 'private-key', type: 'text', labelKey: 'listeners.privateKey' },
      { name: 'alpn', type: 'tags', labelKey: 'listeners.alpn' },
      { name: 'allow-insecure', type: 'boolean', labelKey: 'listeners.allowInsecure', hintKey: 'listeners.allowInsecureHint' },
      { name: 'ech-key', type: 'text', labelKey: 'listeners.echKey' },
      { name: 'client-auth-type', type: 'select', labelKey: 'listeners.clientAuthType', options: ['', 'request', 'require', 'verify', 'require-and-verify'] },
      { name: 'client-auth-cert', type: 'text', labelKey: 'listeners.clientAuthCert' },
    ],
  },
  {
    id: 'reality',
    titleKey: 'listeners.sectionReality',
    when: 'reality_fields',
    fields: [
      { name: 'reality_dest', type: 'string', labelKey: 'listeners.realityDest', required: true },
      { name: 'reality_private_key', type: 'secret', labelKey: 'listeners.realityPrivateKey', required: true },
      { name: 'reality_short_id', type: 'string', labelKey: 'listeners.realityShortId' },
      { name: 'reality_server_names', type: 'tags', labelKey: 'listeners.realityServerNames' },
    ],
  },
  {
    id: 'simple_obfs_toggle',
    titleKey: 'listeners.sectionSimpleObfs',
    when: 'simple_obfs',
    fields: [
      { name: 'simple_obfs_enabled', type: 'boolean', labelKey: 'listeners.simpleObfs' },
    ],
  },
  {
    id: 'simple_obfs',
    titleKey: 'listeners.sectionSimpleObfs',
    when: ['simple_obfs', 'flag:simple_obfs_enabled'],
    fields: [
      { name: 'simple_obfs_mode', type: 'select', labelKey: 'listeners.obfsOptsMode', options: ['http', 'tls'] },
    ],
  },
  {
    id: 'shadow_tls_toggle',
    titleKey: 'listeners.sectionShadowTLS',
    when: 'wrapper',
    fields: [
      { name: 'shadow_tls_enabled', type: 'boolean', labelKey: 'listeners.shadowTLS' },
    ],
  },
  {
    id: 'shadow_tls',
    titleKey: 'listeners.sectionShadowTLS',
    when: ['wrapper', 'flag:shadow_tls_enabled'],
    fields: [
      { name: 'shadow_tls_version', type: 'select', labelKey: 'listeners.shadowTLSVersion', options: ['2', '3'] },
      { name: 'shadow_tls_password', type: 'secret', labelKey: 'listeners.password' },
      { name: 'shadow_tls_handshake_dest', type: 'string', labelKey: 'listeners.handshakeDest' },
      { name: 'shadow_tls_handshake_proxy', type: 'string', labelKey: 'listeners.handshakeProxy' },
      { name: 'shadow_tls_users', type: 'text', labelKey: 'listeners.shadowTLSUsers' },
    ],
  },
  {
    id: 'res_tls_toggle',
    titleKey: 'listeners.sectionResTLS',
    when: 'wrapper',
    fields: [
      { name: 'res_tls_enabled', type: 'boolean', labelKey: 'listeners.resTLS' },
    ],
  },
  {
    id: 'res_tls',
    titleKey: 'listeners.sectionResTLS',
    when: ['wrapper', 'flag:res_tls_enabled'],
    fields: [
      { name: 'res_tls_password', type: 'secret', labelKey: 'listeners.password' },
      { name: 'res_tls_dest', type: 'string', labelKey: 'listeners.resTLSDest' },
      { name: 'res_tls_proxy', type: 'string', labelKey: 'listeners.jlsProxy' },
      { name: 'res_tls_rate_limit', type: 'integer', labelKey: 'listeners.rateLimit' },
      { name: 'res_tls_min_record_len', type: 'integer', labelKey: 'listeners.minRecordLen' },
      { name: 'res_tls_restls_script', type: 'text', labelKey: 'listeners.restlsScript' },
    ],
  },
  {
    id: 'jls_toggle',
    titleKey: 'listeners.sectionJLS',
    when: 'wrapper',
    fields: [
      { name: 'jls_enabled', type: 'boolean', labelKey: 'listeners.jls' },
    ],
  },
  {
    id: 'jls',
    titleKey: 'listeners.sectionJLS',
    when: ['wrapper', 'flag:jls_enabled'],
    fields: [
      { name: 'jls_dest', type: 'string', labelKey: 'listeners.jlsDest' },
      { name: 'jls_sni', type: 'string', labelKey: 'listeners.jlsSni' },
      { name: 'jls_proxy', type: 'string', labelKey: 'listeners.jlsProxy' },
      { name: 'jls_rate_limit', type: 'integer', labelKey: 'listeners.rateLimit' },
      { name: 'jls_alpn', type: 'tags', labelKey: 'listeners.alpn' },
      { name: 'jls_users', type: 'text', labelKey: 'listeners.jlsUsers' },
    ],
  },
  {
    id: 'mux_toggle',
    titleKey: 'listeners.sectionMux',
    when: 'mux',
    fields: [
      { name: 'mux_enabled', type: 'boolean', labelKey: 'listeners.mux' },
    ],
  },
  {
    id: 'mux',
    titleKey: 'listeners.sectionMux',
    when: ['mux', 'flag:mux_enabled'],
    fields: [
      { name: 'mux_protocol', type: 'select', labelKey: 'listeners.muxProtocol', options: ['smux', 'yamux', 'h2mux'] },
      { name: 'mux_max_connections', type: 'integer', labelKey: 'listeners.muxMaxConnections' },
      { name: 'mux_min_streams', type: 'integer', labelKey: 'listeners.muxMinStreams' },
      { name: 'mux_max_streams', type: 'integer', labelKey: 'listeners.muxMaxStreams' },
      { name: 'mux_padding', type: 'boolean', labelKey: 'listeners.muxPadding' },
      { name: 'mux_statistic', type: 'boolean', labelKey: 'listeners.muxStatistic' },
      { name: 'mux_only_tcp', type: 'boolean', labelKey: 'listeners.muxOnlyTcp' },
      { name: 'mux_brutal_enabled', type: 'boolean', labelKey: 'listeners.muxBrutal' },
      { name: 'mux_brutal_up', type: 'string', labelKey: 'listeners.muxBrutalUp' },
      { name: 'mux_brutal_down', type: 'string', labelKey: 'listeners.muxBrutalDown' },
    ],
  },
  {
    id: 'kcp_tun_toggle',
    titleKey: 'listeners.sectionKcpTun',
    when: 'kcp_tun',
    fields: [
      { name: 'kcp_tun_enabled', type: 'boolean', labelKey: 'listeners.kcpTun' },
    ],
  },
  {
    id: 'kcp_tun',
    titleKey: 'listeners.sectionKcpTun',
    when: ['kcp_tun', 'flag:kcp_tun_enabled'],
    fields: [
      { name: 'kcp_tun_key', type: 'secret', labelKey: 'listeners.kcpTunKey' },
      { name: 'kcp_tun_crypt', type: 'select', labelKey: 'listeners.kcpTunCrypt', options: ['aes', 'aes-128', 'aes-192', 'salsa20', 'blowfish', 'twofish', 'cast5', '3des', 'tea', 'xtea', 'xor', 'sm4', 'none'] },
      { name: 'kcp_tun_mode', type: 'select', labelKey: 'listeners.kcpTunMode', options: ['normal', 'fast', 'fast2', 'fast3'] },
      { name: 'kcp_tun_mtu', type: 'integer', label: 'MTU' },
      { name: 'kcp_tun_sndwnd', type: 'integer', labelKey: 'listeners.kcpTunSndwnd' },
      { name: 'kcp_tun_rcvwnd', type: 'integer', labelKey: 'listeners.kcpTunRcvwnd' },
      { name: 'kcp_tun_conn', type: 'integer', labelKey: 'listeners.kcpTunConn' },
      { name: 'kcp_tun_nocomp', type: 'boolean', labelKey: 'listeners.kcpTunNocomp' },
    ],
  },
  {
    id: 'xhttp_toggle',
    titleKey: 'listeners.sectionXHTTP',
    when: 'xhttp',
    fields: [
      { name: 'xhttp_enabled', type: 'boolean', labelKey: 'listeners.xhttp' },
    ],
  },
  {
    id: 'xhttp',
    titleKey: 'listeners.sectionXHTTP',
    when: ['xhttp', 'flag:xhttp_enabled'],
    fields: [
      { name: 'xhttp_path', type: 'string', labelKey: 'listeners.xhttpPath' },
      { name: 'xhttp_host', type: 'string', labelKey: 'listeners.xhttpHost' },
      { name: 'xhttp_mode', type: 'select', labelKey: 'listeners.xhttpMode', options: ['auto', 'packet-up', 'stream-up', 'stream-one'] },
    ],
  },
  {
    id: 'mkcp_toggle',
    titleKey: 'listeners.sectionMKCP',
    when: 'mkcp',
    fields: [
      { name: 'mkcp_enabled', type: 'boolean', labelKey: 'listeners.mkcp' },
    ],
  },
  {
    id: 'mkcp',
    titleKey: 'listeners.sectionMKCP',
    when: ['mkcp', 'flag:mkcp_enabled'],
    fields: [
      { name: 'mkcp_mtu', type: 'integer', label: 'MTU' },
      { name: 'mkcp_tti', type: 'integer', label: 'TTI' },
      { name: 'mkcp_uplink', type: 'integer', labelKey: 'listeners.mkcpUplink' },
      { name: 'mkcp_downlink', type: 'integer', labelKey: 'listeners.mkcpDownlink' },
      { name: 'mkcp_congestion', type: 'boolean', labelKey: 'listeners.mkcpCongestion' },
      { name: 'mkcp_write_buffer', type: 'integer', labelKey: 'listeners.mkcpWriteBuffer' },
      { name: 'mkcp_read_buffer', type: 'integer', labelKey: 'listeners.mkcpReadBuffer' },
      { name: 'mkcp_seed', type: 'string', labelKey: 'listeners.mkcpSeed' },
      { name: 'mkcp_header', type: 'select', labelKey: 'listeners.mkcpHeader', options: ['none', 'srtp', 'utp', 'wechat-video', 'dtls', 'wireguard'] },
    ],
  },
  {
    id: 'mekya_toggle',
    titleKey: 'listeners.sectionMekya',
    when: 'mekya',
    fields: [
      { name: 'mekya_enabled', type: 'boolean', labelKey: 'listeners.mekya' },
    ],
  },
  {
    id: 'mekya',
    titleKey: 'listeners.sectionMekya',
    when: ['mekya', 'flag:mekya_enabled'],
    fields: [
      { name: 'mekya_max_write_size', type: 'integer', labelKey: 'listeners.mekyaMaxWriteSize' },
      { name: 'mekya_max_write_duration_ms', type: 'integer', labelKey: 'listeners.mekyaMaxWriteDuration' },
      { name: 'mekya_max_simultaneous_write_connection', type: 'integer', labelKey: 'listeners.mekyaMaxSimultaneous' },
      { name: 'mekya_packet_writing_buffer', type: 'integer', labelKey: 'listeners.mekyaPacketBuffer' },
      { name: 'mekya_kcp_mtu', type: 'integer', label: 'KCP MTU' },
      { name: 'mekya_kcp_tti', type: 'integer', label: 'KCP TTI' },
      { name: 'mekya_kcp_uplink', type: 'integer', labelKey: 'listeners.mkcpUplink' },
      { name: 'mekya_kcp_downlink', type: 'integer', labelKey: 'listeners.mkcpDownlink' },
      { name: 'mekya_kcp_congestion', type: 'boolean', labelKey: 'listeners.mkcpCongestion' },
      { name: 'mekya_kcp_seed', type: 'string', labelKey: 'listeners.mkcpSeed' },
      { name: 'mekya_kcp_header', type: 'select', labelKey: 'listeners.mkcpHeader', options: ['none', 'srtp', 'utp', 'wechat-video', 'dtls', 'wireguard'] },
    ],
  },
  {
    id: 'jls_upstream_toggle',
    titleKey: 'listeners.sectionJLSUpstream',
    when: 'shadowquic',
    fields: [
      { name: 'jls_upstream_enabled', type: 'boolean', labelKey: 'listeners.jlsUpstream' },
    ],
  },
  {
    id: 'jls_upstream',
    titleKey: 'listeners.sectionJLSUpstream',
    when: ['shadowquic', 'flag:jls_upstream_enabled'],
    fields: [
      { name: 'jls_upstream_addr', type: 'string', labelKey: 'listeners.jlsUpstreamAddr' },
      { name: 'jls_upstream_sni', type: 'string', labelKey: 'listeners.jlsSni' },
      { name: 'jls_upstream_proxy', type: 'string', labelKey: 'listeners.jlsProxy' },
      { name: 'jls_upstream_rate_limit', type: 'integer', labelKey: 'listeners.rateLimit' },
    ],
  },
  {
    id: 'realm_toggle',
    titleKey: 'listeners.sectionRealm',
    when: 'hysteria2',
    fields: [
      { name: 'realm_enabled', type: 'boolean', labelKey: 'listeners.realm' },
    ],
  },
  {
    id: 'realm',
    titleKey: 'listeners.sectionRealm',
    when: ['hysteria2', 'flag:realm_enabled'],
    fields: [
      { name: 'realm_server_url', type: 'string', labelKey: 'listeners.realmServerUrl' },
      { name: 'realm_token', type: 'secret', labelKey: 'listeners.realmToken' },
      { name: 'realm_id', type: 'string', labelKey: 'listeners.realmId' },
      { name: 'realm_stun', type: 'tags', labelKey: 'listeners.realmStun' },
      { name: 'realm_proxy', type: 'string', labelKey: 'listeners.realmProxy' },
      { name: 'realm_skip_cert', type: 'boolean', labelKey: 'listeners.realmSkipCert' },
    ],
  },
]

export function visibleSections(protocol: string, values: Record<string, any>): FormSection[] {
  return LISTENER_FORM_SECTIONS.filter((s) => {
    const when = Array.isArray(s.when) ? s.when : [s.when]
    // For multi-condition with flag, require ALL (protocol/set AND flag)
    if (when.length > 1 && when.some((w) => w.startsWith('flag:'))) {
      return when.every((w) => sectionVisible({ ...s, when: w }, protocol, values))
    }
    return sectionVisible(s, protocol, values)
  })
}

export { TRANSPORT as TRANSPORT_PROTOCOLS, TLS as TLS_PROTOCOLS, REALITY as REALITY_PROTOCOLS }
