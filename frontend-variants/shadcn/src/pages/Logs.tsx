import { useEffect, useState } from 'react'
import dayjs from 'dayjs'
import { fetchLogs } from '@shared/api/system'
import { useI18n } from '@shared/i18n'

export default function Logs() {
  const { t } = useI18n()
  const [logs, setLogs] = useState<any[]>([])
  const [auto, setAuto] = useState(true)
  const load = async () => { try { const d = await fetchLogs(); setLogs(Array.isArray(d)?d:[]) } catch {} }
  useEffect(() => { load(); if (!auto) return; const id = setInterval(load, 3000); return () => clearInterval(id) }, [auto])
  return (
    <div>
      <h2>{t('logs.title')}</h2>
      <div className="row" style={{marginBottom:12}}>
        <button className="btn" onClick={load}>{t('common.refresh')}</button>
        <button className="btn" onClick={()=>setLogs([])}>{t('logs.clear')}</button>
        <button className={`btn ${auto?'primary':''}`} onClick={()=>setAuto(!auto)}>{t('logs.autoRefresh')}: {auto?t('common.enabled'):t('common.disabled')}</button>
      </div>
      <div className="card" style={{fontFamily:'ui-monospace,monospace',fontSize:13,maxHeight:520,overflow:'auto'}}>
        {logs.length===0 ? <p className="muted">{t('logs.empty')}</p> : logs.map((log,i)=>(
          <div key={i} style={{marginBottom:4}}>
            <span className="muted">[{dayjs(log.timestamp).format('YYYY-MM-DD HH:mm:ss')}] </span>
            <span className="badge">{log.level}</span> {log.payload}
          </div>
        ))}
      </div>
    </div>
  )
}
