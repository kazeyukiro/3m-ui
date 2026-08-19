import { useMemo } from 'react'
import { Stack, TextInput, PasswordInput, NumberInput, Select, Switch, Textarea, Divider, Text } from '@mantine/core'
import type { ProtocolCapability, FieldCapability } from '@shared/api/capabilities'
import { useI18n } from '@shared/i18n'

function FieldControl({ field, value, onChange }: { field: FieldCapability; value: any; onChange: (v: any) => void }) {
  if (field.type === 'boolean') {
    return <Switch label={field.label} checked={!!value} onChange={(e) => onChange(e.currentTarget.checked)} description={field.description} />
  }
  if (field.type === 'integer') {
    return <NumberInput label={field.label} description={field.description} value={value ?? ''} onChange={(v) => onChange(v === '' ? undefined : Number(v))} />
  }
  if (field.options?.length) {
    return (
      <Select
        label={field.label}
        description={field.description}
        data={field.options.map((o) => ({ value: o, label: o }))}
        value={value ?? null}
        onChange={(v) => onChange(v)}
      />
    )
  }
  if (field.type === 'secret') {
    return <PasswordInput label={field.label} description={field.description} value={value ?? ''} onChange={(e) => onChange(e.currentTarget.value)} />
  }
  if (field.type === 'text' || field.type === 'string-list') {
    return <Textarea label={field.label} description={field.description} value={Array.isArray(value) ? value.join(',') : (value ?? '')} onChange={(e) => onChange(e.currentTarget.value)} minRows={2} />
  }
  return <TextInput label={field.label} description={field.description} value={value ?? ''} onChange={(e) => onChange(e.currentTarget.value)} />
}

type Props = {
  protocol?: string
  capability?: ProtocolCapability
  showAdvanced?: boolean
  values: Record<string, any>
  onChange: (key: string, value: any) => void
}

export default function CapabilityFormFields({ capability, showAdvanced = true, values, onChange }: Props) {
  const { t } = useI18n()
  const protocolFields = useMemo(
    () => (capability?.fields || []).filter((f) => showAdvanced || !f.advanced),
    [capability, showAdvanced],
  )
  if (!capability || protocolFields.length === 0) return null
  return (
    <Stack gap="sm">
      <Divider label={t('common.advanced') !== 'common.advanced' ? t('common.advanced') : 'Advanced (capability)'} />
      <Text size="sm" c="dimmed">{capability.label || capability.kind}</Text>
      {protocolFields.map((f) => (
        <FieldControl key={f.path} field={f} value={values[f.path]} onChange={(v) => onChange(f.path, v)} />
      ))}
    </Stack>
  )
}
