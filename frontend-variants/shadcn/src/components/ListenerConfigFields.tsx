import { visibleSections, FormField } from '@shared/logic/listenerFormSchema'
import { useI18n } from '@shared/i18n'

type Props = { protocol?: string; values: Record<string, any>; onChange: (key: string, value: any) => void }

function Field({ field, value, onChange, t }: { field: FormField; value: any; onChange: (v: any) => void; t: (k: string) => string }) {
  const label = field.labelKey ? (t(field.labelKey) !== field.labelKey ? t(field.labelKey) : (field.label || field.name)) : (field.label || field.name)
  const hint = field.hintKey && t(field.hintKey) !== field.hintKey ? t(field.hintKey) : undefined
  if (field.type === 'boolean') {
    return <label className="label"><input type="checkbox" checked={!!value} onChange={(e) => onChange(e.target.checked)} /> {label}</label>
  }
  if ((field.type === 'radio' || field.type === 'select') && field.options) {
    return (
      <label className="label">{label}
        <select className="select" value={value ?? field.default ?? ''} onChange={(e) => onChange(e.target.value)}>
          {field.options.map((o, i) => <option key={String(o)} value={o}>{field.optionLabels?.[i] || o || '(none)'}</option>)}
        </select>
        {hint && <span className="muted">{hint}</span>}
      </label>
    )
  }
  if (field.type === 'tags') {
    const str = Array.isArray(value) ? value.join(',') : (value ?? '')
    return (
      <label className="label">{label}
        <input className="input" value={str} onChange={(e) => onChange(e.target.value.split(',').map((s) => s.trim()).filter(Boolean))} />
        <span className="muted">{hint || 'Comma-separated'}</span>
      </label>
    )
  }
  if (field.type === 'text') {
    return <label className="label">{label}<textarea className="textarea" value={value ?? ''} onChange={(e) => onChange(e.target.value)} />{hint && <span className="muted">{hint}</span>}</label>
  }
  return (
    <label className="label">{label}
      <input
        className="input"
        type={field.type === 'secret' ? 'password' : field.type === 'integer' ? 'number' : 'text'}
        value={value ?? ''}
        required={field.required}
        onChange={(e) => onChange(field.type === 'integer' ? (e.target.value === '' ? undefined : Number(e.target.value)) : e.target.value)}
      />
      {hint && <span className="muted">{hint}</span>}
    </label>
  )
}

export default function ListenerConfigFields({ protocol, values, onChange }: Props) {
  const { t } = useI18n()
  if (!protocol) return <div className="alert ok">{t('listeners.selectProtocolFirst')}</div>
  const sections = visibleSections(protocol, values)
  return (
    <div>
      <div className="alert ok" style={{ background: '#eff6ff', borderColor: '#bfdbfe', color: '#1e40af' }}>{t('listeners.usersHint')}</div>
      {sections.map((sec) => (
        <div key={sec.id} style={{ marginTop: 16 }}>
          <h4 style={{ margin: '8px 0', borderBottom: '1px solid #e4e4e7', paddingBottom: 6 }}>
            {t(sec.titleKey) !== sec.titleKey ? t(sec.titleKey) : sec.id}
          </h4>
          {sec.fields.map((f) => (
            <Field key={f.name} field={f} value={values[f.name]} onChange={(v) => onChange(f.name, v)} t={t} />
          ))}
        </div>
      ))}
    </div>
  )
}
