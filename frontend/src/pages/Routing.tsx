import React, { useEffect, useState } from 'react';
import { Card, Table, Button, Space, Modal, Form, Input, InputNumber, Select, message, Popconfirm } from 'antd';
import { PlusOutlined, DeleteOutlined, SaveOutlined } from '@ant-design/icons';
import { fetchGroups, saveGroups, fetchRules, saveRules, GroupEntry } from '../api/routing';
import { useI18n } from '../i18n';

const RoutingPage: React.FC = () => {
  const { t } = useI18n();
  const [groups, setGroups] = useState<GroupEntry[]>([]);
  const [rulesText, setRulesText] = useState('');
  const [loading, setLoading] = useState(false);
  const [groupOpen, setGroupOpen] = useState(false);
  const [form] = Form.useForm();

  const load = async () => {
    setLoading(true);
    try {
      const [g, r] = await Promise.all([fetchGroups(), fetchRules()]);
      setGroups(g || []);
      setRulesText((r || []).join('\n'));
    } catch (e: any) {
      message.error(e.message || t('common.error'));
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => { load(); }, []);

  const onSaveRules = async () => {
    const rules = rulesText.split('\n').map((s) => s.trim()).filter(Boolean);
    try {
      await saveRules(rules);
      message.success(t('routing.rulesSaved'));
      load();
    } catch (e: any) {
      message.error(e.message || t('common.error'));
    }
  };

  const onAddGroup = async (values: any) => {
    const proxies = String(values.proxies || '').split(',').map((s: string) => s.trim()).filter(Boolean);
    const next = [...groups, { name: values.name, type: values.type || 'select', proxies, url: values.url, interval: values.interval }];
    try {
      await saveGroups(next);
      message.success(t('routing.groupSaved'));
      setGroupOpen(false);
      form.resetFields();
      load();
    } catch (e: any) {
      message.error(e.message || t('common.error'));
    }
  };

  const onDeleteGroup = async (idx: number) => {
    const next = groups.filter((_, i) => i !== idx);
    try {
      await saveGroups(next);
      message.success(t('routing.groupDeleted'));
      load();
    } catch (e: any) {
      message.error(e.message || t('common.error'));
    }
  };

  return (
    <div>
      <h2>{t('routing.title')}</h2>
      <p style={{ opacity: 0.65 }}>{t('routing.subtitle')}</p>
      <Card title={t('routing.groups')} extra={<Button type="primary" icon={<PlusOutlined />} onClick={() => setGroupOpen(true)}>{t('routing.addGroup')}</Button>} style={{ marginBottom: 16 }}>
        <Table loading={loading} rowKey={(_, i) => String(i)} dataSource={groups} columns={[
          { title: t('common.name'), dataIndex: 'name' },
          { title: t('common.type'), dataIndex: 'type' },
          { title: t('routing.proxies'), dataIndex: 'proxies', render: (v: string[]) => (v || []).join(', ') },
          { title: t('common.actions'), key: 'a', render: (_: any, __: any, idx: number) => (
            <Popconfirm title={t('common.confirmDelete')} onConfirm={() => onDeleteGroup(idx)}>
              <Button size="small" danger icon={<DeleteOutlined />} />
            </Popconfirm>
          )},
        ]} />
      </Card>
      <Card title={t('routing.rules')} extra={<Button type="primary" icon={<SaveOutlined />} onClick={onSaveRules}>{t('common.save')}</Button>}>
        <Input.TextArea rows={12} value={rulesText} onChange={(e) => setRulesText(e.target.value)} placeholder={'GEOIP,CN,DIRECT\nMATCH,PROXY'} />
        <div style={{ marginTop: 8, opacity: 0.65, fontSize: 12 }}>{t('routing.rulesHint')}</div>
      </Card>
      <Modal open={groupOpen} title={t('routing.addGroup')} onCancel={() => setGroupOpen(false)} onOk={() => form.submit()} destroyOnClose>
        <Form form={form} layout="vertical" onFinish={onAddGroup} initialValues={{ type: 'select' }}>
          <Form.Item name="name" label={t('common.name')} rules={[{ required: true }]}><Input /></Form.Item>
          <Form.Item name="type" label={t('common.type')}>
            <Select options={[
              { value: 'select', label: 'select' }, { value: 'url-test', label: 'url-test' },
              { value: 'fallback', label: 'fallback' }, { value: 'load-balance', label: 'load-balance' },
            ]} />
          </Form.Item>
          <Form.Item name="proxies" label={t('routing.proxies')} tooltip={t('routing.proxiesHint')}>
            <Input placeholder="DIRECT, Proxy1, Proxy2" />
          </Form.Item>
          <Form.Item name="url" label="URL (url-test)"><Input placeholder="http://www.gstatic.com/generate_204" /></Form.Item>
          <Form.Item name="interval" label="Interval (s)"><InputNumber min={0} style={{ width: '100%' }} /></Form.Item>
        </Form>
      </Modal>
    </div>
  );
};

export default RoutingPage;
