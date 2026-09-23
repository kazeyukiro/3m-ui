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
} from 'antd';
import {
  ListPlus,
  Trash2,
  ArrowUp,
  ArrowDown,
  FolderPlus,
  CircleCheck,
  Play,
  Layers,
  Cloud,
} from 'lucide-react';
import {
  fetchGroups,
  saveGroups,
  fetchRules,
  saveRules,
  type GroupEntry,
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
      const [g, r, px] = await Promise.all([
        fetchGroups(),
        fetchRules(),
        fetchProxies().catch(() => []),
      ]);
      setGroups(Array.isArray(g) ? g : []);
      setRules(parseRulesText((Array.isArray(r) ? r : []).join('\n')));
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
    Modal.confirm({
      title: t('routing.applyPromptTitle') || 'Apply configuration?',
      content:
        t('routing.applyPromptBody') ||
        'Saved for client subscriptions. Generate & apply only refreshes the panel core (server stays MATCH,DIRECT — no server-side split). Update the subscription in your client to see new groups.',
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

  const onSaveRules = async () => {
    const issues = validateRules(rules);
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
      const saved = await saveRules(serializeRules(rules));
      setRules(parseRulesText((Array.isArray(saved) ? saved : serializeRules(rules)).join('\n')));
      message.success(t('routing.rulesSaved') || 'Rules saved');
      offerApply();
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
    setRules((prev) => prev.map((r, i) => (i === index ? { ...r, ...patch, raw: undefined } : r)));
  };

  const moveRule = (index: number, dir: -1 | 1) => {
    setRules((prev) => {
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
    setRules((prev) => (prev.length <= 1 ? prev : prev.filter((_, i) => i !== index)));
  };

  const addRule = () => {
    setRules((prev) => [...prev, emptyRule({ type: 'DOMAIN-SUFFIX', target: 'DIRECT' })]);
  };

  const onTemplate = async (id: string) => {
    const tpl = applyTemplate(id, {
      groupName: groups.find((g) => g.name === 'PROXY')?.name || groups[0]?.name || 'PROXY',
      existingProxies: proxyNames,
    });
    setRules(tpl.rules);
    if (tpl.groups && tpl.groups.length) {
      try {
        let next: GroupEntry[] = tpl.groups.map((g) => ({
          name: g.name,
          type: g.type || 'select',
          proxies: g.proxies?.length ? g.proxies : ['DIRECT'],
          ...(g.url ? { url: g.url } : {}),
          ...(g.interval != null ? { interval: g.interval } : {}),
        }));
        if (tpl.mergeGroups) {
          const byName = new Map<string, GroupEntry>(groups.map((g) => [g.name, g]));
          for (const g of next) {
            byName.set(g.name, { ...(byName.get(g.name) || {}), ...g });
          }
          next = Array.from(byName.values());
        }
        const saved = await saveGroups(next);
        setGroups(Array.isArray(saved) ? saved : next);
      } catch (e: any) {
        message.error(errMsg(e));
        return;
      }
    }
    message.success(
      (t('routing.templateApplied') || 'Template applied — save to persist') +
        (id.startsWith('community-')
          ? ' · ' + (t('routing.tplCommunityHint') || 'Needs Geo files (Settings → update geodata)')
          : ''),
    );
  };

  const templateMenu = {
    items: [
      { key: 'direct_only', label: t('routing.tplDirect') || 'MATCH → DIRECT only' },
      { key: 'cn_direct', label: t('routing.tplCnDirect') || 'GEOIP CN → DIRECT, else group' },
      { key: 'reject_ads', label: t('routing.tplAds') || 'Sample ad domains → REJECT' },
      { key: 'via_group', label: t('routing.tplViaGroup') || 'MATCH → first group / PROXY' },
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
      <Dropdown menu={templateMenu}>
      <Button size="small" icon={<ListPlus size={16} />} onClick={addRule}>
        {t('routing.addRule') || 'Add rule'}
      </Button>
      <Button type="primary" size="small" icon={<CircleCheck size={16} />} loading={saving} onClick={onSaveRules}>
        {t('common.save') || 'Save'}
      </Button>
      <Button
        size="small"
        icon={<Play size={16} />}
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
      <Typography.Paragraph type="secondary" style={{ marginTop: -8, marginBottom: 12 }}>
        {t('routing.pageHint') ||
          'Proxy-groups and rules are for **client** Mihomo/Clash subscriptions only (not server-side split). Save, then update the subscription in the client. Panel core always uses MATCH,DIRECT.'}
      </Typography.Paragraph>

      <Card
        title={t('routing.groups')}
        extra={
          <Button type="primary" icon={<FolderPlus size={16} />} onClick={() => setGroupOpen(true)}>
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
                  <Button size="small" danger icon={<Trash2 size={16} />} />
                </Popconfirm>
              ),
            },
          ]}
        />
      </Card>

      <Card title={t('routing.rules')} extra={isMobile ? undefined : ruleCardExtra}>
        {isMobile ? <div style={{ marginBottom: 12 }}>{ruleCardExtra}</div> : null}
        <div style={{ display: 'flex', flexDirection: 'column', gap: 10 }}>
          {rules.map((row, index) => (
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
                    <Button size="small" icon={<ArrowUp size={14} />} disabled={index === 0} onClick={() => moveRule(index, -1)} />
                  </Tooltip>
                  <Tooltip title={t('routing.moveDown') || 'Move down'}>
                    <Button
                      size="small"
                      icon={<ArrowDown size={14} />}
                      disabled={index === rules.length - 1}
                      onClick={() => moveRule(index, 1)}
                    />
                  </Tooltip>
                  <Button size="small" danger icon={<Trash2 size={14} />} disabled={rules.length <= 1} onClick={() => removeRule(index)} />
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
