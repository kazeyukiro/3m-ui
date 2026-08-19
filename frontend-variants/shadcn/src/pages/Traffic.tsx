import { useEffect, useState } from 'react'
import { fetchTrafficStatus, fetchTrafficUsers, fetchConnections } from '@shared/api/traffic'
import { formatBytes } from '@shared/utils/format'
import { useI18n } from '@shared/i18n'

export default function TrafficPage() {
  const { t } = useI18n()
  const [status, setStatus] = useState<any>()
  const [users, setUsers] = useState<any[]>([])
  const [conns, setConns] = useState<any[]>([])
  const [error, setError] = useState('')
  const load = async () => {
    try {
      const [s,u,c] = await Promise.all([fetchTrafficStatus(), fetchTrafficUsers(), fetchConnections()])
      setStatus(s); setUsers(u||[]); setConns(c||[])
    } catch (e: any) { setError(e.message) }
  }
  useEffect(() => { load(); const id = setInterval(load, 5000); return () => clearInterval(id) }, [])
  return (
    <div>
      <h2>{t('traffic.title')}</h2>
      {error && <div className="alert">{error}</div>}
      <div className="grid grid-5">
        {[
          ['Upload', formatBytes(status?.upload_bytes||0)],
          ['Download', formatBytes(status?.download_bytes||0)],
          ['↑', formatBytes(status?.upload_rate||0)+'/s'],
          ['↓', formatBytes(status?.download_rate||0)+'/s'],
          ['Conn', status?.connections ?? conns.length],
        ].map(([a,b]) => <div key={String(a)} className="card"><div className="muted">{a}</div><strong>{b}</strong></div>)}
      </div>
      <div className="card">
        <table className="table">
          <thead><tr><th>User</th><th>Used</th><th>Limit</th><th>Status</th></tr></thead>
          <tbody>
            {users.map((u)=>(
              <tr key={u.user_id}>
                <td>{u.username}</td>
                <td>{formatBytes(u.traffic_used||0)}</td>
                <td>{u.traffic_limit?formatBytes(u.traffic_limit):'∞'}</td>
                <td><span className={`badge ${u.online?'green':''}`}>{u.online?'Online':'Offline'}</span></td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </div>
  )
}
