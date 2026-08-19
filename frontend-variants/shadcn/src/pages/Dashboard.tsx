import { useEffect, useState } from 'react'
import { Play, Square, RotateCcw } from 'lucide-react'
import { fetchDashboard, startMihomo, stopMihomo, restartMihomo } from '@shared/api/system'
import { formatBytes } from '@shared/utils/format'
import { useI18n } from '@shared/i18n'

export default function Dashboard() {
  const { t } = useI18n()
  const [data, setData] = useState<any>()
  const [error, setError] = useState('')
  const load = async () => { try { setData(await fetchDashboard()) } catch (e: any) { setError(e.message) } }
  useEffect(() => { load(); const id = setInterval(load, 5000); return () => clearInterval(id) }, [])
  const act = async (fn: () => Promise<any>) => { try { await fn(); await load() } catch (e: any) { setError(e.message) } }
  const sys = data?.system || {}
  const m = data?.mihomo || {}
  return (
    <div>
      <h2>{t('dashboard.title')}</h2>
      {error && <div className="alert">{error}</div>}
      <div className="grid grid-3">
        <div className="card">
          <strong>{t('dashboard.mihomoStatus')}</strong>
          <p className="muted">{m.running?'Running':'Stopped'} · {m.version||'—'} · PID {m.pid||'—'}</p>
          <div className="row" style={{justifyContent:'flex-start'}}>
            <button className="btn primary" onClick={()=>act(startMihomo)}><Play size={16}/>{t('dashboard.start')}</button>
            <button className="btn" onClick={()=>act(restartMihomo)}><RotateCcw size={16}/>{t('dashboard.restart')}</button>
            <button className="btn danger" onClick={()=>act(stopMihomo)}><Square size={16}/>{t('dashboard.stop')}</button>
          </div>
        </div>
        <div className="card">
          <strong>{t('dashboard.listeners')}</strong>
          <p>{data?.listeners?.total||0} total · {data?.listeners?.enabled||0} on · {data?.listeners?.disabled||0} off</p>
        </div>
        <div className="card">
          <strong>{t('dashboard.traffic')}</strong>
          <p>↑ {formatBytes(data?.traffic?.uploadRate||0)}/s · ↓ {formatBytes(data?.traffic?.downloadRate||0)}/s</p>
          <p className="muted">{t('dashboard.onlineUsers')}: {data?.traffic?.onlineUsers||0}</p>
        </div>
      </div>
      <div className="grid grid-3">
        <div className="card"><div className="muted">{t('dashboard.cpu')}</div><strong>{sys.cpu?.percent||0}%</strong></div>
        <div className="card"><div className="muted">{t('dashboard.memory')}</div><strong>{sys.memory?.percent||0}%</strong><div className="muted">{formatBytes(sys.memory?.used||0)} / {formatBytes(sys.memory?.total||0)}</div></div>
        <div className="card"><div className="muted">{t('dashboard.disk')}</div><strong>{sys.disk?.percent||0}%</strong></div>
      </div>
    </div>
  )
}
