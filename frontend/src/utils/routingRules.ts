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

/** Community rule templates (adapted for 3m-ui server visual-config).
 * Inspired by public Mihomo rule projects — see README acknowledgements.
 * Prefer GEOSITE/GEOIP so MetaCubeX geodata works without extra rule-providers.
 */
export function applyTemplate(
  id: string,
  opts: { groupName?: string },
): RuleRow[] {
  const g = (opts.groupName || 'PROXY').trim() || 'PROXY';
  switch (id) {
    case 'direct_only':
      return [emptyRule({ type: 'MATCH', target: 'DIRECT' })];
    case 'cn_direct':
      return [
        emptyRule({ type: 'GEOIP', payload: 'CN', target: 'DIRECT', noResolve: true }),
        emptyRule({ type: 'MATCH', target: g }),
      ];
    case 'reject_ads':
      return [
        emptyRule({ type: 'DOMAIN-SUFFIX', payload: 'doubleclick.net', target: 'REJECT' }),
        emptyRule({ type: 'DOMAIN-SUFFIX', payload: 'googlesyndication.com', target: 'REJECT' }),
        emptyRule({ type: 'DOMAIN-KEYWORD', payload: 'adservice', target: 'REJECT' }),
        emptyRule({ type: 'MATCH', target: 'DIRECT' }),
      ];
    case 'via_group':
      return [emptyRule({ type: 'MATCH', target: g })];
    // —— YiXuanZX/rules inspired: CN direct + GFW/Telegram proxy ——
    case 'community-yixuan':
      return [
        emptyRule({ type: 'GEOSITE', payload: 'private', target: 'DIRECT' }),
        emptyRule({ type: 'GEOSITE', payload: 'cn', target: 'DIRECT' }),
        emptyRule({ type: 'GEOIP', payload: 'CN', target: 'DIRECT', noResolve: true }),
        emptyRule({ type: 'GEOSITE', payload: 'telegram', target: g }),
        emptyRule({ type: 'GEOSITE', payload: 'gfw', target: g }),
        emptyRule({ type: 'MATCH', target: g }),
      ];
    // —— echs-top/proxy inspired: light ad reject + CN direct + proxy ——
    case 'community-echs':
      return [
        emptyRule({ type: 'DOMAIN-SUFFIX', payload: 'doubleclick.net', target: 'REJECT' }),
        emptyRule({ type: 'DOMAIN-SUFFIX', payload: 'googleadservices.com', target: 'REJECT' }),
        emptyRule({ type: 'DOMAIN-KEYWORD', payload: 'adservice', target: 'REJECT' }),
        emptyRule({ type: 'GEOSITE', payload: 'private', target: 'DIRECT' }),
        emptyRule({ type: 'GEOSITE', payload: 'cn', target: 'DIRECT' }),
        emptyRule({ type: 'GEOIP', payload: 'CN', target: 'DIRECT', noResolve: true }),
        emptyRule({ type: 'GEOSITE', payload: 'gfw', target: g }),
        emptyRule({ type: 'MATCH', target: g }),
      ];
    // —— AIsouler/MyClash lite inspired: CN + Google/Telegram/AI/GFW ——
    case 'community-aisouler':
      return [
        emptyRule({ type: 'GEOSITE', payload: 'private', target: 'DIRECT' }),
        emptyRule({ type: 'GEOSITE', payload: 'cn', target: 'DIRECT' }),
        emptyRule({ type: 'GEOIP', payload: 'CN', target: 'DIRECT', noResolve: true }),
        emptyRule({ type: 'GEOSITE', payload: 'google', target: g }),
        emptyRule({ type: 'GEOSITE', payload: 'telegram', target: g }),
        emptyRule({ type: 'GEOSITE', payload: 'openai', target: g }),
        emptyRule({ type: 'GEOSITE', payload: 'gfw', target: g }),
        emptyRule({ type: 'MATCH', target: g }),
      ];
    default:
      return [emptyRule({ type: 'MATCH', target: 'DIRECT' })];
  }
}
