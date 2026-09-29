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
  type GroupEntry,
  type RuleProvider,
} from '../api/routing';
import { generateConfig, applyConfigYAML, fetchProxies } from '../api/config';
import PageHeader from '../components/PageHeader';
import { useI18n } from '../i18n';
import useIsMobile from '../hooks/useIsMobile';
import {
  RULE_TYPES,
  type RuleRow,
  emptyRule,
  parseRulesText,
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
  const [loading, setLoading] = useState(false);
  const [saving, setSaving] = useState(false);
  const [applying, setApplying] = useState(false);
  const [groupOpen, setGroupOpen] = useState(false);
  const [scope, setScope] = useState<'client' | 'server'>('client');
  const [serverRules, setServerRules] = useState<RuleRow[]>([]);
  const [warpDomains, setWarpDomains] = useState<string>('');
  const [warpGlobal, setWarpGlobal] = useState(false);
  const [ruleProviders, setRuleProviders] = useState<import('../api/routing').RuleProvider[]>([]);
  const [rpOpen, setRpOpen] = useState(false);
  const [rpUpdating, setRpUpdating] = useState<string | null>(null);
  const [rpForm] = Form.useForm();
  const [form] = Form.useForm();

  const targetOptions = useMemo(() => {
    const set = new Set<string>(['DIRECT', 'REJECT', 'COMPATIBLE']);
    groups.forEach((g) => g.name && set.add(g.name));
    proxyNames.forEach((n) => n && set.add(n));
    return Array.from(set).map((v) => ({ value: v, label: v }));
  }, [groups, proxyNames]);

  const load = useCallback(async () => {
    setLoading(true);
    try {
      const [g, r, px, sr] = await Promise.all([
        fetchGroups(),
        fetchRules(),
        fetchProxies().catch(() => []),
        fetchServerRouting().catch(() => ({ rules: ['MATCH,DIRECT'], proxyGroups: [], proxies: [], warpDomains: [], warpGlobal: false, ruleProviders: [] })),
      ]);
      setGroups(Array.isArray(g) ? g : []);
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

  const offerApply = () => {
    const body =
      scope === 'server'
        ? t('routing.applyPromptBodyServer') ||
          'Generate & apply writes server egress (rules + WARP domains + WARP outbound) into the panel Mihomo config and reloads the core.'
        : t('routing.applyPromptBody') ||
          'Saved for client subscriptions. Generate & apply also refreshes the panel core. Update the subscription in your client to see new groups.';
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
        const saved = await saveServerRouting({
          proxies: [],
          proxyGroups: [],
          rules: serializeRules(current),
          warpDomains: domains,
          warpGlobal,
          ruleProviders,
        });
        setServerRules(parseRulesText((Array.isArray(saved?.rules) ? saved.rules : serializeRules(current)).join('\n')));
        if (Array.isArray(saved?.warpDomains)) {
          setWarpDomains(saved.warpDomains.join('\n'));
        }
        setWarpGlobal(!!saved?.warpGlobal);
        message.success(t('routing.serverRulesSaved') || 'Server routing saved — Apply to load WARP domains into Mihomo');
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
        onChange={(k) => setScope(k as 'client' | 'server')}
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
          title={t('routing.warpDomains') || 'WARP domains'}
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
            disabled={false}
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
                  {(row.type === 'IP-CIDR' || row.type === 'IP-CIDR6' || row.type === 'GEOIP') && (
                    <Space>
                      <Typography.Text type="secondary" style={{ fontSize: 12 }}>
                        no-resolve
                      </Typography.Text>
                      <Switch
                        size="small"
                        checked={row.noResolve}
                        onChange={(v) => updateRule(index, { noResolve: v })}
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
