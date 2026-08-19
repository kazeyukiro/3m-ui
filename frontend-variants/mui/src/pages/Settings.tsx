import { useEffect, useState } from 'react'
import {
  Box, Typography, Button, Stack, Alert, TextField, FormControlLabel, Switch, Paper, Divider, MenuItem,
} from '@mui/material'
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
  const [tg, setTg] = useState<any>({ enabled: false, chat_ids: [], bot_token: '' })
  const [pwd, setPwd] = useState({ current: '', next: '', confirm: '' })

  useEffect(() => {
    fetchTelegramSettings().then((d) => setTg({
      ...d,
      chat_ids_text: (d.chat_ids || []).join(', '),
    })).catch(() => {})
  }, [])

  return (
    <Box>
      <Typography variant="h5" fontWeight={700} sx={{ mb: 1 }}>{t('settings.title')}</Typography>
      <Typography color="text.secondary" sx={{ mb: 2 }}>{t('settings.subtitle')}</Typography>
      {error && <Alert severity="error" sx={{ mb: 2 }} onClose={() => setError('')}>{error}</Alert>}
      {ok && <Alert severity="success" sx={{ mb: 2 }} onClose={() => setOk('')}>{ok}</Alert>}

      <Paper sx={{ p: 2, mb: 2 }}>
        <Typography variant="h6" sx={{ mb: 2 }}>{t('settings.language')} / {t('settings.theme')}</Typography>
        <Stack direction={{ xs: 'column', sm: 'row' }} spacing={2}>
          <TextField select label={t('settings.language')} value={locale} onChange={(e) => setLocale(e.target.value as any)} sx={{ minWidth: 160 }}>
            <MenuItem value="zh">中文</MenuItem>
            <MenuItem value="en">English</MenuItem>
          </TextField>
          <TextField select label={t('settings.theme')} value={mode} onChange={(e) => setMode(e.target.value as ThemeMode)} sx={{ minWidth: 160 }}>
            <MenuItem value="light">{t('settings.light')}</MenuItem>
            <MenuItem value="dark">{t('settings.dark')}</MenuItem>
            <MenuItem value="system">{t('settings.system')}</MenuItem>
          </TextField>
        </Stack>
      </Paper>

      <Paper sx={{ p: 2, mb: 2 }}>
        <Typography variant="h6" sx={{ mb: 2 }}>{t('settings.passwordTitle')}</Typography>
        <Stack spacing={2} maxWidth={420}>
          <TextField type="password" label="Current" value={pwd.current} onChange={(e) => setPwd({ ...pwd, current: e.target.value })} />
          <TextField type="password" label="New" value={pwd.next} onChange={(e) => setPwd({ ...pwd, next: e.target.value })} />
          <TextField type="password" label="Confirm" value={pwd.confirm} onChange={(e) => setPwd({ ...pwd, confirm: e.target.value })} />
          <Button variant="contained" onClick={async () => {
            if (pwd.next !== pwd.confirm) { setError('Passwords do not match'); return }
            try { await changePassword(pwd.current, pwd.next); setOk('Password updated'); setPwd({ current: '', next: '', confirm: '' }) }
            catch (e: any) { setError(e.message) }
          }}>{t('settings.changePassword')}</Button>
        </Stack>
      </Paper>

      <Paper sx={{ p: 2, mb: 2 }}>
        <Typography variant="h6" sx={{ mb: 2 }}>{t('settings.backup')}</Typography>
        <Typography variant="body2" color="text.secondary" sx={{ mb: 2 }}>{t('settings.backupHint')}</Typography>
        <Stack direction="row" spacing={1}>
          <Button variant="contained" onClick={async () => { try { await downloadBackup(); setOk(t('settings.backupDone')) } catch (e: any) { setError(e.message) } }}>{t('settings.downloadBackup')}</Button>
          <Button component="label">
            {t('settings.restoreDb')}
            <input hidden type="file" accept=".db,.sqlite,.zip" onChange={async (e) => {
              const f = e.target.files?.[0]
              if (!f) return
              try { await restoreDatabase(f); setOk(t('settings.restoreDone')) } catch (err: any) { setError(err.message) }
            }} />
          </Button>
          <Button href={openApiUrl} target="_blank">{t('settings.openOpenAPI')}</Button>
        </Stack>
      </Paper>

      <Paper sx={{ p: 2 }}>
        <Typography variant="h6" sx={{ mb: 2 }}>{t('settings.telegram')}</Typography>
        <FormControlLabel control={<Switch checked={!!tg.enabled} onChange={(e) => setTg({ ...tg, enabled: e.target.checked })} />} label="Enabled" />
        <TextField fullWidth label={t('settings.botToken')} type="password" value={tg.bot_token || ''} onChange={(e) => setTg({ ...tg, bot_token: e.target.value })} sx={{ my: 1 }} />
        <TextField fullWidth label={t('settings.chatIds')} helperText={t('settings.chatIdsHint')} value={tg.chat_ids_text || ''} onChange={(e) => setTg({ ...tg, chat_ids_text: e.target.value })} sx={{ mb: 1 }} />
        <FormControlLabel control={<Switch checked={!!tg.notify_on_block} onChange={(e) => setTg({ ...tg, notify_on_block: e.target.checked })} />} label={t('settings.notifyBlock')} />
        <FormControlLabel control={<Switch checked={!!tg.notify_on_unblock} onChange={(e) => setTg({ ...tg, notify_on_unblock: e.target.checked })} />} label={t('settings.notifyUnblock')} />
        <FormControlLabel control={<Switch checked={!!tg.notify_on_expiry} onChange={(e) => setTg({ ...tg, notify_on_expiry: e.target.checked })} />} label={t('settings.notifyExpiry')} />
        <FormControlLabel control={<Switch checked={!!tg.notify_daily_digest} onChange={(e) => setTg({ ...tg, notify_daily_digest: e.target.checked })} />} label={t('settings.notifyDailyDigest')} />
        <Stack direction="row" spacing={1} sx={{ mt: 2 }}>
          <Button variant="contained" onClick={async () => {
            try {
              const chat_ids = String(tg.chat_ids_text || '').split(',').map((s: string) => s.trim()).filter(Boolean)
              await saveTelegramSettings({ ...tg, chat_ids, keep_token: !tg.bot_token })
              setOk(t('settings.telegramSaved'))
            } catch (e: any) { setError(e.message) }
          }}>{t('common.save')}</Button>
          <Button onClick={async () => { try { await testTelegram(); setOk(t('settings.telegramTestOk')) } catch (e: any) { setError(e.message) } }}>{t('settings.telegramTest')}</Button>
        </Stack>
      </Paper>
    </Box>
  )
}
