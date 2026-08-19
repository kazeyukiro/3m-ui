import { useMemo } from 'react'
import type { ProtocolCapability, FieldCapability } from '@shared/api/capabilities'
import { useI18n } from '@shared/i18n'

function FieldControl({ field, value, onChange }: { field: FieldCapability; value: any; onChange: (v: any) => void }) {
  if (field.type === 'boolean') {
    return (
      <label className="label">
        <input type="checkbox" checked={!!value} onChange={(e) => onChange(e.target.checked)} /> {field.label}
        {field.description && <span className="muted">{field.description}</span>}
      </label>
    )
  }
  if (field.options?.length) {
    return (
      <label className="label">{field.label}
        <select className="select" value={value ?? ''} onChange={(e) => onChange(e.target.value)}>
          <option value="">(none)</option>
          {field.options.map((o) => <option key={o} value={o}>{o}</option>)}
        </select>
      </label>
    )
  }
  if (field.type === 'text' || field.type === 'string-list') {
    return (
      <label className="label">{field.label}
        <textarea className="textarea" value={Array.isArray(value) ? value.join(',') : (value ?? '')} onChange={(e) => onChange(e.target.value)} />
      </label>
    )
  }
  return (
    <label className="label">{field.label}
      <input
        className="input"
        type={field.type === 'secret' ? 'password' : field.type === 'integer' ? 'number' : 'text'}
        value={value ?? ''}
        onChange={(e) => onChange(field.type === 'integer' ? (e.target.value === '' ? undefined : Number(e.target.value)) : e.target.value)}
      />
      {field.description && <span className="muted">{field.description}</span>}
    </label>
  )
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
    <div style={{ marginTop: 16 }}>
      <h4 style={{ margin: '8px 0', borderBottom: '1px solid #e4e4e7', paddingBottom: 6 }}>
        {t('common.advanced') !== 'common.advanced' ? t('common.advanced') : 'Advanced (capability)'}
      </h4>
      {protocolFields.map((f) => (
        <FieldControl key={f.path} field={f} value={values[f.path]} onChange={(v) => onChange(f.path, v)} />
      ))}
    </div>
  )
}
