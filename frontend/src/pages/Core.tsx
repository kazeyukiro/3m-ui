import { useState, useEffect, useRef } from 'react';
import { Alert, Button, Card, Descriptions, Modal, Select, Space, Spin, Steps, Typography, message } from 'antd';
import { PlayCircleOutlined, StopOutlined, RedoOutlined, DownloadOutlined, RollbackOutlined } from '@ant-design/icons';
import { coreAPI, type CoreStatus, type CoreRelease, type CoreUpdateStatus } from '../api/core';
import { isCanceledError } from '../api/client';
import { useI18n } from '../i18n';
import useIsMobile from '../hooks/useIsMobile';
import PageHeader from '../components/PageHeader';
import { startVisiblePolling } from '../utils/visiblePolling';

const stages = ['preparing', 'resolving', 'downloading', 'verifying', 'validating', 'switching', 'starting', 'checking'];

export default function Core() {
  const { t } = useI18n();
  const isMobile = useIsMobile();
  const [status, setStatus] = useState<CoreStatus | null>(null);
  const [update, setUpdate] = useState<CoreUpdateStatus | null>(null);
  const [releases, setReleases] = useState<CoreRelease[]>([]);
  const [selected, setSelected] = useState<string>();
  const [loading, setLoading] = useState(true);
  const [requesting, setRequesting] = useState(false);
  const [checking, setChecking] = useState(false);
  const [error, setError] = useState('');
  const [confirmation, setConfirmation] = useState<'install' | 'rollback' | null>(null);
  const mounted = useRef(true);
  const busy = requesting || !!update?.busy;
  const job = update?.job;

  useEffect(() => {
    mounted.current = true;
    let delay = 5000;
    const stopPolling = startVisiblePolling(async signal => {
      delay = 5000;
      try {
        const [nextStatus, nextUpdate] = await Promise.all([coreAPI.status(signal), coreAPI.updateStatus(signal)]);
        if (signal.aborted) return;
        setStatus(nextStatus); setUpdate(nextUpdate); setError('');
        delay = nextUpdate.busy ? 1000 : 5000;
      } catch (e) {
        if (!signal.aborted && !isCanceledError(e)) setError((e as Error).message);
      } finally {
        if (!signal.aborted) setLoading(false);
      }
    }, () => delay);
    return () => { mounted.current = false; stopPolling(); };
  }, []);

  const action = async (path: 'start' | 'stop' | 'restart', successKey: 'started' | 'stopped' | 'restarted') => {
    setRequesting(true);
    try { await coreAPI.action(path); setStatus(await coreAPI.status()); message.success(t(`core.${successKey}`)); }
    catch (e) { message.error((e as Error).message || t('core.operationFailed')); }
    finally { if (mounted.current) setRequesting(false); }
  };
  const checkReleases = async () => {
    setChecking(true);
    try {
      const list = await coreAPI.releases();
      if (mounted.current) { setReleases(list); setSelected(list.find(item => !item.prerelease)?.version); }
      if (list.length === 0) message.info(t('core.updates.noVersions'));
    } catch (e) { message.error((e as Error).message); }
    finally { if (mounted.current) setChecking(false); }
  };
  const begin = async () => {
    if (!confirmation || (confirmation === 'install' && !selected)) return;
    setRequesting(true);
    try {
      const nextJob = confirmation === 'rollback' ? await coreAPI.rollback() : await coreAPI.install(selected!);
      if (mounted.current) {
        setUpdate(previous => previous ? { ...previous, busy: true, job: nextJob } : previous);
        setConfirmation(null);
      }
    } catch (e) { message.error((e as Error).message); }
    finally { if (mounted.current) setRequesting(false); }
  };
  const release = releases.find(item => item.version === selected);
  const latestStable = releases.find(item => !item.prerelease)?.version;
  const progress = !job ? 0 : ['switching', 'starting', 'checking', 'recovering', 'complete'].includes(job.stage) ? 3 : job.stage === 'validating' ? 2 : ['downloading', 'verifying'].includes(job.stage) ? 1 : 0;

  return (
    <div>
      <PageHeader title={t('core.title')} subtitle={t('core.subtitle')} />
      {error && <Alert type="error" showIcon title={t('core.unavailable')} description={error} style={{ marginBottom: 16 }} />}
      {loading ? <Spin /> : (
        <Space orientation="vertical" style={{ width: '100%' }} size="large">
          <Card>
            <Space orientation="vertical" style={{ width: '100%' }} size="large">
              <Alert title={status?.running ? t('core.running') : t('core.stopped')} type={status?.running ? 'success' : 'warning'} showIcon />
              <Descriptions bordered column={1}>
                <Descriptions.Item label={t('core.version')}>{status?.version || '-'}</Descriptions.Item>
                <Descriptions.Item label={t('core.pid')}>{status?.pid || '-'}</Descriptions.Item>
                <Descriptions.Item label={t('core.uptime')}>{status?.uptime || '-'}</Descriptions.Item>
                <Descriptions.Item label={t('core.updates.source')}>{t(update?.managed ? 'core.updates.managed' : 'core.updates.bundled')}</Descriptions.Item>
                <Descriptions.Item label={t('core.updates.previous')}>{update?.previous_version || t('core.updates.noPrevious')}</Descriptions.Item>
              </Descriptions>
              <Space wrap>
                <Button icon={<PlayCircleOutlined />} disabled={busy || !!error} onClick={() => action('start', 'started')}>{t('core.start')}</Button>
                <Button icon={<StopOutlined />} danger disabled={busy || !!error} onClick={() => action('stop', 'stopped')}>{t('core.stop')}</Button>
                <Button icon={<RedoOutlined />} disabled={busy || !!error} onClick={() => action('restart', 'restarted')}>{t('core.restart')}</Button>
              </Space>
            </Space>
          </Card>
          <Card title={t('core.updates.title')}>
            <Space orientation="vertical" size="large" style={{ width: '100%' }}>
              <Typography.Paragraph style={{ margin: 0 }}>{t('core.updates.description')}</Typography.Paragraph>
              {!update?.supported && <Alert type="warning" showIcon title={t('core.updates.unavailable')} description={update?.disabled_reason} />}
              <Space wrap style={{ width: '100%' }}>
                <Button onClick={checkReleases} loading={checking} disabled={busy || !update?.supported || !!error}>{t('core.updates.check')}</Button>
                <Select aria-label={t('core.updates.select')} placeholder={t('core.updates.select')} value={selected} onChange={setSelected} style={{ width: isMobile ? 260 : 320, maxWidth: '100%' }} disabled={busy || !releases.length} options={[true, false].map(pre => ({ label: t(pre ? 'core.updates.pre' : 'core.updates.stable'), options: releases.filter(item => item.prerelease === pre).map(item => ({ value: item.version, label: `${item.version}${item.prerelease ? ` · ${t('core.updates.pre')}` : ''}${item.version === latestStable ? ` · ${t('core.updates.latest')}` : ''}${item.version === status?.version ? ` · ${t('core.updates.current')}` : ''}` })) })).filter(group => group.options.length > 0)} />
                <Button type="primary" icon={<DownloadOutlined />} disabled={busy || !update?.supported || !selected || selected === status?.version || !!error} onClick={() => setConfirmation('install')}>{t('core.updates.install')}</Button>
                <Button icon={<RollbackOutlined />} disabled={busy || !update?.supported || !update?.previous_version || !!error} onClick={() => setConfirmation('rollback')}>{t('core.updates.rollback')}</Button>
              </Space>
              {release && <Typography.Link href={release.url} target="_blank" rel="noopener noreferrer">{t('core.updates.releaseNotes')} · {release.version}</Typography.Link>}
              {release?.prerelease && <Alert type="warning" showIcon title={t('core.updates.preNotice')} />}
              {job && <>
                <Alert type={job.status === 'failed' ? 'error' : job.status === 'succeeded' ? 'success' : 'info'} showIcon title={`${t(`core.updates.${job.status}`)} · ${job.version}`} description={<>
                  <div>{t(`core.updates.stage.${stages.includes(job.stage) || ['recovering', 'complete', 'interrupted'].includes(job.stage) ? job.stage : 'preparing'}`)}</div>
                  {job.error && <div style={{ overflowWrap: 'anywhere' }}>{job.error}</div>}
                  {job.rolled_back && <div>{t('core.updates.restored')}</div>}
                </>} />
                <Steps size="small" orientation={isMobile ? 'vertical' : 'horizontal'} current={job.status === 'succeeded' ? 4 : progress} status={job.status === 'failed' ? 'error' : 'process'} items={['prepare', 'download', 'validate', 'activate'].map(step => ({ title: t(`core.updates.steps.${step}`) }))} />
              </>}
            </Space>
          </Card>
        </Space>
      )}
      <Modal open={confirmation !== null} title={t(confirmation === 'rollback' ? 'core.updates.rollback' : 'core.updates.install')} okText={t('common.confirm')} cancelText={t('common.cancel')} confirmLoading={requesting} onOk={begin} onCancel={() => { if (!requesting) setConfirmation(null); }} closable={!requesting} maskClosable={!requesting} cancelButtonProps={{ disabled: requesting }}>
        <p>{t('core.updates.confirm')} <strong>{confirmation === 'rollback' ? update?.previous_version : selected}</strong></p>
        {confirmation === 'install' && release?.prerelease && <p>{t('core.updates.preNotice')}</p>}
        <p>{t(status?.running ? 'core.updates.interruption' : 'core.updates.staysStopped')}</p>
      </Modal>
    </div>
  );
}
