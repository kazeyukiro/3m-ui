import { useState } from 'react'
import { useNavigate, useLocation } from 'react-router-dom'
import { login } from '@shared/api/auth'
import { useI18n } from '@shared/i18n'

export default function Login() {
  const { t } = useI18n()
  const navigate = useNavigate()
  const location = useLocation()
  const [u, setU] = useState('admin')
  const [p, setP] = useState('')
  const [err, setErr] = useState('')
  const from = (location.state as any)?.from?.pathname || '/'
  return (
    <div className="login-wrap">
      <form className="login-card" onSubmit={async (e) => {
        e.preventDefault()
        try { const data = await login({ username: u, password: p }); navigate(data.must_change_password ? '/change-password' : from, { replace: true }) } catch (ex: any) { setErr(ex.message) }
      }}>
        <h2>{t('login.title')}</h2>
        <p className="muted">{t('login.subtitle')}</p>
        {err && <div className="alert">{err}</div>}
        <label className="label">{t('login.username')}<input className="input" value={u} onChange={(e)=>setU(e.target.value)} /></label>
        <label className="label">{t('login.password')}<input className="input" type="password" value={p} onChange={(e)=>setP(e.target.value)} /></label>
        <button className="btn primary" type="submit" style={{width:'100%'}}>{t('login.button')}</button>
      </form>
    </div>
  )
}
