import { useEffect, useState } from 'react'
import { fetchGroups, saveGroups, fetchRules, saveRules, GroupEntry } from '@shared/api/routing'
import { useI18n } from '@shared/i18n'

export default function RoutingPage() {
  const { t } = useI18n()
  const [groups, setGroups] = useState<GroupEntry[]>([])
  const [rulesText, setRulesText] = useState('')
  const [error, setError] = useState('')
  const [ok, setOk] = useState('')
  const load = async () => {
    try {
      const [g,r] = await Promise.all([fetchGroups(), fetchRules()])
      setGroups(g||[]); setRulesText((r||[]).join('\n'))
    } catch (e: any) { setError(e.message) }
  }
  useEffect(() => { load() }, [])
  return (
    <div>
      <h2>{t('routing.title')}</h2>
      <p className="muted">{t('routing.subtitle')}</p>
      {error && <div className="alert">{error}</div>}
      {ok && <div className="alert ok">{ok}</div>}
      <div className="card">
        <h3>{t('routing.groups')}</h3>
        <textarea className="textarea" style={{minHeight:180}} value={JSON.stringify(groups,null,2)} onChange={(e)=>{ try{ setGroups(JSON.parse(e.target.value)) }catch{} }} />
        <button className="btn primary" style={{marginTop:8}} onClick={async ()=>{ try{ await saveGroups(groups); setOk(t('routing.groupSaved')); load() }catch(e:any){setError(e.message)}}}>{t('common.save')}</button>
      </div>
      <div className="card">
        <h3>{t('routing.rules')}</h3>
        <p className="muted">{t('routing.rulesHint')}</p>
        <textarea className="textarea" style={{minHeight:240}} value={rulesText} onChange={(e)=>setRulesText(e.target.value)} />
        <button className="btn primary" style={{marginTop:8}} onClick={async ()=>{ try{ await saveRules(rulesText.split('\n').map(s=>s.trim()).filter(Boolean)); setOk(t('routing.rulesSaved')); load() }catch(e:any){setError(e.message)}}}>{t('common.save')}</button>
      </div>
    </div>
  )
}
