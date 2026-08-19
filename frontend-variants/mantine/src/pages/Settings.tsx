import { useEffect, useState } from 'react'
import { Title, Text, Paper, Stack, Select, TextInput, PasswordInput, Switch, Button, Group, Alert, FileButton } from '@mantine/core'
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
  useEffect(() => {
    fetchTelegramSettings().then((d) => setTg({ ...d, chat_ids_text: (d.chat_ids||[]).join(', ') })).catch(() => {})
  }, [])
  return (
    <Stack>
      <Title order={2}>{t('settings.title')}</Title>
      {error && <Alert color="red" onClose={() => setError('')}>{error}</Alert>}
      {ok && <Alert color="green" onClose={() => setOk('')}>{ok}</Alert>}
      <Paper withBorder p="md">
        <Group>
          <Select label={t('settings.language')} value={locale} onChange={(v) => v && setLocale(v as any)} data={[{value:'zh',label:'中文'},{value:'en',label:'English'}]} />
          <Select label={t('settings.theme')} value={mode} onChange={(v) => v && setMode(v as ThemeMode)} data={[{value:'light',label:t('settings.light')},{value:'dark',label:t('settings.dark')},{value:'system',label:t('settings.system')}]} />
        </Group>
      </Paper>
      <Paper withBorder p="md">
        <Text fw={600} mb="sm">{t('settings.passwordTitle')}</Text>
        <Stack maw={420}>
          <PasswordInput label="Current" value={pwd.current} onChange={(e)=>setPwd({...pwd,current:e.currentTarget.value})} />
          <PasswordInput label="New" value={pwd.next} onChange={(e)=>setPwd({...pwd,next:e.currentTarget.value})} />
          <PasswordInput label="Confirm" value={pwd.confirm} onChange={(e)=>setPwd({...pwd,confirm:e.currentTarget.value})} />
          <Button onClick={async () => { if (pwd.next!==pwd.confirm){setError('mismatch');return} try{await changePassword(pwd.current,pwd.next);setOk('updated');setPwd({current:'',next:'',confirm:''})}catch(e:any){setError(e.message)} }}>{t('settings.changePassword')}</Button>
        </Stack>
      </Paper>
      <Paper withBorder p="md">
        <Text fw={600} mb="sm">{t('settings.backup')}</Text>
        <Group>
          <Button onClick={async ()=>{try{await downloadBackup();setOk(t('settings.backupDone'))}catch(e:any){setError(e.message)}}}>{t('settings.downloadBackup')}</Button>
          <FileButton onChange={async (f)=>{ if(!f)return; try{await restoreDatabase(f);setOk(t('settings.restoreDone'))}catch(e:any){setError(e.message)} }} accept=".db,.sqlite,.zip">
            {(props)=><Button {...props} variant="default">{t('settings.restoreDb')}</Button>}
          </FileButton>
          <Button component="a" href={openApiUrl} target="_blank" variant="default">{t('settings.openOpenAPI')}</Button>
        </Group>
      </Paper>
      <Paper withBorder p="md">
        <Text fw={600} mb="sm">{t('settings.telegram')}</Text>
        <Switch label="Enabled" checked={!!tg.enabled} onChange={(e)=>setTg({...tg,enabled:e.currentTarget.checked})} mb="sm" />
        <PasswordInput label={t('settings.botToken')} value={tg.bot_token||''} onChange={(e)=>setTg({...tg,bot_token:e.currentTarget.value})} mb="sm" />
        <TextInput label={t('settings.chatIds')} value={tg.chat_ids_text||''} onChange={(e)=>setTg({...tg,chat_ids_text:e.currentTarget.value})} mb="sm" />
        <Group>
          <Button onClick={async ()=>{try{const chat_ids=String(tg.chat_ids_text||'').split(',').map((s:string)=>s.trim()).filter(Boolean);await saveTelegramSettings({...tg,chat_ids,keep_token:!tg.bot_token});setOk(t('settings.telegramSaved'))}catch(e:any){setError(e.message)}}}>{t('common.save')}</Button>
          <Button variant="default" onClick={async ()=>{try{await testTelegram();setOk(t('settings.telegramTestOk'))}catch(e:any){setError(e.message)}}}>{t('settings.telegramTest')}</Button>
        </Group>
      </Paper>
    </Stack>
  )
}
