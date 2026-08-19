import { useEffect, useState } from 'react'
import { Box, Card, CardContent, Typography, Button, Stack, Alert, Chip } from '@mui/material'
import PlayArrowIcon from '@mui/icons-material/PlayArrow'
import StopIcon from '@mui/icons-material/Stop'
import RestartAltIcon from '@mui/icons-material/RestartAlt'
import { fetchDashboard, startMihomo, stopMihomo, restartMihomo } from '@shared/api/system'
import { useI18n } from '@shared/i18n'

export default function Core() {
  const { t } = useI18n()
  const [data, setData] = useState<any>()
  const [error, setError] = useState('')
  const [busy, setBusy] = useState(false)
  const load = async () => { try { setData(await fetchDashboard()) } catch (e: any) { setError(e.message) } }
  useEffect(() => { load() }, [])
  const act = async (fn: () => Promise<any>) => {
    setBusy(true)
    try { await fn(); await load() } catch (e: any) { setError(e.message) } finally { setBusy(false) }
  }
  const m = data?.mihomo
  return (
    <Box>
      <Typography variant="h5" fontWeight={700} sx={{ mb: 2 }}>{t('core.title')}</Typography>
      {error && <Alert severity="error" sx={{ mb: 2 }} onClose={() => setError('')}>{error}</Alert>}
      <Card>
        <CardContent>
          <Stack direction="row" spacing={1} alignItems="center" sx={{ mb: 2 }}>
            <Chip label={m?.running ? t('core.running') : t('core.stopped')} color={m?.running ? 'success' : 'default'} />
            <Typography>{m?.version || '—'} · PID {m?.pid || '—'} · {m?.uptime || '—'}</Typography>
          </Stack>
          <Stack direction="row" spacing={1}>
            <Button startIcon={<PlayArrowIcon />} variant="contained" disabled={busy} onClick={() => act(startMihomo)}>{t('core.start')}</Button>
            <Button startIcon={<RestartAltIcon />} disabled={busy} onClick={() => act(restartMihomo)}>{t('core.restart')}</Button>
            <Button startIcon={<StopIcon />} color="error" disabled={busy} onClick={() => act(stopMihomo)}>{t('core.stop')}</Button>
          </Stack>
        </CardContent>
      </Card>
    </Box>
  )
}
