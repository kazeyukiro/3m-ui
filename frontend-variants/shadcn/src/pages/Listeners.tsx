import { useEffect, useState } from 'react'
import { Plus, Trash2, Pencil, RefreshCw, Link, Copy, History, Files, Save } from 'lucide-react'
import { fetchListeners, createListener, updateListener, deleteListener, reloadListener, exportNodeURI, Listener } from '@shared/api/nodes'
import {
  listListenerTemplates, createListenerTemplate, deleteListenerTemplate, instantiateListenerTemplate,
  cloneListener, batchSetListenersEnabled, listListenerVersions, diffListenerVersion, rollbackListenerVersion,
  ListenerTemplate, ListenerVersion,
} from '@shared/api/listeners'
import { fetchCapabilities, protocolCapability, CapabilityManifest } from '@shared/api/capabilities'
import { configToFormValues, formValuesToConfig, protocolSupportsUDP } from '@shared/logic/listenerConfig'
import { capabilityFormToConfig } from '@shared/logic/capabilityForm'
import ListenerConfigFields from '../components/ListenerConfigFields'
import { useI18n } from '@shared/i18n'

const PROTOCOLS = ['shadowsocks','snell','vmess','vless','trojan','hysteria2','tuic','shadowquic','anytls','mieru','sudoku','trusttunnel']
const REALITY = new Set(['vmess','vless','trojan'])

export default function Listeners() {
  const { t } = useI18n()
  const [rows, setRows] = useState<Listener[]>([])
  const [templates, setTemplates] = useState<ListenerTemplate[]>([])
  const [error, setError] = useState('')
  const [ok, setOk] = useState('')
  const [edit, setEdit] = useState<any>(null)
  const [capabilities, setCapabilities] = useState<CapabilityManifest|null>(null)
  const [selected, setSelected] = useState<number[]>([])
  const [uris, setUris] = useState<string[]>([])
  const [uriOpen, setUriOpen] = useState(false)
  const [cloneSrc, setCloneSrc] = useState<Listener|null>(null)
  const [cloneForm, setCloneForm] = useState({ name: '', port: '' })
  const [tplSrc, setTplSrc] = useState<Listener|null>(null)
  const [tplName, setTplName] = useState('')
  const [instSrc, setInstSrc] = useState<ListenerTemplate|null>(null)
  const [instForm, setInstForm] = useState({ name: '', port: '' })
  const [versions, setVersions] = useState<ListenerVersion[]>([])
  const [verListener, setVerListener] = useState<Listener|null>(null)
  const [diffText, setDiffText] = useState('')
  const [diffOpen, setDiffOpen] = useState(false)

  const load = async () => { try { setRows(await fetchListeners()) } catch (e: any) { setError(e.message) } }
  const loadTemplates = async () => { try { setTemplates(await listListenerTemplates()) } catch {} }
  useEffect(() => { load(); loadTemplates(); fetchCapabilities().then(setCapabilities).catch(()=>null) }, [])
  const set = (k: string, v: any) => setEdit((p: any) => ({ ...p, [k]: v }))
  const cap = capabilities && edit?.protocol ? protocolCapability(capabilities, edit.protocol) : undefined

  const save = async () => {
    try {
      if (!edit?.name || !edit?.protocol || !String(edit?.port||'').trim()) { setError('Name/protocol/port required'); return }
      if (REALITY.has(edit.protocol) && (edit.security_layer==='reality'||!edit.security_layer) && (!edit.reality_dest||!edit.reality_private_key)) {
        setError('Reality Dest / Private Key required'); return
      }
      const previous = edit.id ? (()=>{ try{return JSON.parse(edit.config||'{}')}catch{return null}})() : null
      const config = { ...formValuesToConfig(edit.protocol, edit, previous), ...(cap ? capabilityFormToConfig(edit.protocol, edit, cap) : {}) }
      const payload: any = {
        name: edit.name, protocol: edit.protocol, port: String(edit.port), bind_address: edit.bind_address||'0.0.0.0',
        enabled: !!edit.enabled, udp: protocolSupportsUDP(edit.protocol)?!!edit.udp:false, config: JSON.stringify(config),
        public_host: edit.public_host||'', public_port: edit.public_port||'', access_sni: edit.access_sni||'',
        client_fingerprint: edit.client_fingerprint||'', access_alpn: edit.access_alpn||'',
      }
      if (edit.id) await updateListener(edit.id, payload); else await createListener(payload)
      setEdit(null); setOk('Saved'); load()
    } catch (e: any) { setError(e.message) }
  }

  return (
    <div>
      <div className="row">
        <h2>{t('listeners.title')}</h2>
        <div className="row" style={{justifyContent:'flex-start'}}>
          {selected.length>0 && <>
            <button className="btn" onClick={()=>batchSetListenersEnabled(selected,true).then(()=>{setSelected([]);load()}).catch((e)=>setError(e.message))}>Enable</button>
            <button className="btn" onClick={()=>batchSetListenersEnabled(selected,false).then(()=>{setSelected([]);load()}).catch((e)=>setError(e.message))}>Disable</button>
          </>}
          <button className="btn primary" onClick={()=>setEdit({ protocol:'vless', port:'443', bind_address:'0.0.0.0', enabled:true, transport_layer:'raw', security_layer:'reality', flow:'xtls-rprx-vision', client_fingerprint:'chrome' })}><Plus size={16}/>{t('common.create')}</button>
        </div>
      </div>
      {error && <div className="alert">{error}<button className="icon-btn" onClick={()=>setError('')}>×</button></div>}
      {ok && <div className="alert ok">{ok}</div>}
      <div className="card">
        <table className="table">
          <thead><tr><th></th><th>{t('listeners.name')}</th><th>{t('listeners.protocol')}</th><th>{t('listeners.port')}</th><th>{t('listeners.status')}</th><th></th></tr></thead>
          <tbody>
            {rows.map((l)=>(
              <tr key={l.id}>
                <td><input type="checkbox" checked={selected.includes(l.id)} onChange={()=>setSelected((s)=>s.includes(l.id)?s.filter(x=>x!==l.id):[...s,l.id])} /></td>
                <td>{l.name}</td><td>{l.protocol}</td>
                <td>{l.bind_address||'0.0.0.0'}:{l.port}</td>
                <td><span className={`badge ${l.enabled?'green':''}`}>{l.enabled?t('common.enabled'):t('common.disabled')}</span></td>
                <td style={{textAlign:'right'}}>
                  <button className="icon-btn" onClick={()=>exportNodeURI(l.id).then((d:any)=>{setUris(d?.uris||d?.links||(d?.uri?[d.uri]:[]));setUriOpen(true)}).catch((e)=>setError(e.message))}><Link size={16}/></button>
                  <button className="icon-btn" onClick={()=>reloadListener(l.id).then(load).catch((e)=>setError(e.message))}><RefreshCw size={16}/></button>
                  <button className="icon-btn" onClick={()=>{setCloneSrc(l);setCloneForm({name:l.name+'-copy',port:l.port})}}><Files size={16}/></button>
                  <button className="icon-btn" onClick={()=>{setTplSrc(l);setTplName(l.name)}}><Save size={16}/></button>
                  <button className="icon-btn" onClick={()=>listListenerVersions(l.id).then((v)=>{setVersions(v||[]);setVerListener(l)}).catch((e)=>setError(e.message))}><History size={16}/></button>
                  <button className="icon-btn" onClick={()=>setEdit({...l,...configToFormValues(l.config),public_host:(l as any).public_host||'',public_port:(l as any).public_port||'',access_sni:(l as any).access_sni||'',client_fingerprint:(l as any).client_fingerprint||'chrome',access_alpn:(l as any).access_alpn||''})}><Pencil size={16}/></button>
                  <button className="icon-btn" onClick={()=>deleteListener(l.id).then(load).catch((e)=>setError(e.message))}><Trash2 size={16}/></button>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>

      {templates.length>0 && (
        <div className="card">
          <h3>Templates</h3>
          {templates.map((tpl)=>(
            <div className="row" key={tpl.id}>
              <span>{tpl.name} · {tpl.protocol}</span>
              <div>
                <button className="btn" onClick={()=>{setInstSrc(tpl);setInstForm({name:tpl.name,port:'443'})}}>Instantiate</button>
                <button className="icon-btn" onClick={()=>deleteListenerTemplate(tpl.id).then(loadTemplates).catch((e)=>setError(e.message))}><Trash2 size={16}/></button>
              </div>
            </div>
          ))}
        </div>
      )}

      {edit && (
        <div className="overlay" onClick={()=>setEdit(null)}>
          <div className="modal" onClick={(e)=>e.stopPropagation()} style={{maxWidth:800}}>
            <h3>{edit.id?t('common.edit'):t('common.create')}</h3>
            <label className="label">{t('listeners.name')}<input className="input" value={edit.name||''} onChange={(e)=>set('name',e.target.value)} /></label>
            <label className="label">{t('listeners.protocol')}
              <select className="select" value={edit.protocol||'vless'} onChange={(e)=>set('protocol',e.target.value)}>
                {PROTOCOLS.map((p)=><option key={p} value={p}>{p}</option>)}
              </select>
            </label>
            <div className="row">
              <label className="label" style={{flex:1}}>{t('listeners.port')}<input className="input" value={edit.port||''} onChange={(e)=>set('port',e.target.value)} /></label>
              <label className="label" style={{flex:1}}>Bind<input className="input" value={edit.bind_address||'0.0.0.0'} onChange={(e)=>set('bind_address',e.target.value)} /></label>
            </div>
            <label className="label"><input type="checkbox" checked={!!edit.enabled} onChange={(e)=>set('enabled',e.target.checked)} /> {t('common.enabled')}</label>
            <h4>{t('settings.accessProfile')}</h4>
            <div className="row">
              <label className="label" style={{flex:1}}>{t('settings.publicHost')}<input className="input" value={edit.public_host||''} onChange={(e)=>set('public_host',e.target.value)} /></label>
              <label className="label" style={{flex:1}}>{t('settings.publicPort')}<input className="input" value={edit.public_port||''} onChange={(e)=>set('public_port',e.target.value)} /></label>
            </div>
            <ListenerConfigFields protocol={edit.protocol} values={edit} onChange={set} />
            <button className="btn primary" onClick={save}>{t('common.save')}</button>
          </div>
        </div>
      )}

      {uriOpen && (
        <div className="overlay" onClick={()=>setUriOpen(false)}>
          <div className="modal" onClick={(e)=>e.stopPropagation()}>
            <h3>{t('listeners.urisTitle')}</h3>
            {uris.map((u,i)=><div key={i} className="row"><input className="input" readOnly value={u} /><button className="icon-btn" onClick={()=>navigator.clipboard.writeText(u)}><Copy size={16}/></button></div>)}
          </div>
        </div>
      )}
      {cloneSrc && (
        <div className="overlay" onClick={()=>setCloneSrc(null)}>
          <div className="modal" onClick={(e)=>e.stopPropagation()}>
            <h3>Clone</h3>
            <label className="label">Name<input className="input" value={cloneForm.name} onChange={(e)=>setCloneForm({...cloneForm,name:e.target.value})} /></label>
            <label className="label">Port<input className="input" value={cloneForm.port} onChange={(e)=>setCloneForm({...cloneForm,port:e.target.value})} /></label>
            <button className="btn primary" onClick={async ()=>{ try{await cloneListener(cloneSrc.id,cloneForm);setCloneSrc(null);load();setOk('Cloned')}catch(e:any){setError(e.message)}}}>{t('common.save')}</button>
          </div>
        </div>
      )}
      {tplSrc && (
        <div className="overlay" onClick={()=>setTplSrc(null)}>
          <div className="modal" onClick={(e)=>e.stopPropagation()}>
            <h3>Template</h3>
            <label className="label">Name<input className="input" value={tplName} onChange={(e)=>setTplName(e.target.value)} /></label>
            <button className="btn primary" onClick={async ()=>{ try{await createListenerTemplate({name:tplName,protocol:tplSrc.protocol,config:tplSrc.config});setTplSrc(null);loadTemplates();setOk('Saved')}catch(e:any){setError(e.message)}}}>{t('common.save')}</button>
          </div>
        </div>
      )}
      {instSrc && (
        <div className="overlay" onClick={()=>setInstSrc(null)}>
          <div className="modal" onClick={(e)=>e.stopPropagation()}>
            <h3>Instantiate</h3>
            <label className="label">Name<input className="input" value={instForm.name} onChange={(e)=>setInstForm({...instForm,name:e.target.value})} /></label>
            <label className="label">Port<input className="input" value={instForm.port} onChange={(e)=>setInstForm({...instForm,port:e.target.value})} /></label>
            <button className="btn primary" onClick={async ()=>{ try{await instantiateListenerTemplate(instSrc.id,instForm);setInstSrc(null);load();setOk('Created')}catch(e:any){setError(e.message)}}}>{t('common.save')}</button>
          </div>
        </div>
      )}
      {verListener && (
        <div className="overlay" onClick={()=>setVerListener(null)}>
          <div className="modal" onClick={(e)=>e.stopPropagation()} style={{maxWidth:800}}>
            <h3>Versions — {verListener.name}</h3>
            <table className="table">
              <thead><tr><th>Ver</th><th>Reason</th><th>At</th><th></th></tr></thead>
              <tbody>
                {versions.map((v)=>(
                  <tr key={v.id}>
                    <td>{v.version}</td><td>{v.reason||'—'}</td><td>{new Date(v.created_at).toLocaleString()}</td>
                    <td>
                      <button className="btn" onClick={()=>diffListenerVersion(verListener.id,v.version).then((d)=>{setDiffText(typeof d==='string'?d:JSON.stringify(d,null,2));setDiffOpen(true)}).catch((e)=>setError(e.message))}>Diff</button>
                      <button className="btn" onClick={()=>rollbackListenerVersion(verListener.id,v.version).then(()=>{setVerListener(null);load();setOk('Rolled back')}).catch((e)=>setError(e.message))}>Rollback</button>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </div>
      )}
      {diffOpen && (
        <div className="overlay" onClick={()=>setDiffOpen(false)}>
          <div className="modal" onClick={(e)=>e.stopPropagation()} style={{maxWidth:900}}>
            <h3>Diff</h3>
            <pre style={{maxHeight:'65vh',overflow:'auto',whiteSpace:'pre-wrap'}}>{diffText||t('common.empty')}</pre>
          </div>
        </div>
      )}
    </div>
  )
}
