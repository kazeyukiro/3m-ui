import { useCallback, useEffect, useRef, useState } from 'react';
import { Alert, Button, Form, Input, Space, Typography } from 'antd';
import { scanRealityTargets, RealityTarget } from '../api/listeners';
import { useI18n } from '../i18n';
import { clearScannedRealityNames } from '../utils/realityTarget';

/** A scan only proposes form values. Saving never runs a scan or rotates a target. */
export default function RealityTargetFields({ autoSelect = false }: { autoSelect?: boolean }) {
  const form = Form.useFormInstance();
  const { t } = useI18n();
  const dest = Form.useWatch('reality_dest', form);
  const serverNames = Form.useWatch('reality_server_names', { form, preserve: true });
  const sni = Form.useWatch('access_sni', { form, preserve: true });
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState('');
  const [selected, setSelected] = useState<RealityTarget | null>(null);
  const [candidates, setCandidates] = useState<RealityTarget[]>([]);
  const attempted = useRef(false);
  const scannedName = useRef<string | null>(null);
  const request = useRef<AbortController | null>(null);
  const snapshot = useCallback(() => JSON.stringify(form.getFieldsValue(['reality_dest', 'reality_server_names', 'access_sni'])), [form]);
  const currentSnapshot = snapshot();
  const previousSnapshot = useRef(currentSnapshot);

  const cancel = useCallback(() => {
    request.current?.abort();
    request.current = null;
    setLoading(false);
  }, []);

  // Editing any target/SNI field invalidates the outstanding response. This
  // also covers changing security/protocol and closing/resetting the form.
  useEffect(() => {
    if (previousSnapshot.current !== currentSnapshot) cancel();
    previousSnapshot.current = currentSnapshot;
  }, [currentSnapshot, cancel]);
  useEffect(() => () => { request.current?.abort(); }, []);

  const apply = useCallback((target: RealityTarget) => {
    scannedName.current = target.server_name;
    form.setFieldsValue({
      reality_dest: target.target,
      reality_server_names: [target.server_name],
      access_sni: target.server_name,
    });
    setSelected(target);
    setError('');
  }, [form]);

  const scan = useCallback(async () => {
    attempted.current = true;
    cancel();
    const controller = new AbortController();
    const startingSnapshot = snapshot();
    request.current = controller;
    setLoading(true);
    setError('');
    setCandidates([]);
    try {
      const result = await scanRealityTargets(controller.signal);
      if (controller.signal.aborted || request.current !== controller || snapshot() !== startingSnapshot) return;
      const eligible = result.candidates.filter(candidate => candidate.eligible);
      setCandidates(eligible);
      if (result.selected) apply(result.selected);
      else setError(t('realityScan.noTargets'));
    } catch (e: unknown) {
      if (!controller.signal.aborted && request.current === controller) {
        setError(e instanceof Error ? e.message : t('realityScan.failed'));
      }
    } finally {
      if (request.current === controller) {
        request.current = null;
        setLoading(false);
      }
    }
  }, [cancel, snapshot, apply, t]);

  useEffect(() => {
    // Defer until after StrictMode's setup/cleanup cycle and Form subscriptions.
    let active = true;
    void Promise.resolve().then(() => {
      if (active && autoSelect && !attempted.current && !String(form.getFieldValue('reality_dest') || '').trim()
        && !form.getFieldValue('access_sni') && !(form.getFieldValue('reality_server_names') || []).length) void scan();
    });
    return () => { active = false; };
  }, [autoSelect, form, scan]);

  const changeTarget = () => {
    const alternatives = candidates.filter(candidate => candidate.target !== form.getFieldValue('reality_dest'));
    if (alternatives.length) {
      cancel();
      const random = crypto.getRandomValues(new Uint32Array(1))[0] / 0x100000000;
      apply(alternatives[Math.floor(random * alternatives.length)]);
    } else void scan();
  };

  return <>
    <Form.Item name="reality_dest" label={t('listeners.realityDest')}
      rules={[{ required: true, whitespace: true, message: t('realityScan.required') }]}
      extra={t('realityScan.hint')}>
      <Input placeholder={t('realityScan.placeholder')} onChange={() => {
        attempted.current = true;
        cancel();
        form.setFieldsValue(clearScannedRealityNames(scannedName.current, form.getFieldsValue(['reality_server_names', 'access_sni'])));
        scannedName.current = null;
        setSelected(null);
        setError('');
      }} />
    </Form.Item>
    <Space wrap style={{ marginBottom: 12 }}>
      <Button loading={loading} onClick={() => void scan()}>{t('realityScan.scan')}</Button>
      <Button disabled={loading} onClick={changeTarget}>{t('realityScan.change')}</Button>
      {loading && <Typography.Text type="secondary">{t('realityScan.scanning')}</Typography.Text>}
      {!loading && selected && selected.target === dest && sni === selected.server_name
        && serverNames?.length === 1 && serverNames[0] === selected.server_name && <Typography.Text type="success">
        {t('realityScan.passed')} · {selected.latency_ms} ms
      </Typography.Text>}
    </Space>
    {error && <Alert type="warning" showIcon title={error} style={{ marginBottom: 12 }} />}
    <Typography.Paragraph type="secondary">{t('realityScan.scope')}</Typography.Paragraph>
  </>;
}
