import { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { changePassword } from '@shared/api/auth'

export default function ChangePassword() {
  const navigate = useNavigate()
  const [cur, setCur] = useState('')
  const [next, setNext] = useState('')
  const [confirm, setConfirm] = useState('')
  const [err, setErr] = useState('')
  return (
    <div className="login-wrap">
      <div className="login-card">
        <h2>Change Password</h2>
        {err && <div className="alert">{err}</div>}
        <label className="label">Current<input className="input" type="password" value={cur} onChange={(e)=>setCur(e.target.value)} /></label>
        <label className="label">New<input className="input" type="password" value={next} onChange={(e)=>setNext(e.target.value)} /></label>
        <label className="label">Confirm<input className="input" type="password" value={confirm} onChange={(e)=>setConfirm(e.target.value)} /></label>
        <button className="btn primary" style={{width:'100%'}} onClick={async () => {
          if (next !== confirm) { setErr('Passwords do not match'); return }
          try { await changePassword(cur, next); navigate('/', { replace: true }) } catch (e: any) { setErr(e.message) }
        }}>Save</button>
      </div>
    </div>
  )
}
