import { useEffect, useState } from 'react'
import Editor from '@monaco-editor/react'
import { fetchConfigYAML, validateConfigYAML, generateConfig } from '@shared/api/config'
import { useI18n } from '@shared/i18n'

export default function ConfigPage() {
  const { t } = useI18n()
  const [yaml, setYaml] = useState('')
  const [error, setError] = useState('')
  const [ok, setOk] = useState('')
  const load = async () => { try { setYaml((await fetchConfigYAML())?.config||'') } catch (e: any) { setError(e.message) } }
  useEffect(() => { load() }, [])
  return (
    <div>
      <h2>{t('config.title')}</h2>
      {error && <div className="alert">{error}</div>}
      {ok && <div className="alert ok">{ok}</div>}
      <div className="row" style={{marginBottom:12,justifyContent:'flex-start'}}>
        <button className="btn primary" onClick={async ()=>{ try{ await generateConfig(); await load(); setOk('Generated') }catch(e:any){setError(e.message)}}}>{t('config.generate')}</button>
        <button className="btn" onClick={async ()=>{ try{ const r=await validateConfigYAML(yaml); if(r?.valid===false)setError(r.error||'Invalid'); else{setError('');setOk('Valid')} }catch(e:any){setError(e.message)}}}>{t('config.validate')}</button>
        <button className="btn" onClick={load}>{t('common.refresh')}</button>
      </div>
      <div className="card" style={{padding:0,height:560,overflow:'hidden'}}>
        <Editor language="yaml" value={yaml} onChange={(v)=>setYaml(v||'')} options={{ minimap:{enabled:false}, fontSize:13 }} />
      </div>
    </div>
  )
}
