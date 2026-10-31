import { Alert, Button, Descriptions, Drawer, Space, Tag } from 'antd';
import { IconRefreshList } from '../icons';
import type { ConnectionCheck } from '../api/listenerRuntime';
import { useOutboundConnectionMessages } from '../i18n/outboundConnection';

export function OutboundConnectionResult({ result }: { result?: ConnectionCheck }) {
  const text = useOutboundConnectionMessages();
  return <Tag color={result?.state === 'available' ? 'success' : result?.state === 'unavailable' ? 'error' : 'default'}>
    {result ? text[result.state] : text.pending}
    {result?.state === 'available' && result.delay_ms !== undefined ? ` · ${result.delay_ms} ms` : ''}
  </Tag>;
}

export function OutboundConnectionDrawer({ name, result, checking, disabled, onClose, onCheck }: {
  name: string | null; result?: ConnectionCheck; checking: boolean; disabled: boolean; onClose: () => void; onCheck: () => void;
}) {
  const text = useOutboundConnectionMessages();
  const connection = checking ? undefined : result;
  const stepStates = { passed: text.passed, failed: text.failedStep, unknown: text.unknownStep };
  return <Drawer open={name !== null} title={`${text.details} — ${name || ''}`} onClose={onClose}
    extra={<Button icon={<IconRefreshList />} loading={checking} disabled={disabled} onClick={onCheck}>{text.check}</Button>}>
    <Space orientation="vertical" size="large" style={{ width: '100%' }}>
      {checking ? <Tag color="processing">{text.checking}</Tag> : <OutboundConnectionResult result={connection} />}
      <Alert type="info" showIcon title={text.source} description={text.scope} />
      <Descriptions column={1} size="small" items={[
        { key: 'time', label: text.lastChecked, children: checking ? text.checking : connection ? new Date(connection.checked_at).toLocaleString() : '—' },
        { key: 'reason', label: text.reason, children: checking ? text.waitingResult : connection ? text.reasons[connection.reason] || text.unknown : text.pending },
        { key: 'address', label: text.address, children: connection?.address || '—' },
        { key: 'target', label: text.target, children: connection?.target || '—' },
        { key: 'delay', label: text.delay, children: connection?.delay_ms !== undefined ? `${connection.delay_ms} ms` : '—' },
      ]} />
      <Descriptions column={1} bordered size="small" items={(['client_config', 'client_start', 'proxy_request'] as const).map(name => {
        const step = connection?.steps.find(step => step.name === name);
        return { key: name, label: text[name], children: checking ? text.waitingResult : step ? <><Tag color={step.state === 'passed' ? 'success' : step.state === 'failed' ? 'error' : 'default'}>{stepStates[step.state]}</Tag><div>{text.reasons[step.reason] || text.unknown}</div></> : text.notRun };
      })} />
    </Space>
  </Drawer>;
}
