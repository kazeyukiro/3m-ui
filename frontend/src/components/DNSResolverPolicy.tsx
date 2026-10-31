import { Button, Card, Input, Select, Space, message } from 'antd';
import { IconAddField, IconDelete } from '../icons';
import { useI18n } from '../i18n';

type ResolverPolicy = Record<string, string | string[]>;

export default function DNSResolverPolicy({ value = {}, onChange }: { value?: ResolverPolicy; onChange?: (value: ResolverPolicy) => void }) {
  const { t } = useI18n();
  const entries = Object.entries(value);
  const update = (index: number, domain: string, resolvers: string | string[]) => {
    if (domain !== entries[index][0] && Object.hasOwn(value, domain)) {
      message.error(t('config.dnsPolicyDuplicateDomain'));
      return;
    }
    onChange?.(Object.fromEntries(entries.map((entry, i) => i === index ? [domain, resolvers] : entry)));
  };
  return <Space orientation="vertical" style={{ width: '100%' }}>
    {entries.map(([domain, resolvers], index) => <Card size="small" key={index}>
      <Space wrap style={{ width: '100%' }}>
        <Input aria-label={t('config.dnsPolicyDomain')} placeholder={t('config.dnsPolicyDomain')} value={domain} onChange={e => update(index, e.target.value, resolvers)} />
        <Button aria-label={t('common.delete')} icon={<IconDelete />} onClick={() => onChange?.(Object.fromEntries(entries.filter((_, i) => i !== index)))} />
      </Space>
      <Select mode="tags" style={{ width: '100%', marginTop: 8 }} value={Array.isArray(resolvers) ? resolvers : resolvers ? [resolvers] : []} onChange={next => update(index, domain, next)} />
    </Card>)}
    <Button icon={<IconAddField />} onClick={() => onChange?.({ ...value, '': [] })} disabled={Object.hasOwn(value, '')}>{t('config.dnsPolicyAdd')}</Button>
  </Space>;
}
