import { Stack, Typography, TextField, MenuItem, Switch, FormControlLabel, Divider, Alert, ToggleButton, ToggleButtonGroup } from '@mui/material'
import { visibleSections, FormField } from '@shared/logic/listenerFormSchema'
import { useI18n } from '@shared/i18n'

type Props = {
  protocol?: string
  values: Record<string, any>
  onChange: (key: string, value: any) => void
}

function Field({ field, value, onChange, t }: { field: FormField; value: any; onChange: (v: any) => void; t: (k: string) => string }) {
  const label = field.labelKey ? (t(field.labelKey) !== field.labelKey ? t(field.labelKey) : (field.label || field.name)) : (field.label || field.name)
  const hint = field.hintKey ? (t(field.hintKey) !== field.hintKey ? t(field.hintKey) : undefined) : undefined

  if (field.type === 'boolean') {
    return <FormControlLabel control={<Switch checked={!!value} onChange={(e) => onChange(e.target.checked)} />} label={label} />
  }
  if (field.type === 'radio' && field.options) {
    return (
      <Stack spacing={0.5}>
        <Typography variant="body2" color="text.secondary">{label}</Typography>
        <ToggleButtonGroup exclusive size="small" value={value ?? field.default ?? field.options[0]} onChange={(_, v) => v != null && onChange(v)}>
          {field.options.map((o, i) => (
            <ToggleButton key={o} value={o}>{field.optionLabels?.[i] || o || '(none)'}</ToggleButton>
          ))}
        </ToggleButtonGroup>
      </Stack>
    )
  }
  if (field.type === 'select' || (field.options && field.type !== 'tags')) {
    return (
      <TextField select fullWidth label={label} helperText={hint} required={field.required} value={value ?? ''} onChange={(e) => onChange(e.target.value)}>
        {(field.options || []).map((o, i) => (
          <MenuItem key={String(o)} value={o}>{field.optionLabels?.[i] || o || '(none)'}</MenuItem>
        ))}
      </TextField>
    )
  }
  if (field.type === 'tags') {
    const str = Array.isArray(value) ? value.join(',') : (value ?? '')
    return (
      <TextField fullWidth label={label} helperText={hint || 'Comma-separated'} value={str}
        onChange={(e) => onChange(e.target.value.split(',').map((s) => s.trim()).filter(Boolean))} />
    )
  }
  return (
    <TextField
      fullWidth
      label={label}
      helperText={hint}
      required={field.required}
      type={field.type === 'secret' ? 'password' : field.type === 'integer' ? 'number' : 'text'}
      multiline={field.type === 'text'}
      minRows={field.type === 'text' ? 2 : undefined}
      value={value ?? ''}
      onChange={(e) => onChange(field.type === 'integer' ? (e.target.value === '' ? undefined : Number(e.target.value)) : e.target.value)}
    />
  )
}

export default function ListenerConfigFields({ protocol, values, onChange }: Props) {
  const { t } = useI18n()
  if (!protocol) return <Alert severity="info">{t('listeners.selectProtocolFirst')}</Alert>
  const sections = visibleSections(protocol, values)
  return (
    <Stack spacing={2}>
      <Alert severity="info">{t('listeners.usersHint')}</Alert>
      {sections.map((sec) => (
        <Stack key={sec.id} spacing={1.5}>
          <Divider>{t(sec.titleKey) !== sec.titleKey ? t(sec.titleKey) : sec.id}</Divider>
          {sec.fields.map((f) => (
            <Field key={f.name} field={f} value={values[f.name]} onChange={(v) => onChange(f.name, v)} t={t} />
          ))}
        </Stack>
      ))}
    </Stack>
  )
}
