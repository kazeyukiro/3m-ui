import { useSearchParams } from 'react-router-dom';
import React, { useCallback, useEffect, useMemo, useState } from 'react';
import {
  Card,
  Table,
  Button,
  Space,
  Modal,
  Form,
  Input,
  Select,
  InputNumber,
  message,
  Popconfirm,
  Switch,
  Typography,
  Dropdown,
  Tooltip,
  Divider,
  Tabs,
  Alert,
} from 'antd';
import {
  IconAddRule,
  IconAddGroup,
  IconMoveUp,
  IconMoveDown,
  IconSaveRules,
  IconApplyRules,
  IconDelete,
} from '../icons';
import {
  fetchGroups,
  saveGroups,
  fetchRules,
  saveRules,
  fetchServerRouting,
  saveServerRouting,
  updateRuleProvider,
  parseRulesImport,
  type GroupEntry,
  type RuleProvider,
  type RuleImportResult,
} from '../api/routing';
import { generateConfig, applyConfigYAML, fetchProxies } from '../api/config';
import { fetchOutbounds } from '../api/outbounds';
import PageHeader from '../components/PageHeader';
import { useI18n } from '../i18n';
import useIsMobile from '../hooks/useIsMobile';
import {
  RULE_TYPES,
  PARAM_RULE_TYPES,
  type RuleRow,
  emptyRule,
  parseRulesText,
  expandWarpDomainLines,
  mergeWarpDomainRules,
  isManagedWarpDomainRule,
  serializeRules,
  validateRules,
  applyTemplate,
  newRuleKey,
} from '../utils/routingRules';

const errMsg = (e: any) => e?.response?.data?.error || e?.message || String(e);

const RoutingPage: React.FC = () => {
  const { t } = useI18n();
  const isMobile = useIsMobile();
  const [groups, setGroups] = useState<GroupEntry[]>([]);
  const [rules, setRules] = useState<RuleRow[]>([]);
  const [proxyNames, setProxyNames] = useState<string[]>([]);
  const [serverTargets, setServerTargets] = useState<string[]>([]);
  const [loading, setLoading] = useState(false);
  const [saving, setSaving] = useState(false);
  const [applying, setApplying] = useState(false);
  const [groupOpen, setGroupOpen] = useState(false);
  const [searchParams, setSearchParams] = useSearchParams();
  const scopeFromUrl = searchParams.get('scope');
  const [scope, setScope] = useState<'client' | 'server'>(scopeFromUrl === 'server' ? 'server' : 'client');
  useEffect(() => {
    const s = searchParams.get('scope');
    if (s === 'client' || s === 'server') setScope(s);
  }, [searchParams]);
  const selectScope = (k: string) => {
    const next = k as 'client' | 'server';
    setScope(next);
    if (next === 'client') {
      const p = new URLSearchParams(searchParams);
      p.delete('scope');
      setSearchParams(p, { replace: true });
    } else {
      setSearchParams({ scope: next }, { replace: true });
    }
  };
  const [serverRules, setServerRules] = useState<RuleRow[]>([]);
  const [warpDomains, setWarpDomains] = useState<string>('');
  const [warpGlobal, setWarpGlobal] = useState(false);
  const [ruleProviders, setRuleProviders] = useState<import('../api/routing').RuleProvider[]>([]);
  const [rpOpen, setRpOpen] = useState(false);
  const [rpUpdating, setRpUpdating] = useState<string | null>(null);
  const [rpForm] = Form.useForm();
  const [form] = Form.useForm();
  const [importOpen, setImportOpen] = useState(false);
  const [importText, setImportText] = useState('');
  const [importPreview, setImportPreview] = useState<RuleImportResult | null>(null);
  const [importParsing, setImportParsing] = useState(false);

  const targetOptions = useMemo(() => {
    const set = new Set<string>(['DIRECT', 'REJECT', 'COMPATIBLE']);
    if (scope === 'server') serverTargets.forEach(n => set.add(n));
    else {
      groups.forEach((g) => g.name && set.add(g.name));
      proxyNames.forEach((n) => n && set.add(n));
    }
    return Array.from(set).map((v) => ({ value: v, label: v }));
  }, [scope, groups, proxyNames, serverTargets]);

  const load = useCallback(async () => {
    setLoading(true);
    try {
      const [g, r, px, sr, outbounds] = await Promise.all([
        fetchGroups(),
        fetchRules(),
        fetchProxies().catch(() => []),
        fetchServerRouting().catch(() => ({ rules: ['MATCH,DIRECT'], proxyGroups: [], proxies: [], warpDomains: [], warpGlobal: false, ruleProviders: [] })),
        fetchOutbounds(),
      ]);
      setGroups(Array.isArray(g) ? g : []);
      setServerTargets([...outbounds.filter(e => e.enabled).map(e => e.proxy.name), ...(sr.proxyGroups || []).map(g => g.name)]);
      setRules(parseRulesText((Array.isArray(r) ? r : []).join('\n')));
      setServerRules(parseRulesText((Array.isArray(sr?.rules) ? sr.rules : ['MATCH,DIRECT']).join('\n')));
      setWarpDomains(Array.isArray(sr?.warpDomains) ? sr.warpDomains.join('\n') : '');
      setWarpGlobal(!!sr?.warpGlobal);
      setRuleProviders(Array.isArray(sr?.ruleProviders) ? sr.ruleProviders : []);
      setProxyNames(
        (Array.isArray(px) ? px : [])
          .map((p: any) => String(p?.name || '').trim())
          .filter(Boolean),
      );
    } catch (e: any) {
      message.error(errMsg(e));
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    load();
  }, [load]);

  /** Persist current editor state to the API before generate (otherwise Apply uses stale DB). */
  const persistRoutingEditor = async () => {
    if (scope === 'server') {
      const domains = warpDomains
        .split(/[\n,，]+/)
        .map((s) => s.trim())
        .filter(Boolean);
      const mergedRuleLines = mergeWarpDomainRules(serializeRules(serverRules), warpDomains);
      const saved = await saveServerRouting({
        rules: mergedRuleLines,
        warpDomains: domains,
        warpGlobal,
        ruleProviders,
      });
      setServerRules(parseRulesText((Array.isArray(saved?.rules) ? saved.rules : mergedRuleLines).join('\n')));
      if (Array.isArray(saved?.warpDomains)) {
        setWarpDomains(saved.warpDomains.join('\n'));
      }
      setWarpGlobal(!!saved?.warpGlobal);
      return;
    }
    // Client subscription: rules + groups currently edited
    const savedRules = await saveRules(serializeRules(rules));
    setRules(parseRulesText((Array.isArray(savedRules) ? savedRules : serializeRules(rules)).join('\n')));
    if (groups.length) {
      await saveGroups(groups);
    }
  };

  const offerApply = () => {
    const body =
      scope === 'server'
        ? t('routing.applyPromptBodyServer') ||
          'Saves the current server egress (rules, WARP domains/global, rule-providers), then generates and reloads Mihomo.'
        : t('routing.applyPromptBody') ||
          'Saves client subscription rules/groups, then refreshes the panel core. Update the subscription in your client to see new groups.';
    Modal.confirm({
      title: t('routing.applyPromptTitle') || 'Apply configuration?',
      content: body,
      okText: t('routing.applyNow') || 'Generate & apply',
      cancelText: t('common.cancel') || 'Later',
      centered: true,
      width: isMobile ? '100%' : 440,
      onOk: async () => {
        setApplying(true);
        try {
          await persistRoutingEditor();
          const res = await generateConfig();
          await applyConfigYAML(res?.config || undefined);
          message.success(t('routing.applyDone') || 'Configuration applied');
        } catch (e: any) {
          message.error(errMsg(e));
          throw e;
        } finally {
          setApplying(false);
        }
      },
    });
  };

  const activeRules = scope === 'server' ? serverRules : rules;
  const setActiveRules = scope === 'server' ? setServerRules : setRules;

  /** Ends-with-MATCH detection that also covers raw (logic/regex) rows. */
  const endsWithMatch = (rows: RuleRow[]): boolean => {
    const last = rows[rows.length - 1];
    if (!last) return false;
    if (last.raw) return last.raw.trim().toUpperCase().startsWith('MATCH');
    return (last.type || '').toUpperCase() === 'MATCH';
  };

  const onImportParse = async () => {
    const text = importText.trim();
    if (!text) {
      message.error(t('routing.importEmpty') || 'Nothing to import');
      return;
    }
    setImportParsing(true);
    try {
      const res = await parseRulesImport(importText);
      setImportPreview(res);
    } catch (e: any) {
      message.error(errMsg(e));
    } finally {
      setImportParsing(false);
    }
  };

  const onImportApply = () => {
    const preview = importPreview;
    if (!preview) return;
    const existing = new Set(serializeRules(activeRules));
    const fresh = preview.rules.filter((r) => !existing.has(r));
    const dupExisting = preview.rules.length - fresh.length;
    if (!fresh.length) {
      message.info(t('routing.importAllDup') || 'All imported rules already exist in the editor');
      setImportOpen(false);
      setImportPreview(null);
      setImportText('');
      return;
    }
    const rows = parseRulesText(fresh.join('\n'));
    setActiveRules((prev) => {
      // Rules after a trailing MATCH can never match — insert before it.
      if (prev.length > 1 && endsWithMatch(prev)) {
        return [...prev.slice(0, -1), ...rows, prev[prev.length - 1]];
      }
      return [...prev, ...rows];
    });
    message.success(
      ((t('routing.importDone') || 'Imported {n} rules (unsaved)') as string).replace(
        '{n}',
        String(fresh.length),
      ) +
        (dupExisting > 0
          ? ' · ' +
            ((t('routing.importDupExisting') || '{n} duplicates of existing rules skipped') as string).replace(
              '{n}',
              String(dupExisting),
            )
          : ''),
    );
    setImportOpen(false);
    setImportPreview(null);
    setImportText('');
  };

  const closeImport = () => {
    setImportOpen(false);
    setImportPreview(null);
    setImportText('');
  };

  const onSaveRules = async () => {
    const current = scope === 'server' ? serverRules : rules;
    const issues = validateRules(current);
    const hard = issues.filter((i) => !i.message.includes('recommended'));
    if (hard.length) {
      message.error(
        (t('routing.ruleInvalid') || 'Invalid rules') +
          ': #' +
          (hard[0].index + 1) +
          ' ' +
          hard[0].message,
      );
      return;
    }
    if (issues.length) {
      message.warning(t('routing.ruleWarnMatch') || 'Last rule should be MATCH (recommended)');
    }
    setSaving(true);
    try {
      if (scope === 'server') {
        const domains = warpDomains
          .split(/[\n,，]+/)
          .map((s) => s.trim())
          .filter(Boolean);
        const mergedRuleLines = mergeWarpDomainRules(serializeRules(current), warpDomains);
        const saved = await saveServerRouting({
          rules: mergedRuleLines,
          warpDomains: domains,
          warpGlobal,
          ruleProviders,
        });
        setServerRules(parseRulesText((Array.isArray(saved?.rules) ? saved.rules : serializeRules(current)).join('\n')));
        if (Array.isArray(saved?.warpDomains)) {
          setWarpDomains(saved.warpDomains.join('\n'));
        }
        setWarpGlobal(!!saved?.warpGlobal);
        message.success(t('routing.serverRulesSaved') || 'Server routing saved');
        offerApply();
      } else {
        const saved = await saveRules(serializeRules(current));
        setRules(parseRulesText((Array.isArray(saved) ? saved : serializeRules(current)).join('\n')));
        message.success(t('routing.rulesSaved') || 'Rules saved');
        offerApply();
      }
    } catch (e: any) {
      message.error(errMsg(e));
    } finally {
      setSaving(false);
    }
  };

  const onAddGroup = async (values: any) => {
    const proxies = Array.isArray(values.proxies)
      ? values.proxies.map((s: string) => String(s).trim()).filter(Boolean)
      : String(values.proxies || '')
          .split(/[,，\s]+/)
          .map((s: string) => s.trim())
          .filter(Boolean);
    const next = [
      ...groups,
      {
        name: values.name,
        type: values.type || 'select',
        proxies: proxies.length ? proxies : ['DIRECT'],
        url: values.url,
        interval: values.interval,
      },
    ];
    try {
      setGroups(await saveGroups(next));
      message.success(t('routing.groupAdded') || 'Group added');
      setGroupOpen(false);
      form.resetFields();
      offerApply();
    } catch (e: any) {
      message.error(errMsg(e));
    }
  };

  const onDeleteGroup = async (idx: number) => {
    const next = groups.filter((_, i) => i !== idx);
    try {
      setGroups(await saveGroups(next));
      message.success(t('common.deleted') || 'Deleted');
      offerApply();
    } catch (e: any) {
      message.error(errMsg(e));
    }
  };

  const updateRule = (index: number, patch: Partial<RuleRow>) => {
    const setter = scope === 'server' ? setServerRules : setRules;
    setter((prev) => prev.map((r, i) => (i === index ? { ...r, ...patch, raw: undefined } : r)));
  };

  const moveRule = (index: number, dir: -1 | 1) => {
    const setter = scope === 'server' ? setServerRules : setRules;
    setter((prev) => {
      const j = index + dir;
      if (j < 0 || j >= prev.length) return prev;
      const next = [...prev];
      const tmp = next[index];
      next[index] = next[j];
      next[j] = tmp;
      return next;
    });
  };

  const removeRule = (index: number) => {
    const setter = scope === 'server' ? setServerRules : setRules;
    setter((prev) => (prev.length <= 1 ? prev : prev.filter((_, i) => i !== index)));
  };

  const addRule = () => {
    const setter = scope === 'server' ? setServerRules : setRules;
    setter((prev) => [...prev, emptyRule({ type: 'DOMAIN-SUFFIX', target: 'DIRECT' })]);
  };

  const onTemplate = async (id: string) => {
    if (scope === 'server') {
      message.info(t('routing.noServerTemplates') || 'Server egress has no templates — set WARP domains below or edit rules.');
      return;
    }
    const tpl = applyTemplate(id, {
      groupName: groups.find((g) => g.name === 'PROXY')?.name || groups[0]?.name || 'PROXY',
      existingProxies: proxyNames,
    });
    // Template switch always overwrites rules (never append).
    const nextRules = tpl.rules;
    setRules(nextRules);
    try {
      // Persist rules immediately so a refresh does not revive the previous template.
      const savedRules = await saveRules(serializeRules(nextRules));
      setRules(parseRulesText((Array.isArray(savedRules) ? savedRules : serializeRules(nextRules)).join('\n')));

      // When the template ships groups, replace the whole group list (no merge / no leftovers).
      if (tpl.groups && tpl.groups.length) {
        const next: GroupEntry[] = tpl.groups.map((g) => ({
          name: g.name,
          type: g.type || 'select',
          proxies: g.proxies?.length ? g.proxies : ['DIRECT'],
          ...(g.url ? { url: g.url } : {}),
          ...(g.interval != null ? { interval: g.interval } : {}),
        }));
        const saved = await saveGroups(next);
        setGroups(Array.isArray(saved) ? saved : next);
      }
    } catch (e: any) {
      message.error(errMsg(e));
      return;
    }
    message.success(
      (t('routing.templateApplied') || 'Template applied (replaced previous rules/groups)') +
        (id.startsWith('community-')
          ? ' · ' + (t('routing.tplCommunityHint') || 'Needs Geo files (Settings → update geodata)')
          : ''),
    );
  };

  const templateMenu = {
    items: [
      { key: 'direct_only', label: t('routing.tplDirect') || 'MATCH → DIRECT only' },
      {
        key: 'cn_direct',
        label: t('routing.tplCnDirect') || 'GEOSITE/GEOIP CN → DIRECT, else group',
      },
      { key: 'reject_ads', label: t('routing.tplAds') || 'Sample ad domains → REJECT' },
      {
        key: 'via_group',
        label: t('routing.tplViaGroup') || 'MATCH → first group / PROXY',
      },
      { type: 'divider' as const },
      {
        key: 'community-yixuan',
        label: t('routing.tplYixuan') || 'Community: YiXuanZX/rules (CN + GFW)',
      },
      {
        key: 'community-echs',
        label: t('routing.tplEchs') || 'Community: echs-top/proxy (ads + CN)',
      },
      {
        key: 'community-aisouler',
        label: t('routing.tplAisouler') || 'Community: AIsouler/MyClash lite',
      },
    ],
    onClick: ({ key }: { key: string }) => onTemplate(key),
  };

  const ruleCardExtra = (
    <Space wrap size="small">
      {scope === 'client' ? (
        <Dropdown menu={templateMenu}>
          <Button size="small">{t('routing.templates') || 'Templates'}</Button>
        </Dropdown>
      ) : null}
      <Button size="small" onClick={() => setImportOpen(true)}>
        {t('routing.importOpen') || 'Batch import'}
      </Button>
      <Button size="small" icon={<IconAddRule />} onClick={addRule}>
        {t('routing.addRule') || 'Add rule'}
      </Button>
      <Button type="primary" size="small" icon={<IconSaveRules />} loading={saving} onClick={onSaveRules}>
        {t('common.save') || 'Save'}
      </Button>
      <Button
        size="small"
        icon={<IconApplyRules />}
        loading={applying}
        onClick={() => offerApply()}
      >
        {t('routing.applyNow') || 'Apply'}
      </Button>
    </Space>
  );

  return (
    <div>
      <PageHeader title={t('routing.title')} subtitle={t('routing.subtitle')} />
      <Tabs
        activeKey={scope}
        onChange={selectScope}
        style={{ marginBottom: 8 }}
        items={[
          { key: 'client', label: t('routing.tabClient') || 'Client subscription' },
          { key: 'server', label: t('routing.tabServer') || 'Server egress' },
        ]}
      />
      <Alert
        type="info"
        showIcon
        style={{ marginBottom: isMobile ? 8 : 12 }}
        message={
          scope === 'server'
            ? (t('routing.serverHint') ||
              'Server egress: rules applied to the panel Mihomo process (for panel Mihomo egress). Affects traffic after it hits your listeners. Save then Apply.')
            : (t('routing.pageHint') ||
              'Client subscription: proxy-groups and rules go into Mihomo/Clash subscription YAML only. Save, then refresh the subscription in the client.')
        }
      />

      {scope === 'server' && (
        <Card
          title={t('routing.warpDomains') || 'WARP domains → rules'}
          style={{ marginBottom: 16 }}
          extra={
            <Typography.Text type="secondary" style={{ fontSize: 12 }}>
              {t('routing.warpDomainsHintShort') || 'Requires WARP account in Settings'}
            </Typography.Text>
          }
        >
          <div style={{ marginBottom: 12, display: 'flex', alignItems: 'center', gap: 8, flexWrap: 'wrap' }}>
            <Switch checked={warpGlobal} onChange={setWarpGlobal} />
            <Typography.Text>
              {t('routing.warpGlobal') || 'Send all traffic via WARP'}
            </Typography.Text>
          </div>
          <Typography.Paragraph type="secondary" style={{ fontSize: 12, marginTop: 0 }}>
            {t('routing.warpGlobalHint') ||
              'When on, the final rule is MATCH,WARP (Cloudflare tunnel endpoints stay DIRECT). Domain list below is optional extra matching; leave empty if you only need global.'}
          </Typography.Paragraph>
          <Input.TextArea
            rows={4}
            value={warpDomains}
            onChange={(e) => setWarpDomains(e.target.value)}
            onBlur={() => {
              const merged = mergeWarpDomainRules(serializeRules(serverRules), warpDomains);
              setServerRules(parseRulesText(merged.join('\n')));
            }}
            placeholder={'openai.com\nfull:api.openai.com\nkeyword:openai\ngeosite:openai'}
          />
          <Typography.Paragraph type="secondary" style={{ marginTop: 8, marginBottom: 0, fontSize: 12 }}>
            {t('routing.warpDomainsHint') ||
              'One per line: host (DOMAIN-SUFFIX), domain:/full:/keyword:/geosite: prefixes, or GEOSITE:name. After Apply, sniffer+DNS help match SNI. Register WARP under Settings first.'}
          </Typography.Paragraph>
        </Card>
      )}



      {scope === 'server' && (
        <Card
          title={t('routing.ruleProviders') || 'Rule providers (rule-set)'}
          style={{ marginBottom: 16 }}
          extra={
            <Space wrap size="small">
              <Button
                size="small"
                type="primary"
                onClick={() => {
                  rpForm.resetFields();
                  rpForm.setFieldsValue({ type: 'http', behavior: 'domain', format: 'mrs', interval: 86400, proxy: 'DIRECT' });
                  setRpOpen(true);
                }}
              >
                {t('routing.rpAdd') || 'Add'}
              </Button>
            </Space>
          }
        >
          <Typography.Paragraph type="secondary" style={{ fontSize: 12, marginTop: 0 }}>
            {t('routing.rpHint') ||
              'Official Mihomo rule-providers. After Save + Apply, use Update to hot-reload a set (PUT /providers/rules/{name}) without full restart. Rules: RULE-SET,name,TARGET'}
          </Typography.Paragraph>
          {ruleProviders.length === 0 ? (
            <Typography.Text type="secondary">{t('routing.rpEmpty') || 'No rule providers yet'}</Typography.Text>
          ) : (
            <Space direction="vertical" style={{ width: '100%' }} size="small">
              {ruleProviders.map((rp) => (
                <div
                  key={rp.name}
                  style={{
                    display: 'flex',
                    flexWrap: 'wrap',
                    gap: 8,
                    alignItems: 'center',
                    justifyContent: 'space-between',
                    padding: '8px 0',
                    borderBottom: '1px solid var(--ant-color-border-secondary, #f0f0f0)',
                  }}
                >
                  <div style={{ minWidth: 0, flex: 1 }}>
                    <Typography.Text strong>{rp.name}</Typography.Text>
                    <Typography.Text type="secondary" style={{ display: 'block', fontSize: 12 }}>
                      {rp.type}/{rp.behavior}/{rp.format || 'yaml'}
                      {rp.url ? ` · ${rp.url}` : ''}
                    </Typography.Text>
                  </div>
                  <Space size="small" wrap>
                    <Button
                      size="small"
                      loading={rpUpdating === rp.name}
                      onClick={async () => {
                        setRpUpdating(rp.name);
                        try {
                          await updateRuleProvider(rp.name);
                          message.success(t('routing.rpUpdated') || `Updated ${rp.name}`);
                        } catch (e: any) {
                          message.error(errMsg(e));
                        } finally {
                          setRpUpdating(null);
                        }
                      }}
                    >
                      {t('routing.rpHotUpdate') || 'Hot update'}
                    </Button>
                    <Button
                      size="small"
                      danger
                      onClick={() => {
                        setRuleProviders((prev) => prev.filter((x) => x.name !== rp.name));
                      }}
                    >
                      {t('common.delete') || 'Delete'}
                    </Button>
                  </Space>
                </div>
              ))}
            </Space>
          )}
          <Modal
            title={t('routing.rpAdd') || 'Add rule provider'}
            open={rpOpen}
            onCancel={() => setRpOpen(false)}
            onOk={async () => {
              try {
                const v = await rpForm.validateFields();
                const name = String(v.name || '').trim();
                if (!name) return;
                if (ruleProviders.some((x) => x.name === name)) {
                  message.error(t('routing.rpDup') || 'Name already exists');
                  return;
                }
                const entry: RuleProvider = {
                  name,
                  type: v.type || 'http',
                  behavior: v.behavior || 'domain',
                  format: v.format || 'yaml',
                  url: v.url,
                  path: v.path,
                  interval: v.interval != null ? Number(v.interval) : 86400,
                  proxy: v.proxy || 'DIRECT',
                  payload:
                    typeof v.payload === 'string'
                      ? String(v.payload)
                          .split(/\n/)
                          .map((s: string) => s.trim())
                          .filter(Boolean)
                      : undefined,
                };
                setRuleProviders((prev) => [...prev, entry]);
                setRpOpen(false);
                message.info(t('routing.rpAddedHint') || 'Added — Save then Generate & apply so Mihomo loads rule-providers');
              } catch {
                /* validate */
              }
            }}
            destroyOnHidden
          >
            <Form form={rpForm} layout="vertical" size="small">
              <Form.Item name="name" label="name" rules={[{ required: true }]}>
                <Input placeholder="gfw" />
              </Form.Item>
              <Form.Item name="type" label="type" initialValue="http">
                <Select
                  options={[
                    { value: 'http', label: 'http' },
                    { value: 'file', label: 'file' },
                    { value: 'inline', label: 'inline' },
                  ]}
                />
              </Form.Item>
              <Form.Item name="behavior" label="behavior" initialValue="domain">
                <Select
                  options={[
                    { value: 'domain', label: 'domain' },
                    { value: 'ipcidr', label: 'ipcidr' },
                    { value: 'classical', label: 'classical' },
                  ]}
                />
              </Form.Item>
              <Form.Item name="format" label="format" initialValue="mrs">
                <Select
                  options={[
                    { value: 'mrs', label: 'mrs' },
                    { value: 'yaml', label: 'yaml' },
                    { value: 'text', label: 'text' },
                  ]}
                />
              </Form.Item>
              <Form.Item name="url" label="url" extra="Required for type=http">
                <Input placeholder="https://..." />
              </Form.Item>
              <Form.Item name="path" label="path" extra="Optional; default ./rule-providers/{name}.{format}">
                <Input placeholder="./rule-providers/gfw.mrs" />
              </Form.Item>
              <Form.Item name="interval" label="interval (s)" initialValue={86400}>
                <InputNumber min={60} style={{ width: '100%' }} />
              </Form.Item>
              <Form.Item name="proxy" label="proxy (download)" initialValue="DIRECT">
                <Input placeholder="DIRECT" />
              </Form.Item>
              <Form.Item name="payload" label="payload (inline, one per line)">
                <Input.TextArea rows={3} placeholder="DOMAIN-SUFFIX,example.com" />
              </Form.Item>
            </Form>
          </Modal>
        </Card>
      )}

      {scope === 'client' && (
      <Card
        title={t('routing.groups')}
        extra={
          <Button type="primary" icon={<IconAddGroup />} onClick={() => setGroupOpen(true)}>
            {t('routing.addGroup')}
          </Button>
        }
        style={{ marginBottom: 16 }}
      >
        <Table
          size={isMobile ? 'small' : 'middle'}
          loading={loading}
          pagination={false}
          scroll={isMobile ? { x: 480 } : undefined}
          rowKey={(_, i) => String(i)}
          dataSource={groups}
          columns={[
            { title: t('common.name'), dataIndex: 'name', ellipsis: true },
            { title: t('common.type'), dataIndex: 'type', width: isMobile ? 90 : 120 },
            {
              title: t('routing.proxies'),
              dataIndex: 'proxies',
              ellipsis: true,
              render: (v: string[]) => (v || []).join(', '),
            },
            {
              title: t('common.actions'),
              key: 'a',
              width: 72,
              render: (_: any, __: any, idx: number) => (
                <Popconfirm title={t('common.confirmDelete')} onConfirm={() => onDeleteGroup(idx)}>
                  <Button size="small" danger icon={<IconDelete />} />
                </Popconfirm>
              ),
            },
          ]}
        />
      </Card>
      )}

      <Card title={t('routing.rules')} extra={isMobile ? undefined : ruleCardExtra}>
        {isMobile ? <div style={{ marginBottom: 12 }}>{ruleCardExtra}</div> : null}
        <div style={{ display: 'flex', flexDirection: 'column', gap: 10 }}>
          {activeRules.map((row, index) => (
            <div
              key={row.key || newRuleKey()}
              style={{
                border: '1px solid rgba(0,0,0,0.08)',
                borderRadius: 8,
                padding: isMobile ? 10 : 12,
                background: 'var(--ant-color-bg-container, #fff)',
              }}
            >
              {row.raw ? (
                <Space direction="vertical" style={{ width: '100%' }} size={8}>
                  <Typography.Text type="warning">
                    {t('routing.rawRule') || 'Advanced / unparsed rule (saved as-is)'}
                  </Typography.Text>
                  <Input.TextArea
                    rows={2}
                    value={row.raw}
                    onChange={(e) => updateRule(index, { raw: e.target.value })}
                  />
                </Space>
              ) : (
                <Space direction={isMobile ? 'vertical' : 'horizontal'} wrap style={{ width: '100%' }} size={8}>
                  <Select
                    style={{ width: isMobile ? '100%' : 160 }}
                    value={row.type}
                    options={RULE_TYPES.map((x) => ({ value: x, label: x }))}
                    onChange={(v) => updateRule(index, { type: v })}
                  />
                  {row.type !== 'MATCH' ? (
                    <Input
                      style={{ width: isMobile ? '100%' : 200, flex: 1 }}
                      placeholder={
                        row.type === 'GEOIP'
                          ? 'CN'
                          : row.type?.startsWith('IP-')
                            ? '1.1.1.1/32'
                            : 'example.com'
                      }
                      value={row.payload}
                      onChange={(e) => updateRule(index, { payload: e.target.value })}
                    />
                  ) : null}
                  <Select
                    showSearch
                    style={{ width: isMobile ? '100%' : 160 }}
                    value={row.target}
                    options={targetOptions}
                    onChange={(v) => updateRule(index, { target: v })}
                    placeholder="DIRECT"
                  />
                  {PARAM_RULE_TYPES.includes(row.type as (typeof PARAM_RULE_TYPES)[number]) && (
                    <Space>
                      <Typography.Text type="secondary" style={{ fontSize: 12 }}>
                        no-resolve
                      </Typography.Text>
                      <Switch
                        size="small"
                        checked={row.noResolve}
                        onChange={(v) => updateRule(index, { noResolve: v, param: v ? 'no-resolve' : undefined })}
                      />
                    </Space>
                  )}
                </Space>
              )}
              <div style={{ marginTop: 8, display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
                <Typography.Text type="secondary" style={{ fontSize: 12 }}>
                  #{index + 1}
                  {!row.raw ? ` · ${serializeRules([row])[0] || ''}` : ''}
                </Typography.Text>
                <Space size={4}>
                  <Tooltip title={t('routing.moveUp') || 'Move up'}>
                    <Button size="small" icon={<IconMoveUp />} disabled={index === 0} onClick={() => moveRule(index, -1)} />
                  </Tooltip>
                  <Tooltip title={t('routing.moveDown') || 'Move down'}>
                    <Button
                      size="small"
                      icon={<IconMoveDown />}
                      disabled={index === rules.length - 1}
                      onClick={() => moveRule(index, 1)}
                    />
                  </Tooltip>
                  <Button size="small" danger icon={<IconDelete />} disabled={activeRules.length <= 1} onClick={() => removeRule(index)} />
                </Space>
              </div>
            </div>
          ))}
        </div>
        <div style={{ marginTop: 8, opacity: 0.65, fontSize: 12 }}>{t('routing.rulesHint')}</div>
      </Card>

      <Modal
        open={importOpen}
        title={t('routing.importTitle') || 'Batch import rules'}
        onOk={importPreview ? onImportApply : onImportParse}
        okText={
          importPreview
            ? ((t('routing.importAccept') || 'Append {n} rules') as string).replace(
                '{n}',
                String(importPreview.rules.length),
              )
            : t('routing.importParse') || 'Parse & preview'
        }
        okButtonProps={{ disabled: importPreview ? importPreview.rules.length === 0 : false }}
        confirmLoading={importParsing}
        cancelText={importPreview ? t('routing.importBack') || 'Back' : t('common.cancel')}
        onCancel={() => (importPreview ? setImportPreview(null) : closeImport())}
        width={isMobile ? '100%' : 640}
        destroyOnClose
      >
        {!importPreview ? (
          <>
            <Alert
              type="info"
              showIcon
              style={{ marginBottom: 12 }}
              message={t('routing.importHintTitle') || 'One rule per line — mihomo official syntax'}
              description={<span style={{ whiteSpace: 'pre-line' }}>{t('routing.importHintBody') || ''}</span>}
            />
            <Input.TextArea
              value={importText}
              onChange={(e) => setImportText(e.target.value)}
              rows={12}
              placeholder={t('routing.importPlaceholder') || ''}
            />
            <Typography.Paragraph type="secondary" style={{ marginTop: 8, marginBottom: 0, fontSize: 12 }}>
              {t('routing.importTargetHint') || ''}
            </Typography.Paragraph>
          </>
        ) : (
          <div>
            <Alert
              type={importPreview.issues.length ? 'warning' : 'success'}
              showIcon
              style={{ marginBottom: 12 }}
              message={
                (t('routing.importPreviewTitle') || 'Parsed {n} rules').replace('{n}', String(importPreview.rules.length)) +
                (importPreview.duplicates > 0
                  ? ' · ' +
                    (t('routing.importDupBatch') || '{n} in-batch duplicates dropped').replace(
                      '{n}',
                      String(importPreview.duplicates),
                    )
                  : '') +
                (importPreview.truncated > 0
                  ? ' · ' +
                    (t('routing.importTruncated') || '{n} over the cap were not imported').replace(
                      '{n}',
                      String(importPreview.truncated),
                    )
                  : '')
              }
              description={t('routing.importBeforeMatch') || ''}
            />
            {importPreview.issues.length > 0 && (
              <div style={{ marginBottom: 12 }}>
                <Typography.Text type="danger">
                  {((t('routing.importIssuesTitle') || '{n} lines rejected') as string).replace(
                    '{n}',
                    String(importPreview.issues.length),
                  )}
                </Typography.Text>
                <div
                  style={{
                    maxHeight: 160,
                    overflow: 'auto',
                    border: '1px solid var(--ant-color-border-secondary, #f0f0f0)',
                    borderRadius: 6,
                    padding: 8,
                    marginTop: 4,
                  }}
                >
                  {importPreview.issues.map((iss) => (
                    <div key={`${iss.line}-${iss.reason}`} style={{ fontSize: 12, marginBottom: 4 }}>
                      <Typography.Text type="danger" style={{ fontSize: 12 }}>
                        #{iss.line}
                      </Typography.Text>{' '}
                      <Typography.Text code style={{ fontSize: 12 }}>
                        {iss.text}
                      </Typography.Text>{' '}
                      — {iss.reason}
                    </div>
                  ))}
                </div>
              </div>
            )}
            <Typography.Text strong>{t('routing.importPreviewRules') || 'Normalized rules'}</Typography.Text>
            <pre
              style={{
                margin: '4px 0 0',
                fontSize: 12,
                whiteSpace: 'pre-wrap',
                wordBreak: 'break-all',
                maxHeight: 240,
                overflow: 'auto',
                border: '1px solid var(--ant-color-border-secondary, #f0f0f0)',
                borderRadius: 6,
                padding: 8,
              }}
            >
              {importPreview.rules.join('\n')}
            </pre>
          </div>
        )}
      </Modal>

      <Modal
        open={groupOpen}
        title={t('routing.addGroup')}
        onCancel={() => setGroupOpen(false)}
        onOk={() => form.submit()}
        destroyOnClose
        width={isMobile ? '100%' : 520}
        style={isMobile ? { top: 8 } : undefined}
      >
        <Form form={form} layout="vertical" onFinish={onAddGroup} initialValues={{ type: 'select' }}>
          <Form.Item name="name" label={t('common.name')} rules={[{ required: true }]}>
            <Input />
          </Form.Item>
          <Form.Item name="type" label={t('common.type')}>
            <Select
              options={[
                { value: 'select', label: 'select' },
                { value: 'url-test', label: 'url-test' },
                { value: 'fallback', label: 'fallback' },
                { value: 'load-balance', label: 'load-balance' },
              ]}
            />
          </Form.Item>
          <Form.Item name="proxies" label={t('routing.proxies')} tooltip={t('routing.proxiesHint')}>
            <Select
              mode="tags"
              tokenSeparators={[',', ' ']}
              placeholder="DIRECT, PROXY, …"
              options={targetOptions}
            />
          </Form.Item>
          <Form.Item name="url" label="URL (url-test)">
            <Input placeholder="http://www.gstatic.com/generate_204" />
          </Form.Item>
          <Form.Item name="interval" label="Interval (s)">
            <InputNumber min={0} style={{ width: '100%' }} />
          </Form.Item>
        </Form>
      </Modal>
    </div>
  );
};

export default RoutingPage;
