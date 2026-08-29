import React, { useCallback, useEffect, useMemo, useState } from 'react';
import {
  Alert, Button, Card, Collapse, Empty, Input, Select, Space, Spin, Tabs, Tag, Typography, message,
} from 'antd';
import { CopyOutlined, QrcodeOutlined, ReloadOutlined, LinkOutlined } from '@ant-design/icons';
import { useSearchParams } from 'react-router-dom';
import { useI18n } from '../i18n';
import { copyText } from '../utils/clipboard';
import {
  fetchUsers, fetchUserSubscription, rotateUserSubscription, fetchUserNodes,
  ProxyUser, BoundNode,
} from '../api/users';
import { exportNodeURI } from '../api/nodes';
import QRCode from '../components/QRCode';

const { Title, Paragraph } = Typography;

function withTarget(base: string, target: string) {
  if (!base) return '';
  const join = base.includes('?') ? '&' : '?';
  return `${base}${join}target=${target}`;
}

const CopyField: React.FC<{ label: string; value: string; qr?: boolean }> = ({ label, value, qr }) => {
  const { t } = useI18n();
  if (!value) return null;
  return (
    <div style={{ marginBottom: 16 }}>
      <div style={{ marginBottom: 4, fontWeight: 500 }}>{label}</div>
      <Input
        value={value}
        readOnly
        addonAfter={
          <Button
            type="text"
            size="small"
            icon={<CopyOutlined />}
            onClick={async () => {
              const ok = await copyText(value);
              if (ok) message.success(t('common.copied') || 'Copied');
              else message.error(t('common.copyFailed') || 'Copy failed');
            }}
          />
        }
      />
      {qr && (
        <div style={{ textAlign: 'center', marginTop: 12 }}>
          <QRCode value={value} size={160} />
        </div>
      )}
    </div>
  );
};

const SharePage: React.FC = () => {
  const { t } = useI18n();
  const [searchParams, setSearchParams] = useSearchParams();
  const [users, setUsers] = useState<ProxyUser[]>([]);
  const [userId, setUserId] = useState<number | undefined>();
  const [shareUrl, setShareUrl] = useState('');
  const [nodes, setNodes] = useState<BoundNode[]>([]);
  const [uriMap, setUriMap] = useState<Record<number, { uris: string[]; hint?: string; loading?: boolean; error?: string }>>({});
  const [loading, setLoading] = useState(false);
  const [subLoading, setSubLoading] = useState(false);

  const selected = useMemo(() => users.find((u) => u.id === userId), [users, userId]);

  const loadUsers = useCallback(async () => {
    setLoading(true);
    try {
      const list = await fetchUsers();
      setUsers(list || []);
      const q = Number(searchParams.get('user') || 0);
      if (q > 0 && (list || []).some((u) => u.id === q)) {
        setUserId(q);
      } else if (!userId && list?.length) {
        setUserId(list[0].id);
      }
    } catch (e: any) {
      message.error(e.message || t('common.error'));
    } finally {
      setLoading(false);
    }
  }, [searchParams, t]); // eslint-disable-line

  useEffect(() => {
    loadUsers();
  }, [loadUsers]);

  const loadShare = useCallback(async (id: number) => {
    setSubLoading(true);
    setShareUrl('');
    setNodes([]);
    setUriMap({});
    try {
      const [sub, bound] = await Promise.all([
        fetchUserSubscription(id),
        fetchUserNodes(id),
      ]);
      setShareUrl(sub.url || '');
      setNodes(bound || []);
    } catch (e: any) {
      message.error(e.message || t('common.error'));
    } finally {
      setSubLoading(false);
    }
  }, [t]);

  useEffect(() => {
    if (userId) {
      loadShare(userId);
      setSearchParams({ user: String(userId) }, { replace: true });
    }
  }, [userId]); // eslint-disable-line

  const loadNodeURIs = async (nodeId: number) => {
    setUriMap((m) => ({ ...m, [nodeId]: { ...(m[nodeId] || { uris: [] }), loading: true } }));
    try {
      const data = await exportNodeURI(nodeId);
      const uris = (data.uris && data.uris.length ? data.uris : ((data as any).uri ? [(data as any).uri] : [])).filter(Boolean);
      setUriMap((m) => ({
        ...m,
        [nodeId]: { uris, hint: (data as any).hint, loading: false },
      }));
    } catch (e: any) {
      setUriMap((m) => ({
        ...m,
        [nodeId]: { uris: [], loading: false, error: e.message || 'failed' },
      }));
    }
  };

  const onRotate = async () => {
    if (!userId) return;
    setSubLoading(true);
    try {
      const res = await rotateUserSubscription(userId);
      setShareUrl(res.url || '');
      message.success(t('users.subRotated') || 'Subscription rotated');
    } catch (e: any) {
      message.error(e.message || t('common.error'));
    } finally {
      setSubLoading(false);
    }
  };

  const subTab = (
    <Spin spinning={subLoading}>
      {!shareUrl && !subLoading ? (
        <Empty description={t('share.noSub') || 'No subscription URL'} />
      ) : (
        <>
          <Alert
            type="info"
            showIcon
            style={{ marginBottom: 16 }}
            message={t('share.subHint') || 'Copy a URL into the client. Default (no target) is Mihomo YAML. Use target=v2ray for classic Base64 URI list.'}
          />
          <CopyField label={t('users.subMihomo') || 'Mihomo / Clash YAML'} value={shareUrl} qr />
          <CopyField label={t('users.subV2ray') || 'V2Ray / Base64'} value={withTarget(shareUrl, 'v2ray')} />
          <CopyField label={t('users.subSingbox') || 'Sing-box JSON'} value={withTarget(shareUrl, 'singbox')} />
          <CopyField label={t('share.subHtml') || 'Subscription info page (HTML)'} value={`${shareUrl}${shareUrl.includes('?') ? '&' : '?'}html=1`} />
          <Button icon={<ReloadOutlined />} onClick={onRotate} loading={subLoading}>
            {t('users.rotateSub') || 'Rotate subscription token'}
          </Button>
        </>
      )}
    </Spin>
  );

  const uriTab = (
    <Spin spinning={subLoading}>
      {!nodes.length && !subLoading ? (
        <Empty description={t('share.noNodes') || 'No nodes bound to this user. Bind listeners in Users first.'} />
      ) : (
        <Collapse
          accordion
          onChange={(key) => {
            const id = Number(Array.isArray(key) ? key[0] : key);
            if (id && !uriMap[id]?.uris?.length && !uriMap[id]?.loading) {
              loadNodeURIs(id);
            }
          }}
          items={nodes.map((n) => ({
            key: String(n.id),
            label: (
              <Space>
                <span>{n.name}</span>
                <Tag>{n.protocol}</Tag>
                <Tag>{n.port}</Tag>
                {!n.enabled && <Tag color="default">off</Tag>}
              </Space>
            ),
            children: (
              <div>
                {uriMap[n.id]?.loading && <Spin />}
                {uriMap[n.id]?.error && <Alert type="error" message={uriMap[n.id].error} />}
                {uriMap[n.id]?.hint && (
                  <Alert type="warning" showIcon style={{ marginBottom: 8 }} message={uriMap[n.id].hint} />
                )}
                {(uriMap[n.id]?.uris || []).map((uri, i) => (
                  <CopyField key={i} label={`${t('share.uri') || 'URI'} #${i + 1}`} value={uri} qr={i === 0} />
                ))}
                {!uriMap[n.id]?.loading && !uriMap[n.id]?.uris?.length && !uriMap[n.id]?.error && (
                  <Button size="small" icon={<LinkOutlined />} onClick={() => loadNodeURIs(n.id)}>
                    {t('share.loadUri') || 'Load URI'}
                  </Button>
                )}
              </div>
            ),
          }))}
        />
      )}
    </Spin>
  );

  return (
    <div>
      <Title level={3} style={{ marginTop: 0 }}>
        <QrcodeOutlined style={{ marginRight: 8 }} />
        {t('share.title') || 'Share / Subscription'}
      </Title>
      <Paragraph type="secondary">
        {t('share.subtitle') || 'Unified view: user subscription links and per-node share URIs.'}
      </Paragraph>

      <Card size="small" style={{ marginBottom: 16 }}>
        <Space wrap style={{ width: '100%' }} align="center">
          <span>{t('share.selectUser') || 'User'}</span>
          <Select
            showSearch
            style={{ minWidth: 220 }}
            loading={loading}
            value={userId}
            placeholder={t('share.selectUser') || 'Select user'}
            optionFilterProp="label"
            onChange={(v) => setUserId(v)}
            options={users.map((u) => ({
              value: u.id,
              label: `${u.username}${u.remark ? ` (${u.remark})` : ''}${u.enabled === false ? ' [off]' : ''}`,
            }))}
          />
          {selected && (
            <Tag color={selected.enabled ? 'success' : 'default'}>
              {selected.enabled ? (t('common.enabled') || 'enabled') : (t('common.disabled') || 'disabled')}
            </Tag>
          )}
          <Button icon={<ReloadOutlined />} onClick={() => userId && loadShare(userId)}>
            {t('common.refresh') || 'Refresh'}
          </Button>
        </Space>
      </Card>

      <Tabs
        items={[
          { key: 'sub', label: t('share.tabSub') || 'Subscription links', children: subTab },
          { key: 'uri', label: t('share.tabUri') || 'Node URIs', children: uriTab },
        ]}
      />
    </div>
  );
};

export default SharePage;
