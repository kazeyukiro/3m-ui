import { useEffect, useState } from 'react'
import { Plus, Trash2, Pencil, Copy, Link, RefreshCw, Share2 } from 'lucide-react'
import { fetchUsers, createUser, updateUser, deleteUser, resetUserTraffic, fetchUserNodes, bindUserNodes, fetchUserSubscription, ProxyUser } from '@shared/api/users'
import { fetchListeners, Listener } from '@shared/api/nodes'
import { formatBytes } from '@shared/utils/format'
import { useI18n } from '@shared/i18n'

export default function Users() {
  const { t } = useI18n()
  const [rows, setRows] = useState<ProxyUser[]>([])
  const [error, setError] = useState('')
  const [ok, setOk] = useState('')
  const [edit, setEdit] = useState<any>(null)
  const [bindUser, setBindUser] = useState<ProxyUser|null>(null)
  const [allNodes, setAllNodes] = useState<Listener[]>([])
  const [selectedNodeIds, setSelectedNodeIds] = useState<number[]>([])
  const [subInfo, setSubInfo] = useState<any>(null)

  const load = async () => { try { setRows(await fetchUsers()) } catch (e: any) { setError(e.message) } }
  useEffect(() => { load() }, [])

  const save = async () => {
    try {
      const payload: any = {
        username: edit.username, enabled: !!edit.enabled, remark: edit.remark,
        traffic_limit: edit.traffic_limit_gb ? Math.round(Number(edit.traffic_limit_gb)*1024*1024*1024) : 0,
        ip_limit: edit.ip_limit ? Number(edit.ip_limit) : 0,
      }
      if (edit.password) payload.password = edit.password
      if (edit.id) await updateUser(edit.id, payload); else await createUser(payload)
      setEdit(null); setOk('Saved'); load()
    } catch (e: any) { setError(e.message) }
  }

  return (
    <div>
      <div className="row"><h2>{t('users.title')}</h2><button className="btn primary" onClick={()=>setEdit({enabled:true})}><Plus size={16}/>{t('common.create')}</button></div>
      {error && <div className="alert">{error}</div>}
      {ok && <div className="alert ok">{ok}</div>}
      <div className="card">
        <table className="table">
          <thead><tr><th>{t('users.username')}</th><th>{t('users.traffic')}</th><th>{t('users.status')}</th><th>{t('users.remark')}</th><th></th></tr></thead>
          <tbody>
            {rows.map((u)=>(
              <tr key={u.id}>
                <td><strong>{u.username}</strong></td>
                <td>{formatBytes(u.traffic_used||0)}{u.traffic_limit?` / ${formatBytes(u.traffic_limit)}`:''}</td>
                <td><span className={`badge ${u.enabled?'green':''}`}>{u.enabled?t('common.enabled'):t('common.disabled')}</span>{u.blocked&&<span className="badge red">Blocked</span>}</td>
                <td>{u.remark||'—'}</td>
                <td style={{textAlign:'right'}}>
                  <button className="icon-btn" onClick={async ()=>{ setBindUser(u); const [nodes,bound]=await Promise.all([fetchListeners(),fetchUserNodes(u.id)]); setAllNodes(nodes); setSelectedNodeIds((bound||[]).map((n:any)=>n.id)) }}><Share2 size={16}/></button>
                  <button className="icon-btn" onClick={()=>fetchUserSubscription(u.id).then(setSubInfo).catch((e)=>setError(e.message))}><Link size={16}/></button>
                  <button className="icon-btn" onClick={()=>resetUserTraffic(u.id).then(()=>{setOk('Reset');load()}).catch((e)=>setError(e.message))}><RefreshCw size={16}/></button>
                  {u.sub_token && <button className="icon-btn" onClick={()=>navigator.clipboard.writeText(u.sub_token||'')}><Copy size={16}/></button>}
                  <button className="icon-btn" onClick={()=>setEdit({...u, traffic_limit_gb: u.traffic_limit?(u.traffic_limit/1024/1024/1024).toFixed(2):''})}><Pencil size={16}/></button>
                  <button className="icon-btn" onClick={()=>deleteUser(u.id).then(load).catch((e)=>setError(e.message))}><Trash2 size={16}/></button>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
      {edit && (
        <div className="overlay" onClick={()=>setEdit(null)}>
          <div className="modal" onClick={(e)=>e.stopPropagation()}>
            <h3>{edit.id?t('common.edit'):t('common.create')}</h3>
            <label className="label">{t('users.username')}<input className="input" value={edit.username||''} onChange={(e)=>setEdit({...edit,username:e.target.value})} /></label>
            <label className="label">{t('users.password')}<input className="input" type="password" value={edit.password||''} onChange={(e)=>setEdit({...edit,password:e.target.value})} /></label>
            <label className="label">{t('users.remark')}<input className="input" value={edit.remark||''} onChange={(e)=>setEdit({...edit,remark:e.target.value})} /></label>
            <label className="label">Traffic limit (GB)<input className="input" value={edit.traffic_limit_gb??''} onChange={(e)=>setEdit({...edit,traffic_limit_gb:e.target.value})} /></label>
            <label className="label"><input type="checkbox" checked={!!edit.enabled} onChange={(e)=>setEdit({...edit,enabled:e.target.checked})} /> {t('common.enabled')}</label>
            <button className="btn primary" onClick={save}>{t('common.save')}</button>
          </div>
        </div>
      )}
      {bindUser && (
        <div className="overlay" onClick={()=>setBindUser(null)}>
          <div className="modal" onClick={(e)=>e.stopPropagation()}>
            <h3>Bind — {bindUser.username}</h3>
            {allNodes.map((n)=>(
              <label key={n.id} className="label"><input type="checkbox" checked={selectedNodeIds.includes(n.id)} onChange={()=>setSelectedNodeIds((ids)=>ids.includes(n.id)?ids.filter(x=>x!==n.id):[...ids,n.id])} /> {n.name} ({n.protocol}:{n.port})</label>
            ))}
            <button className="btn primary" onClick={async ()=>{ try{await bindUserNodes(bindUser.id,selectedNodeIds);setBindUser(null);setOk('Bound')}catch(e:any){setError(e.message)}}}>{t('common.save')}</button>
          </div>
        </div>
      )}
      {subInfo && (
        <div className="overlay" onClick={()=>setSubInfo(null)}>
          <div className="modal" onClick={(e)=>e.stopPropagation()}>
            <h3>Subscription</h3>
            <label className="label">URL<input className="input" readOnly value={subInfo.url||''} /></label>
            <label className="label">Token<input className="input" readOnly value={subInfo.token||''} /></label>
            <button className="btn" onClick={()=>navigator.clipboard.writeText(subInfo.url||'')}>{t('common.copy')}</button>
          </div>
        </div>
      )}
    </div>
  )
}
