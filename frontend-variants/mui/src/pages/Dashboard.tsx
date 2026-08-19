import { useEffect, useState } from 'react'
import { Box, Card, CardContent, Typography, Button, Stack, Grid, LinearProgress, Alert } from '@mui/material'
import PlayArrowIcon from '@mui/icons-material/PlayArrow'
import StopIcon from '@mui/icons-material/Stop'
import RestartAltIcon from '@mui/icons-material/RestartAlt'
import { fetchDashboard, startMihomo, stopMihomo, restartMihomo } from '@shared/api/system'
import { formatBytes } from '@shared/utils/format'
import { useI18n } from '@shared/i18n'

function formatRate(n: number) {
  if (!n) return '0 B/s'
  return formatBytes(n) + '/s'
}

export default function Dashboard() {
  const { t } = useI18n()
  const [data, setData] = useState<any>()
  const [error, setError] = useState('')
  const [busy, setBusy] = useState(false)

  const load = async () => {
    try { setData(await fetchDashboard()) } catch (e: any) { setError(e.message) }
  }
  useEffect(() => { load(); const id = setInterval(load, 5000); return () => clearInterval(id) }, [])

  const act = async (a: 'start' | 'stop' | 'restart') => {
    setBusy(true)
    try {
      if (a === 'start') await startMihomo()
      else if (a === 'stop') await stopMihomo()
      else await restartMihomo()
      await load()
    } catch (e: any) { setError(e.message) }
    finally { setBusy(false) }
  }

  const sys = data?.system || {}
  const mihomo = data?.mihomo || {}

  return (
    <Box>
      <Typography variant="h5" fontWeight={700}>{t('dashboard.title')}</Typography>
      <Typography color="text.secondary" sx={{ mb: 2 }}>{t('dashboard.subtitle')}</Typography>
      {error && <Alert severity="error" onClose={() => setError('')} sx={{ mb: 2 }}>{error}</Alert>}
      <Grid container spacing={2}>
        <Grid size={{ xs: 12, md: 6, lg: 4 }}>
          <Card>
            <CardContent>
              <Typography variant="h6" sx={{ mb: 1 }}>{t('dashboard.mihomoStatus')}</Typography>
              <Typography sx={{ mb: 1 }}>
                {mihomo.running ? '🟢 Running' : '🔴 Stopped'} · {mihomo.version || '—'} · PID {mihomo.pid || '—'}
              </Typography>
              <Typography color="text.secondary" sx={{ mb: 2 }}>{mihomo.uptime || '—'}</Typography>
              <Stack direction="row" spacing={1}>
                <Button startIcon={<PlayArrowIcon />} variant="contained" disabled={busy} onClick={() => act('start')}>{t('dashboard.start')}</Button>
                <Button startIcon={<RestartAltIcon />} disabled={busy} onClick={() => act('restart')}>{t('dashboard.restart')}</Button>
                <Button startIcon={<StopIcon />} color="error" disabled={busy} onClick={() => act('stop')}>{t('dashboard.stop')}</Button>
              </Stack>
            </CardContent>
          </Card>
        </Grid>
        <Grid size={{ xs: 12, md: 6, lg: 4 }}>
          <Card>
            <CardContent>
              <Typography variant="h6" sx={{ mb: 1 }}>{t('dashboard.listeners')}</Typography>
              <Stack direction="row" spacing={3}>
                <Box><Typography variant="h4">{data?.listeners?.total || 0}</Typography><Typography variant="caption">{t('dashboard.total')}</Typography></Box>
                <Box><Typography variant="h4" color="success.main">{data?.listeners?.enabled || 0}</Typography><Typography variant="caption">{t('dashboard.enabled')}</Typography></Box>
                <Box><Typography variant="h4" color="error.main">{data?.listeners?.disabled || 0}</Typography><Typography variant="caption">{t('dashboard.disabled')}</Typography></Box>
              </Stack>
            </CardContent>
          </Card>
        </Grid>
        <Grid size={{ xs: 12, md: 6, lg: 4 }}>
          <Card>
            <CardContent>
              <Typography variant="h6" sx={{ mb: 1 }}>{t('dashboard.traffic')}</Typography>
              <Grid container spacing={1}>
                <Grid size={6}><Typography variant="body2" color="text.secondary">{t('dashboard.uploadRate')}</Typography><Typography fontWeight={600}>{formatRate(data?.traffic?.uploadRate || 0)}</Typography></Grid>
                <Grid size={6}><Typography variant="body2" color="text.secondary">{t('dashboard.downloadRate')}</Typography><Typography fontWeight={600}>{formatRate(data?.traffic?.downloadRate || 0)}</Typography></Grid>
                <Grid size={6}><Typography variant="body2" color="text.secondary">{t('dashboard.onlineUsers')}</Typography><Typography fontWeight={600}>{data?.traffic?.onlineUsers || 0}</Typography></Grid>
                <Grid size={6}><Typography variant="body2" color="text.secondary">{t('dashboard.activeConnections')}</Typography><Typography fontWeight={600}>{data?.traffic?.activeConnections || 0}</Typography></Grid>
              </Grid>
            </CardContent>
          </Card>
        </Grid>
        <Grid size={{ xs: 12, md: 4 }}>
          <Card><CardContent>
            <Typography variant="subtitle2">{t('dashboard.cpu')} {sys.cpu?.percent || 0}%</Typography>
            <LinearProgress variant="determinate" value={sys.cpu?.percent || 0} sx={{ mt: 1 }} />
          </CardContent></Card>
        </Grid>
        <Grid size={{ xs: 12, md: 4 }}>
          <Card><CardContent>
            <Typography variant="subtitle2">{t('dashboard.memory')} {sys.memory?.percent || 0}%</Typography>
            <LinearProgress variant="determinate" value={sys.memory?.percent || 0} sx={{ mt: 1 }} color={(sys.memory?.percent || 0) > 90 ? 'error' : 'primary'} />
            <Typography variant="caption" color="text.secondary">{formatBytes(sys.memory?.used || 0)} / {formatBytes(sys.memory?.total || 0)}</Typography>
          </CardContent></Card>
        </Grid>
        <Grid size={{ xs: 12, md: 4 }}>
          <Card><CardContent>
            <Typography variant="subtitle2">{t('dashboard.disk')} {sys.disk?.percent || 0}%</Typography>
            <LinearProgress variant="determinate" value={sys.disk?.percent || 0} sx={{ mt: 1 }} />
            <Typography variant="caption" color="text.secondary">{formatBytes(sys.disk?.used || 0)} / {formatBytes(sys.disk?.total || 0)}</Typography>
          </CardContent></Card>
        </Grid>
      </Grid>
    </Box>
  )
}
