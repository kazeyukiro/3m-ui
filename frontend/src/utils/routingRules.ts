/** Structured Mihomo routing rules helpers. */

export type RuleRow = {
  key: string;
  type: string;
  payload: string;
  target: string;
  noResolve: boolean;
  /** If set, serialize as-is (unparsed advanced rule). */
  raw?: string;
};

export const RULE_TYPES = [
  'DOMAIN',
  'DOMAIN-SUFFIX',
  'DOMAIN-KEYWORD',
  'GEOIP',
  'GEOSITE',
  'IP-CIDR',
  'IP-CIDR6',
  'SRC-IP-CIDR',
  'DST-PORT',
  'SRC-PORT',
  'PROCESS-NAME',
  'RULE-SET',
  'MATCH',
] as const;

let keySeq = 0;
export function newRuleKey(): string {
  keySeq += 1;
  return `r-${Date.now()}-${keySeq}`;
}

export function emptyRule(partial?: Partial<RuleRow>): RuleRow {
  return {
    key: newRuleKey(),
    type: 'DOMAIN-SUFFIX',
    payload: '',
    target: 'DIRECT',
    noResolve: false,
    ...partial,
  };
}

/** Parse a Mihomo rule line into a structured row. */
export function parseRuleLine(line: string): RuleRow {
  const trimmed = line.trim();
  if (!trimmed) {
    return emptyRule({ type: 'MATCH', target: 'DIRECT' });
  }
  const parts = trimmed.split(',').map((p) => p.trim());
  if (parts.length === 0) {
    return emptyRule({ raw: trimmed });
  }
  const type = (parts[0] || '').toUpperCase();
  if (type === 'MATCH') {
    return emptyRule({
      type: 'MATCH',
      payload: '',
      target: parts[1] || 'DIRECT',
      noResolve: false,
    });
  }
  // TYPE,payload,target[,no-resolve]
  if (parts.length >= 3) {
    const noResolve = parts.slice(3).some((p) => p.toLowerCase() === 'no-resolve');
    return emptyRule({
      type,
      payload: parts[1] || '',
      target: parts[2] || 'DIRECT',
      noResolve,
    });
  }
  // TYPE,target only (unusual) or unparsed
  if (parts.length === 2 && RULE_TYPES.includes(type as (typeof RULE_TYPES)[number])) {
    return emptyRule({ type, payload: '', target: parts[1], noResolve: false });
  }
  return emptyRule({ type: 'DOMAIN-SUFFIX', payload: '', target: 'DIRECT', raw: trimmed });
}

export function serializeRule(row: RuleRow): string {
  if (row.raw && row.raw.trim()) {
    return row.raw.trim();
  }
  const type = (row.type || 'MATCH').toUpperCase();
  const target = (row.target || 'DIRECT').trim() || 'DIRECT';
  if (type === 'MATCH') {
    return `MATCH,${target}`;
  }
  const payload = (row.payload || '').trim();
  let s = `${type},${payload},${target}`;
  if (row.noResolve && (type === 'IP-CIDR' || type === 'IP-CIDR6' || type === 'GEOIP')) {
    s += ',no-resolve';
  }
  return s;
}

export function parseRulesText(text: string): RuleRow[] {
  const lines = text.split(/\r?\n/).map((l) => l.trim()).filter(Boolean);
  if (!lines.length) return [emptyRule({ type: 'MATCH', target: 'DIRECT' })];
  return lines.map(parseRuleLine);
}

export function serializeRules(rows: RuleRow[]): string[] {
  return rows.map(serializeRule).filter((s) => s.length > 0);
}

export type RuleIssue = { index: number; message: string };

export function validateRules(rows: RuleRow[]): RuleIssue[] {
  const issues: RuleIssue[] = [];
  rows.forEach((row, index) => {
    if (row.raw) return;
    const type = (row.type || '').toUpperCase();
    if (!type) {
      issues.push({ index, message: 'missing type' });
      return;
    }
    if (!(row.target || '').trim()) {
      issues.push({ index, message: 'missing target' });
    }
    if (type !== 'MATCH' && !(row.payload || '').trim()) {
      issues.push({ index, message: 'missing payload' });
    }
  });
  if (rows.length) {
    const last = rows[rows.length - 1];
    const lastType = (last.raw ? last.raw.split(',')[0] : last.type || '').toUpperCase();
    if (lastType !== 'MATCH') {
      issues.push({ index: rows.length - 1, message: 'last rule should be MATCH (recommended)' });
    }
  }
  return issues;
}

/** Result of a routing template (rules + recommended proxy-groups). */
export type TemplateResult = {
  rules: RuleRow[];
  /** When non-empty, replace or merge into panel groups (see Routing page). */
  groups?: Array<{
    name: string;
    type: string;
    proxies: string[];
    url?: string;
    interval?: number;
  }>;
  /** If true, merge groups by name instead of wiping existing groups. */
  mergeGroups?: boolean;
};

/**
 * Community rule templates adapted for 3m-ui server visual-config.
 * Inspired by public Mihomo rule projects — include matching proxy-groups, not only rules.
 * Prefer GEOSITE/GEOIP so MetaCubeX geodata works without extra rule-providers.
 */
export function applyTemplate(
  id: string,
  opts: { groupName?: string; existingProxies?: string[] } = {},
): TemplateResult {
  const g = (opts.groupName || 'PROXY').trim() || 'PROXY';
  const leaves = (opts.existingProxies || []).filter(Boolean);
  // Default leaf list for select groups when no outbound proxies exist yet
  const leafOrDirect = leaves.length ? leaves : ['DIRECT'];

  const select = (name: string, proxies: string[]) => ({
    name,
    type: 'select',
    proxies: proxies.length ? proxies : ['DIRECT'],
  });

  switch (id) {
    case 'direct_only':
      return { rules: [emptyRule({ type: 'MATCH', target: 'DIRECT' })] };

    case 'cn_direct':
      return {
        rules: [
          emptyRule({ type: 'GEOIP', payload: 'CN', target: 'DIRECT', noResolve: true }),
          emptyRule({ type: 'MATCH', target: g }),
        ],
        groups: [select(g, leafOrDirect)],
        mergeGroups: true,
      };

    case 'reject_ads':
      return {
        rules: [
          emptyRule({ type: 'DOMAIN-SUFFIX', payload: 'doubleclick.net', target: 'REJECT' }),
          emptyRule({ type: 'DOMAIN-SUFFIX', payload: 'googlesyndication.com', target: 'REJECT' }),
          emptyRule({ type: 'DOMAIN-KEYWORD', payload: 'adservice', target: 'REJECT' }),
          emptyRule({ type: 'MATCH', target: 'DIRECT' }),
        ],
      };

    case 'via_group':
      return {
        rules: [emptyRule({ type: 'MATCH', target: g })],
        groups: [select(g, leafOrDirect)],
        mergeGroups: true,
      };

    // —— YiXuanZX/rules: region selects + 代理 / AI / TG ——
    case 'community-yixuan': {
      const regions = ['香港', '新加坡', '日本', '美国', '其他'];
      const groups = [
        ...regions.map((n) => select(n, leafOrDirect)),
        select('代理', [...regions, 'DIRECT']),
        select('AI', ['美国', '新加坡', '香港', '日本', '其他', 'DIRECT']),
        select('TG', ['香港', '新加坡', '日本', '美国', '其他', 'DIRECT']),
      ];
      return {
        mergeGroups: true,
        groups,
        rules: [
          emptyRule({ type: 'GEOSITE', payload: 'private', target: 'DIRECT' }),
          emptyRule({ type: 'GEOSITE', payload: 'cn', target: 'DIRECT' }),
          emptyRule({ type: 'GEOIP', payload: 'CN', target: 'DIRECT', noResolve: true }),
          emptyRule({ type: 'GEOSITE', payload: 'openai', target: 'AI' }),
          emptyRule({ type: 'GEOSITE', payload: 'telegram', target: 'TG' }),
          emptyRule({ type: 'GEOSITE', payload: 'gfw', target: '代理' }),
          emptyRule({ type: 'MATCH', target: '代理' }),
        ],
      };
    }

    // —— echs-top/proxy: ads + CN + policy groups ——
    case 'community-echs': {
      const groups = [
        select('代理连接', leafOrDirect),
        select('TELEGRAM', leafOrDirect),
        select('国外AI', leafOrDirect),
        select('GOOGLE', leafOrDirect),
        select('海外媒体', leafOrDirect),
        select('下载相关', leafOrDirect),
        select('风控安全', leafOrDirect),
      ];
      return {
        mergeGroups: true,
        groups,
        rules: [
          emptyRule({ type: 'DOMAIN-SUFFIX', payload: 'doubleclick.net', target: 'REJECT' }),
          emptyRule({ type: 'DOMAIN-SUFFIX', payload: 'googleadservices.com', target: 'REJECT' }),
          emptyRule({ type: 'DOMAIN-KEYWORD', payload: 'adservice', target: 'REJECT' }),
          emptyRule({ type: 'GEOSITE', payload: 'private', target: 'DIRECT' }),
          emptyRule({ type: 'GEOSITE', payload: 'cn', target: 'DIRECT' }),
          emptyRule({ type: 'GEOIP', payload: 'CN', target: 'DIRECT', noResolve: true }),
          emptyRule({ type: 'GEOSITE', payload: 'telegram', target: 'TELEGRAM' }),
          emptyRule({ type: 'GEOSITE', payload: 'openai', target: '国外AI' }),
          emptyRule({ type: 'GEOSITE', payload: 'google', target: 'GOOGLE' }),
          emptyRule({ type: 'GEOSITE', payload: 'youtube', target: '海外媒体' }),
          emptyRule({ type: 'GEOSITE', payload: 'netflix', target: '海外媒体' }),
          emptyRule({ type: 'GEOSITE', payload: 'gfw', target: '代理连接' }),
          emptyRule({ type: 'MATCH', target: '代理连接' }),
        ],
      };
    }

    // —— AIsouler/MyClash lite: 直连 / AdBlock / Google / AI / Telegram / Steam / 默认代理 / 漏网之鱼 ——
    case 'community-aisouler': {
      const groups = [
        select('直连', ['DIRECT']),
        select('AdBlock', ['REJECT', 'DIRECT']),
        select('Google', leafOrDirect),
        select('AI', leafOrDirect),
        select('Telegram', leafOrDirect),
        select('Steam', leafOrDirect),
        select('默认代理', leafOrDirect),
        select('漏网之鱼', ['默认代理', 'DIRECT', 'REJECT']),
      ];
      return {
        mergeGroups: true,
        groups,
        rules: [
          emptyRule({ type: 'GEOSITE', payload: 'private', target: '直连' }),
          emptyRule({ type: 'GEOSITE', payload: 'cn', target: '直连' }),
          emptyRule({ type: 'GEOIP', payload: 'CN', target: '直连', noResolve: true }),
          emptyRule({ type: 'DOMAIN-SUFFIX', payload: 'doubleclick.net', target: 'AdBlock' }),
          emptyRule({ type: 'DOMAIN-KEYWORD', payload: 'adservice', target: 'AdBlock' }),
          emptyRule({ type: 'GEOSITE', payload: 'google', target: 'Google' }),
          emptyRule({ type: 'GEOSITE', payload: 'openai', target: 'AI' }),
          emptyRule({ type: 'GEOSITE', payload: 'telegram', target: 'Telegram' }),
          emptyRule({ type: 'GEOSITE', payload: 'steam', target: 'Steam' }),
          emptyRule({ type: 'GEOSITE', payload: 'gfw', target: '默认代理' }),
          emptyRule({ type: 'MATCH', target: '漏网之鱼' }),
        ],
      };
    }

    default:
      return { rules: [emptyRule({ type: 'MATCH', target: 'DIRECT' })] };
  }
}
