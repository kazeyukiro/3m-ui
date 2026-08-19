import { useEffect, useState } from 'react'
import { Plus, Trash2, HeartPulse } from 'lucide-react'
import { fetchCluster, createClusterNode, updateClusterNode, deleteClusterNode, healthClusterNode, RemoteServer } from '@shared/api/cluster'
import { useI18n } from '@shared/i18n'

export default function ClusterPage() {
  const { t } = useI18n()
  const [rows, setRows] = useState<RemoteServer[]>([])
  const [error, setError] = useState('')
  const [edit, setEdit] = useState<any>(null)
  const load = async () => { try { setRows(await fetchCluster()) } catch (e: any) { setError(e.message) } }
  useEffect(() => { load() }, [])
  return (
    <div>
      <div className="row"><h2>{t('cluster.title')}</h2><button className="btn primary" onClick={()=>setEdit({enabled:true})}><Plus size={16}/>{t('common.create')}</button></div>
      {error && <div className="alert">{error}</div>}
      <div className="card">
        <table className="table">
          <thead><tr><th>{t('cluster.name')}</th><th>{t('cluster.baseUrl')}</th><th>{t('cluster.status')}</th><th></th></tr></thead>
          <tbody>
            {rows.map((r)=>(
              <tr key={r.id}>
                <td>{r.name}</td><td>{r.base_url}</td><td><span className="badge">{r.last_status||'—'}</span></td>
                <td style={{textAlign:'right'}}>
                  <button className="icon-btn" onClick={()=>healthClusterNode(r.id).then(load).catch((e)=>setError(e.message))}><HeartPulse size={16}/></button>
                  <button className="icon-btn" onClick={()=>deleteClusterNode(r.id).then(load).catch((e)=>setError(e.message))}><Trash2 size={16}/></button>
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
            <label className="label">{t('cluster.name')}<input className="input" value={edit.name||''} onChange={(e)=>setEdit({...edit,name:e.target.value})} /></label>
            <label className="label">{t('cluster.baseUrl')}<input className="input" value={edit.base_url||''} onChange={(e)=>setEdit({...edit,base_url:e.target.value})} /></label>
            <label className="label">{t('cluster.apiToken')}<input className="input" type="password" value={edit.api_token||''} onChange={(e)=>setEdit({...edit,api_token:e.target.value})} /></label>
            <button className="btn primary" onClick={async ()=>{ try{ if(edit.id) await updateClusterNode(edit.id,edit); else await createClusterNode(edit); setEdit(null); load() }catch(e:any){setError(e.message)}}}>{t('common.save')}</button>
          </div>
        </div>
      )}
    </div>
  )
}
