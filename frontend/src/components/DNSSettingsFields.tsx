import React from 'react';
import { Alert, Col, Collapse, Divider, Form, Input, InputNumber, Row, Select, Switch } from 'antd';
import type { FormItemProps } from 'antd';
import { useI18n } from '../i18n';
import DNSResolverPolicy from './DNSResolverPolicy';

export default function DNSSettingsFields() {
  const { t } = useI18n();
  const field = (name: string, input: React.ReactNode, props: FormItemProps = {}, wide = false) => (
    <Col key={name} xs={24} md={wide ? 24 : 12}>
      <Form.Item name={['dns', ...name.split('.')]} label={t(`config.dnsFields.${name.replace('.', '_')}`)} {...props}>
        {input}
      </Form.Item>
    </Col>
  );
  const choice = (name: string, options: string[]) => field(name, <Select allowClear placeholder={t('config.dnsDefaultValue')} options={options.map(value => ({ value, label: value }))} />);
  const boolean = (name: string) => field(name,
    <Select allowClear placeholder={t('config.dnsDefaultValue')} options={[{ value: 'true', label: t('common.enabled') }, { value: 'false', label: t('common.disabled') }]} />,
    { getValueProps: value => ({ value: value == null ? undefined : String(value) }), getValueFromEvent: value => value === undefined ? undefined : value === 'true' });
  const list = (name: string, hint = t('config.dnsListHint'), wide = false) => field(name,
    <Input.TextArea autoSize={{ minRows: 3, maxRows: 8 }} spellCheck={false} />,
    { extra: hint, getValueProps: (value?: string[]) => ({ value: value?.join('\n') ?? '' }), getValueFromEvent: event => event.target.value.split(/\r?\n/).map((line: string) => line.trim()) }, wide);

  return <>
    <Divider titlePlacement="start">{t('config.dns')}</Divider>
    <Alert type="info" showIcon title={t('config.dnsHint')} description={t('config.dnsWarpHint')} style={{ marginBottom: 16 }} />
    <Row gutter={[16, 0]}>
      {field('enable', <Switch />, { valuePropName: 'checked' })}
      {field('listen', <Input placeholder="0.0.0.0:1053" />, { extra: t('config.dnsListenHint') })}
      {choice('enhancedMode', ['redir-host', 'fake-ip'])}
      {boolean('ipv6')}
      {choice('cacheAlgorithm', ['lru', 'arc'])}
      {boolean('preferH3')}
      {boolean('useHosts')}
      {boolean('useSystemHosts')}
      {boolean('respectRules')}
    </Row>
    <Collapse style={{ marginBottom: 16 }} items={[
      { key: 'resolvers', label: t('config.dnsResolvers'), forceRender: true, children: <Row gutter={[16, 0]}>
        {list('defaultNameserver', t('config.dnsBootstrapHint'))}
        {list('nameserver', t('config.dnsResolverHint'))}
        {list('fallback')}
        {list('proxyServerNameserver', t('config.dnsProxyResolverHint'))}
        {list('directNameserver')}
        {boolean('directNameserverFollowPolicy')}
      </Row> },
      { key: 'policies', label: t('config.dnsPolicies'), forceRender: true, children: <>
        <Alert type="info" showIcon title={t('config.dnsPolicyHint')} style={{ marginBottom: 16 }} />
        <Row gutter={[16, 0]}>
          {field('nameserverPolicy', <DNSResolverPolicy />)}
          {field('proxyServerNameserverPolicy', <DNSResolverPolicy />)}
        </Row>
      </> },
      { key: 'fakeip', label: t('config.dnsFakeIP'), forceRender: true, children: <Row gutter={[16, 0]}>
        {field('fakeIPRange', <Input placeholder="198.18.0.1/16" />)}
        {field('fakeIPRange6', <Input placeholder="fdfe:dcba:9876::1/64" />)}
        {choice('fakeIPFilterMode', ['blacklist', 'whitelist', 'rule'])}
        {field('fakeIPTTL', <InputNumber min={0} max={4294967295} precision={0} style={{ width: '100%' }} />, { extra: t('config.dnsTTLHint') })}
        {list('fakeIPFilter', t('config.dnsFakeIPHint'), true)}
      </Row> },
      { key: 'fallback', label: t('config.dnsFallback'), forceRender: true, children: <Row gutter={[16, 0]}>
        {boolean('fallbackFilter.geoip')}
        {field('fallbackFilter.geoipCode', <Input placeholder="CN" maxLength={2} />)}
        {list('fallbackFilter.ipcidr')}
        {list('fallbackFilter.domain')}
        {list('fallbackFilter.geosite', t('config.dnsGeositeHint'))}
        {boolean('fallbackLazyQuery')}
      </Row> },
    ]} />
  </>;
}
