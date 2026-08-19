import { useMemo } from 'react'
import {
  TextField, Switch, FormControlLabel, MenuItem, Divider, Typography, Stack, FormControl, InputLabel, Select, RadioGroup, FormLabel, Radio, Box,
} from '@mui/material'
import type { ProtocolCapability, FieldCapability } from '@shared/api/capabilities'
import { useI18n } from '@shared/i18n'

function FieldControl({
  field, value, onChange,
}: {
  field: FieldCapability
  value: any
  onChange: (v: any) => void
}) {
  if (field.type === 'boolean') {
    return (
      <FormControlLabel
        control={<Switch checked={!!value} onChange={(e) => onChange(e.target.checked)} />}
        label={field.label}
      />
    )
  }
  if (field.type === 'integer') {
    return (
      <TextField
        fullWidth type="number" label={field.label} helperText={field.description}
        value={value ?? ''} onChange={(e) => onChange(e.target.value === '' ? undefined : Number(e.target.value))}
      />
    )
  }
  if (field.options?.length) {
    return (
      <TextField select fullWidth label={field.label} helperText={field.description} value={value ?? ''} onChange={(e) => onChange(e.target.value)}>
        {field.options.map((o) => <MenuItem key={o} value={o}>{o}</MenuItem>)}
      </TextField>
    )
  }
  return (
    <TextField
      fullWidth
      multiline={field.type === 'text'}
      minRows={field.type === 'text' ? 2 : undefined}
      type={field.type === 'secret' ? 'password' : 'text'}
      label={field.label}
      helperText={field.description}
      value={value ?? ''}
      onChange={(e) => onChange(e.target.value)}
    />
  )
}

type Props = {
  protocol?: string
  capability?: ProtocolCapability
  showAdvanced?: boolean
  values: Record<string, any>
  onChange: (key: string, value: any) => void
}

export default function CapabilityFormFields({ protocol, capability, showAdvanced = true, values, onChange }: Props) {
  const { t } = useI18n()
  const transportComps = useMemo(() => (capability?.components || []).filter((c) => c.group === 'transport'), [capability])
  const securityComps = useMemo(() => (capability?.components || []).filter((c) => c.group === 'security'), [capability])

  if (!capability) return null

  const transport = values.transport_layer || transportComps[0]?.kind || 'raw'
  const security = values.security_layer || securityComps[0]?.kind || 'none'
  const selectedTransport = transportComps.find((c) => c.kind === transport)
  const selectedSecurity = securityComps.find((c) => c.kind === security)

  const renderFields = (fields?: FieldCapability[]) =>
    (fields || [])
      .filter((f) => showAdvanced || !f.advanced)
      .map((f) => (
        <FieldControl key={f.path} field={f} value={values[f.path]} onChange={(v) => onChange(f.path, v)} />
      ))

  return (
    <Stack spacing={2}>
      {transportComps.length > 0 && (
        <>
          <Divider>{t('listeners.sectionTransport') !== 'listeners.sectionTransport' ? t('listeners.sectionTransport') : 'Transport'}</Divider>
          <FormControl fullWidth>
            <InputLabel>Transport</InputLabel>
            <Select
              label="Transport"
              value={transport}
              onChange={(e) => onChange('transport_layer', e.target.value)}
            >
              {transportComps.map((c) => <MenuItem key={c.kind} value={c.kind}>{c.label}</MenuItem>)}
            </Select>
          </FormControl>
          {renderFields(selectedTransport?.fields)}
        </>
      )}
      {securityComps.length > 0 && (
        <>
          <Divider>{t('listeners.sectionTLS') !== 'listeners.sectionTLS' ? t('listeners.sectionTLS') : 'Security'}</Divider>
          <FormControl>
            <FormLabel>Security</FormLabel>
            <RadioGroup row value={security} onChange={(e) => onChange('security_layer', e.target.value)}>
              {securityComps.map((c) => (
                <FormControlLabel key={c.kind} value={c.kind} control={<Radio />} label={c.label} />
              ))}
            </RadioGroup>
          </FormControl>
          {renderFields(selectedSecurity?.fields)}
        </>
      )}
      {!!capability.fields?.length && (
        <>
          <Divider>{t('listeners.protocol')}</Divider>
          {renderFields(capability.fields)}
        </>
      )}
      {protocol === 'vless' && (
        <TextField
          fullWidth label="Flow" value={values.flow || ''}
          onChange={(e) => onChange('flow', e.target.value)}
          helperText="xtls-rprx-vision"
        />
      )}
    </Stack>
  )
}
