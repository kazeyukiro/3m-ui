import { useEffect, useState } from 'react'
import { useI18n } from '@shared/i18n'
import { useThemeStore, ThemeMode } from '@shared/stores/themeStore'
import { downloadBackup, restoreDatabase, openApiUrl } from '@shared/api/system'
import { fetchTelegramSettings, saveTelegramSettings, testTelegram } from '@shared/api/telegram'
import { changePassword } from '@shared/api/auth'

export default function Settings() {
  const { t, locale, setLocale } = useI18n()
  const { mode, setMode } = useThemeStore()
  const [error, setError] = useState('')
  const [ok, setOk] = useState('')
  const [tg, setTg] = useState<any>({ enabled: false, chat_ids_text: '', bot_token: '' })
  const [pwd, setPwd] = useState({ current: '', next: '', confirm: '' })
  useEffect(() => { fetchTelegramSettings().then((d)=>setTg({...d, chat_ids_text:(d.chat_ids||[]).join(', ')})).catch(()=>{}) }, [])
  return (
    <div>
      <h2>{t('settings.title')}</h2>
      {error && <div className="alert">{error}</div>}
      {ok && <div className="alert ok">{ok}</div>}
      <div className="card">
        <div className="row" style={{justifyContent:'flex-start'}}>
          <label className="label" style={{margin:0}}>{t('settings.language')}
            <select className="select" value={locale} onChange={(e)=>setLocale(e.target.value as any)}><option value="zh">中文</option><option value="en">English</option></select>
          </label>
          <label className="label" style={{margin:0}}>{t('settings.theme')}
            <select className="select" value={mode} onChange={(e)=>setMode(e.target.value as ThemeMode)}>
              <option value="light">{t('settings.light')}</option>
              <option value="dark">{t('settings.dark')}</option>
              <option value="system">{t('settings.system')}</option>
            </select>
          </label>
        </div>
      </div>
      <div className="card">
        <h3>{t('settings.passwordTitle')}</h3>
        <label className="label">Current<input className="input" type="password" value={pwd.current} onChange={(e)=>setPwd({...pwd,current:e.target.value})} /></label>
        <label className="label">New<input className="input" type="password" value={pwd.next} onChange={(e)=>setPwd({...pwd,next:e.target.value})} /></label>
        <label className="label">Confirm<input className="input" type="password" value={pwd.confirm} onChange={(e)=>setPwd({...pwd,confirm:e.target.value})} /></label>
        <button className="btn primary" onClick={async ()=>{ if(pwd.next!==pwd.confirm){setError('mismatch');return} try{await changePassword(pwd.current,pwd.next);setOk('updated');setPwd({current:'',next:'',confirm:''})}catch(e:any){setError(e.message)}}}>{t('settings.changePassword')}</button>
      </div>
      <div className="card">
        <h3>{t('settings.backup')}</h3>
        <p className="muted">{t('settings.backupHint')}</p>
        <div className="row" style={{justifyContent:'flex-start'}}>
          <button className="btn primary" onClick={async ()=>{try{await downloadBackup();setOk(t('settings.backupDone'))}catch(e:any){setError(e.message)}}}>{t('settings.downloadBackup')}</button>
          <label className="btn">{t('settings.restoreDb')}<input hidden type="file" onChange={async (e)=>{ const f=e.target.files?.[0]; if(!f)return; try{await restoreDatabase(f);setOk(t('settings.restoreDone'))}catch(err:any){setError(err.message)}} } /></label>
          <a className="btn" href={openApiUrl} target="_blank" rel="noreferrer">{t('settings.openOpenAPI')}</a>
        </div>
      </div>
      <div className="card">
        <h3>{t('settings.telegram')}</h3>
        <label className="label"><input type="checkbox" checked={!!tg.enabled} onChange={(e)=>setTg({...tg,enabled:e.target.checked})} /> Enabled</label>
        <label className="label">{t('settings.botToken')}<input className="input" type="password" value={tg.bot_token||''} onChange={(e)=>setTg({...tg,bot_token:e.target.value})} /></label>
        <label className="label">{t('settings.chatIds')}<input className="input" value={tg.chat_ids_text||''} onChange={(e)=>setTg({...tg,chat_ids_text:e.target.value})} /></label>
        <div className="row" style={{justifyContent:'flex-start'}}>
          <button className="btn primary" onClick={async ()=>{try{const chat_ids=String(tg.chat_ids_text||'').split(',').map((s:string)=>s.trim()).filter(Boolean);await saveTelegramSettings({...tg,chat_ids,keep_token:!tg.bot_token});setOk(t('settings.telegramSaved'))}catch(e:any){setError(e.message)}}}>{t('common.save')}</button>
          <button className="btn" onClick={async ()=>{try{await testTelegram();setOk(t('settings.telegramTestOk'))}catch(e:any){setError(e.message)}}}>{t('settings.telegramTest')}</button>
        </div>
      </div>
    </div>
  )
}
