import { Alert, Button, Descriptions, Drawer, Space, Table, Tag, Typography } from 'antd';
import { ReloadOutlined } from '@ant-design/icons';
import type { ListenerRuntime } from '../api/listenerRuntime';
import type { Listener } from '../api/nodes';
import { listenerAvailability, listenerDiagnosticReason } from '../utils/listenerAvailability';
import { useListenerRuntimeMessages } from '../i18n/listenerRuntime';

export function ListenerRuntimeTag({ status, enabled, checking = false, onClick }: {
  status?: ListenerRuntime; enabled: boolean; checking?: boolean; onClick?: () => void;
}) {
  const text = useListenerRuntimeMessages();
  const state = listenerAvailability(status, enabled, checking);
  return <Button type="text" size="small" onClick={onClick} aria-label={`${text.title}: ${text[state]}`}>
    <Tag color={{ checking: 'processing', disabled: 'default', available: 'success', unavailable: 'error', unknown: 'warning', pending: 'default' }[state]} style={{ marginInlineEnd: 0 }}>{text[state]}</Tag>
  </Button>;
}

export function ListenerRuntimeDrawer({ listener, status, checking, onClose, onCheck }: {
  listener: Listener | null; status?: ListenerRuntime; checking: boolean; onClose: () => void; onCheck: () => void;
}) {
  const text = useListenerRuntimeMessages();
  // The API returns all stages together. While it runs, do not present a
  // previous result as progress for the current request.
  const connection = checking ? undefined : status?.connection_check;
  const reason = listenerDiagnosticReason(status);
  const unavailable = listenerAvailability(status, listener?.enabled ?? false, checking) === 'unavailable';
  const stepNames = ['client_config', 'client_start', 'proxy_request'] as const;
  const stepStates = { passed: text.passed, failed: text.failedStep, unknown: text.unknownStep };
  return <Drawer open={!!listener} title={`${text.details} — ${listener?.name || ''}`} onClose={onClose} size="default"
    extra={<Button icon={<ReloadOutlined />} loading={checking} disabled={!listener?.enabled} onClick={onCheck}>{text.check}</Button>}>
    {listener && <Space orientation="vertical" size="large" style={{ width: '100%' }}>
      <ListenerRuntimeTag enabled={listener.enabled} status={status} checking={checking} />
      {unavailable && <Alert type="error" showIcon title={status?.state === 'not_listening' ? text.nodeNotStarted : text.connectionFailed}
        description={<><div>{text.reasons[reason] || text.unknown}</div>{text.actions[reason] && <div style={{ marginTop: 8 }}>{text.actions[reason]}</div>}</>} />}
      <Alert type="info" title={`${text.source}：${text.localSource}`} description={text.localScope} showIcon />
      <Descriptions column={1} size="small" items={[
        { key: 'time', label: text.lastChecked, children: checking ? text.checking : (connection?.checked_at || status?.checked_at) ? new Date(connection?.checked_at || status!.checked_at).toLocaleString() : '—' },
        ...(!unavailable ? [{ key: 'reason', label: text.reason, children: checking ? text.checking : text.reasons[reason] || text.pending }] : []),
        { key: 'address', label: text.address, children: checking ? text.waitingResult : connection?.address || '—' },
        { key: 'target', label: text.target, children: checking ? text.waitingResult : connection?.target || '—' },
        { key: 'delay', label: text.delay, children: checking ? text.checking : connection?.delay_ms !== undefined ? `${connection.delay_ms} ms` : '—' },
      ]} />
      <Descriptions column={1} bordered size="small" items={[
        { key: 'local', label: text.localCheck, children: checking ? <Tag color="processing">{text.checking}</Tag> : status ? <><Tag color={status.state === 'listening' ? 'success' : status.state === 'not_listening' ? 'error' : 'default'}>{text[status.state]}</Tag><div>{text.reasons[status.reason] || text.unknown}</div></> : text.notRun },
        ...stepNames.map(name => {
          const step = connection?.steps.find(value => value.name === name);
          return { key: name, label: text[name], children: checking ? <Tag color="processing">{text.waitingResult}</Tag> : step ? <><Tag color={step.state === 'passed' ? 'success' : step.state === 'failed' ? 'error' : 'default'}>{stepStates[step.state]}</Tag><div>{text.reasons[step.reason] || text.unknown}</div></> : text.notRun };
        }),
      ]} />
      <Typography.Text strong>{text.endpoints}</Typography.Text>
      <Table size="small" locale={checking ? { emptyText: text.waitingResult } : undefined} dataSource={status?.endpoints || []} rowKey={e => `${e.network}:${e.address}:${e.port}`} pagination={{ pageSize: 10, hideOnSinglePage: true }} columns={[
        { title: 'TCP / UDP', dataIndex: 'network', render: (value: string) => value.toUpperCase() },
        { title: 'IP:Port', render: (_, e) => `${e.address.includes(':') ? `[${e.address}]` : e.address}:${e.port}` },
        { title: text.socketStatus, render: (_, e) => checking ? <Tag color="processing">{text.checking}</Tag> : status?.state === 'unknown' ? text.unverified : <Tag color={e.bound ? 'success' : 'error'}>{e.bound ? text.bound : text.missing}</Tag> },
      ]} />
    </Space>}
  </Drawer>;
}
