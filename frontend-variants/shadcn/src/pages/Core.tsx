import { useEffect, useState } from 'react'
import { Play, Square, RotateCcw } from 'lucide-react'
import { fetchDashboard, startMihomo, stopMihomo, restartMihomo } from '@shared/api/system'
import { useI18n } from '@shared/i18n'

export default function Core() {
  const { t } = useI18n()
  const [data, setData] = useState<any>()
  const [error, setError] = useState('')
  const load = async () => { try { setData(await fetchDashboard()) } catch (e: any) { setError(e.message) } }
  useEffect(() => { load() }, [])
  const m = data?.mihomo
  const act = async (fn: () => Promise<any>) => { try { await fn(); await load() } catch (e: any) { setError(e.message) } }
  return (
    <div>
      <h2>{t('core.title')}</h2>
      {error && <div className="alert">{error}</div>}
      <div className="card">
        <span className={`badge ${m?.running?'green':''}`}>{m?.running?t('core.running'):t('core.stopped')}</span>
        <p className="muted">{m?.version} · PID {m?.pid} · {m?.uptime}</p>
        <div className="row" style={{justifyContent:'flex-start'}}>
          <button className="btn primary" onClick={()=>act(startMihomo)}><Play size={16}/>{t('core.start')}</button>
          <button className="btn" onClick={()=>act(restartMihomo)}><RotateCcw size={16}/>{t('core.restart')}</button>
          <button className="btn danger" onClick={()=>act(stopMihomo)}><Square size={16}/>{t('core.stop')}</button>
        </div>
      </div>
    </div>
  )
}
