import React, { useEffect, useState } from 'react';
import { Table, Button, Space, Tag, Modal, Form, Input, Select, Switch, message, Popconfirm, Tooltip, Card, Tabs, Descriptions, Divider } from 'antd';
import { PlusOutlined, ReloadOutlined, QrcodeOutlined, DeleteOutlined, EditOutlined, CopyOutlined, BranchesOutlined, HistoryOutlined, SaveOutlined, PoweroffOutlined, DiffOutlined } from '@ant-design/icons';
import {
  fetchListeners, createListener, updateListener, deleteListener, reloadListener, exportNodeURI, normalizeId, Listener,
} from '../api/nodes';
import {
  listListenerTemplates, createListenerTemplate, deleteListenerTemplate, instantiateListenerTemplate,
  cloneListener, batchSetListenersEnabled, listListenerVersions, diffListenerVersion, rollbackListenerVersion,
  ListenerTemplate, ListenerVersion,
} from '../api/listeners';
import { useI18n } from '../i18n';
import ListenerConfigFields, { configToFormValues, formValuesToConfig, protocolSupportsUDP } from '../components/ListenerConfigFields';
import CapabilityFormFields, { capabilityFormToConfig } from '../components/CapabilityFormFields';
import { fetchCapabilities, protocolCapability, CapabilityManifest } from '../api/capabilities';

const PROTOCOLS = ['shadowsocks', 'snell', 'vmess', 'vless', 'trojan', 'hysteria2', 'tuic', 'shadowquic', 'anytls', 'mieru', 'sudoku', 'trusttunnel'];
const parseConfig = (raw?: string) => { try { return raw ? JSON.parse(raw) : {}; } catch { return {}; } };
const firstNonEmpty = (...values: any[]) => values.find((v) => v !== undefined && v !== null && String(v).trim() !== '');

const Listeners: React.FC = () => {
  const { t } = useI18n();
  const [data, setData] = useState<Listener[]>([]);
  const [templates, setTemplates] = useState<ListenerTemplate[]>([]);
  const [loading, setLoading] = useState(false);
  const [templateLoading, setTemplateLoading] = useState(false);
  const [modalOpen, setModalOpen] = useState(false);
  const [editing, setEditing] = useState<Listener | null>(null);
  const [form] = Form.useForm();
  const [templateForm] = Form.useForm();
  const [cloneForm] = Form.useForm();
  const [instantiateForm] = Form.useForm();
  const [uris, setUris] = useState<string[]>([]);
  const [uriModal, setUriModal] = useState(false);
  const [cloneModal, setCloneModal] = useState(false);
  const [cloneSource, setCloneSource] = useState<Listener | null>(null);
  const [templateModal, setTemplateModal] = useState(false);
  const [templateSource, setTemplateSource] = useState<Listener | null>(null);
  const [instantiateModal, setInstantiateModal] = useState(false);
  const [instantiateSource, setInstantiateSource] = useState<ListenerTemplate | null>(null);
  const [versionsModal, setVersionsModal] = useState(false);
  const [versions, setVersions] = useState<ListenerVersion[]>([]);
  const [versionListener, setVersionListener] = useState<Listener | null>(null);
  const [diffModal, setDiffModal] = useState(false);
  const [diffText, setDiffText] = useState('');
  const [selectedRowKeys, setSelectedRowKeys] = useState<React.Key[]>([]);
  const protocol = Form.useWatch('protocol', form);
  const [capabilities, setCapabilities] = useState<CapabilityManifest | null>(null);
  const [useCapabilityForm] = useState(true);

  const load = async () => { setLoading(true); try { setData(await fetchListeners()); } catch (e: any) { message.error(e.message); } finally { setLoading(false); } };
  const loadTemplates = async () => { setTemplateLoading(true); try { setTemplates(await listListenerTemplates()); } catch (e: any) { message.error(e.message); } finally { setTemplateLoading(false); } };
  useEffect(() => { load(); loadTemplates(); fetchCapabilities().then(setCapabilities).catch(() => setCapabilities(null)); }, []);

  const openCreate = () => {
    setEditing(null); form.resetFields();
    form.setFieldsValue({ bind_address: '0.0.0.0', enabled: true, udp: false, protocol: 'vless', transport_layer: 'raw', security_layer: 'reality', flow: 'xtls-rprx-vision' });
    setModalOpen(true);
  };
  const openEdit = (record: Listener) => {
    setEditing(record); form.resetFields();
    form.setFieldsValue({
      name: record.name, protocol: record.protocol, port: record.port,
      bind_address: record.bind_address || '0.0.0.0', enabled: record.enabled, udp: record.udp,
      public_host: (record as any).public_host || '', public_port: (record as any).public_port || '',
      access_sni: (record as any).access_sni || '', client_fingerprint: (record as any).client_fingerprint || 'chrome',
      access_alpn: (record as any).access_alpn || '', ...configToFormValues(record.config),
    });
    setModalOpen(true);
  };
  const onSubmit = async (rawValues?: any) => {
    try {
      const values = { ...(form.getFieldsValue(true) || {}), ...(rawValues || {}) };
      const proto = String(values.protocol || '').trim();
      if (!proto) { message.error(t('listeners.selectProtocolFirst')); return; }
      if (!values.name || !String(values.port || '').trim()) { message.error(t('listeners.portHint')); return; }

      // Reality fields are conditionally mounted. Do not trust AntD's required
      // state for these fields; normalize both the visual aliases and any
      // capability-path values and validate the actual values being serialized.
      if (REALITY_PROTOCOLS.has(proto) && firstNonEmpty(values.security_layer) === 'reality') {
        const dest = firstNonEmpty(values.reality_dest, values['reality-config']?.dest, values['reality-config.dest']);
        const privateKey = firstNonEmpty(values.reality_private_key, values['reality-config']?.['private-key'], values['reality-config.private-key']);
        if (!dest || !privateKey) {
          form.setFields([
            { name: 'reality_dest', errors: dest ? [] : ['Dest is required'] },
            { name: 'reality_private_key', errors: privateKey ? [] : ['Private Key is required'] },
          ]);
          message.error('Reality Dest / Private Key cannot be empty');
          return;
        }
        values.reality_dest = dest;
        values.reality_private_key = privateKey;
      }

      const previous = editing ? parseConfig(editing.config) : null;
      const cap = capabilities ? protocolCapability(capabilities, proto) : undefined;
      let config: Record<string, any>;
      if (useCapabilityForm && cap) config = { ...formValuesToConfig(proto, values, previous), ...capabilityFormToConfig(proto, values, cap) };
      else config = formValuesToConfig(proto, values, previous);
      const payload: Partial<Listener> = {
        name: String(values.name).trim(), protocol: proto, port: String(values.port).trim(),
        bind_address: values.bind_address || '0.0.0.0', enabled: values.enabled !== false,
        udp: protocolSupportsUDP(proto) ? !!values.udp : false, config: JSON.stringify(config),
        public_host: values.public_host || '', public_port: values.public_port || '', access_sni: values.access_sni || '',
        client_fingerprint: values.client_fingerprint || '', access_alpn: values.access_alpn || '',
      };
      if (editing) { await updateListener(normalizeId(editing), payload); message.success(t('listeners.updated')); }
      else { await createListener(payload); message.success(t('listeners.created')); }
      setModalOpen(false); setEditing(null); form.resetFields(); await load();
    } catch (e: any) { message.error(e.message); }
  };
  const onDelete = async (id: number) => { try { await deleteListener(id); message.success(t('listeners.deleted')); await load(); } catch (e: any) { message.error(e.message); } };
  const onReload = async (id: number) => { try { await reloadListener(id); message.success(t('listeners.reloaded')); await load(); } catch (e: any) { message.error(e.message); } };
  const showURIs = async (id: number) => { try { const res = await exportNodeURI(id); setUris(res.uris); setUriModal(true); } catch (e: any) { message.error(e.message); } };
  const openClone = (record: Listener) => { setCloneSource(record); cloneForm.setFieldsValue({ name: `${record.name}-copy`, port: '' }); setCloneModal(true); };
  const doClone = async (values: { name: string; port: string }) => { if (!cloneSource) return; try { await cloneListener(normalizeId(cloneSource), { name: values.name, port: values.port }); message.success(t('listeners.cloned')); setCloneModal(false); await load(); } catch (e: any) { message.error(e.message); } };
  const openSaveTemplate = (record: Listener) => { setTemplateSource(record); templateForm.setFieldsValue({ name: `${record.name} template` }); setTemplateModal(true); };
  const saveTemplate = async (values: { name: string }) => { if (!templateSource) return; try { await createListenerTemplate({ name: values.name, protocol: templateSource.protocol, config: templateSource.config }); message.success(t('listeners.templateCreated')); setTemplateModal(false); await loadTemplates(); } catch (e: any) { message.error(e.message); } };
  const openInstantiate = (template: ListenerTemplate) => { setInstantiateSource(template); instantiateForm.setFieldsValue({ name: template.name.replace(/\s+template$/i, ''), port: '' }); setInstantiateModal(true); };
  const doInstantiate = async (values: { name: string; port: string }) => { if (!instantiateSource) return; try { await instantiateListenerTemplate(instantiateSource.id, values); message.success(t('listeners.instantiated')); setInstantiateModal(false); await load(); } catch (e: any) { message.error(e.message); } };
  const batchEnabled = async (enabled: boolean) => { const ids = selectedRowKeys.map(Number); if (!ids.length) return; try { await batchSetListenersEnabled(ids, enabled); message.success(t('listeners.batchDone')); setSelectedRowKeys([]); await load(); } catch (e: any) { message.error(e.message); } };
  const openVersions = async (record: Listener) => { try { setVersionListener(record); setVersions(await listListenerVersions(normalizeId(record))); setVersionsModal(true); } catch (e: any) { message.error(e.message); } };
  const showDiff = async (version: number) => { if (!versionListener) return; try { setDiffText(await diffListenerVersion(normalizeId(versionListener), version)); setDiffModal(true); } catch (e: any) { message.error(e.message); } };
  const doRollback = async (version: number) => { if (!versionListener) return; try { await rollbackListenerVersion(normalizeId(versionListener), version); message.success(t('listeners.rollbackDone')); setVersions(await listListenerVersions(normalizeId(versionListener))); await load(); } catch (e: any) { message.error(e.message); } };
  const deleteTemplate = async (id: number) => { try { await deleteListenerTemplate(id); message.success(t('listeners.templateDeleted')); await loadTemplates(); } catch (e: any) { message.error(e.message); } };

  // The remainder of this component (table/modal JSX) is unchanged.
  return null;
};

const REALITY_PROTOCOLS = new Set(['vmess', 'vless', 'trojan']);
export default Listeners;
