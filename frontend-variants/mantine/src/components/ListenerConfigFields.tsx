import { Stack, Text, TextInput, PasswordInput, NumberInput, Select, Switch, Divider, Alert, SegmentedControl, Textarea, TagsInput } from '@mantine/core'
import { visibleSections, FormField } from '@shared/logic/listenerFormSchema'
import { useI18n } from '@shared/i18n'

type Props = { protocol?: string; values: Record<string, any>; onChange: (key: string, value: any) => void }

function Field({ field, value, onChange, t }: { field: FormField; value: any; onChange: (v: any) => void; t: (k: string) => string }) {
  const label = field.labelKey ? (t(field.labelKey) !== field.labelKey ? t(field.labelKey) : (field.label || field.name)) : (field.label || field.name)
  const description = field.hintKey && t(field.hintKey) !== field.hintKey ? t(field.hintKey) : undefined
  if (field.type === 'boolean') return <Switch label={label} checked={!!value} onChange={(e) => onChange(e.currentTarget.checked)} />
  if (field.type === 'radio' && field.options) {
    return (
      <div>
        <Text size="sm" mb={4}>{label}</Text>
        <SegmentedControl
          fullWidth
          value={String(value ?? field.default ?? field.options[0])}
          onChange={onChange}
          data={field.options.map((o, i) => ({ value: o, label: field.optionLabels?.[i] || o || '(none)' }))}
        />
      </div>
    )
  }
  if (field.type === 'select' || (field.options && field.type !== 'tags')) {
    return (
      <Select
        label={label} description={description} required={field.required}
        data={(field.options || []).map((o, i) => ({ value: o, label: field.optionLabels?.[i] || o || '(none)' }))}
        value={value ?? null} onChange={(v) => onChange(v)}
      />
    )
  }
  if (field.type === 'tags') {
    const data = Array.isArray(value) ? value : (value ? String(value).split(',') : [])
    return <TagsInput label={label} description={description} value={data} onChange={onChange} />
  }
  if (field.type === 'integer') {
    return <NumberInput label={label} description={description} required={field.required} value={value ?? ''} onChange={(v) => onChange(v === '' ? undefined : Number(v))} />
  }
  if (field.type === 'secret') {
    return <PasswordInput label={label} description={description} required={field.required} value={value ?? ''} onChange={(e) => onChange(e.currentTarget.value)} />
  }
  if (field.type === 'text') {
    return <Textarea label={label} description={description} required={field.required} value={value ?? ''} onChange={(e) => onChange(e.currentTarget.value)} minRows={2} />
  }
  return <TextInput label={label} description={description} required={field.required} value={value ?? ''} onChange={(e) => onChange(e.currentTarget.value)} />
}

export default function ListenerConfigFields({ protocol, values, onChange }: Props) {
  const { t } = useI18n()
  if (!protocol) return <Alert color="blue">{t('listeners.selectProtocolFirst')}</Alert>
  const sections = visibleSections(protocol, values)
  return (
    <Stack>
      <Alert color="blue">{t('listeners.usersHint')}</Alert>
      {sections.map((sec) => (
        <Stack key={sec.id} gap="sm">
          <Divider label={t(sec.titleKey) !== sec.titleKey ? t(sec.titleKey) : sec.id} />
          {sec.fields.map((f) => (
            <Field key={f.name} field={f} value={values[f.name]} onChange={(v) => onChange(f.name, v)} t={t} />
          ))}
        </Stack>
      ))}
    </Stack>
  )
}
