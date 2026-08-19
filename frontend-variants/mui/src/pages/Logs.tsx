import { useEffect, useState } from 'react'
import { Box, Card, CardContent, Typography, Button, Stack, Chip, List, ListItem, Alert } from '@mui/material'
import RefreshIcon from '@mui/icons-material/Refresh'
import DeleteSweepIcon from '@mui/icons-material/DeleteSweep'
import dayjs from 'dayjs'
import { fetchLogs } from '@shared/api/system'
import { useI18n } from '@shared/i18n'

export default function Logs() {
  const { t } = useI18n()
  const [logs, setLogs] = useState<any[]>([])
  const [auto, setAuto] = useState(true)
  const [error, setError] = useState('')
  const load = async () => {
    try { const d = await fetchLogs(); setLogs(Array.isArray(d) ? d : []) } catch (e: any) { setError(e.message) }
  }
  useEffect(() => {
    load()
    if (!auto) return
    const id = setInterval(load, 3000)
    return () => clearInterval(id)
  }, [auto])
  const color = (lv: string) => {
    const l = (lv || '').toLowerCase()
    if (l === 'error' || l === 'fatal') return 'error'
    if (l === 'warn' || l === 'warning') return 'warning'
    if (l === 'info') return 'info'
    return 'default'
  }
  return (
    <Box>
      <Typography variant="h5" fontWeight={700} sx={{ mb: 2 }}>{t('logs.title')}</Typography>
      {error && <Alert severity="error" sx={{ mb: 2 }}>{error}</Alert>}
      <Stack direction="row" spacing={1} sx={{ mb: 2 }}>
        <Button startIcon={<RefreshIcon />} onClick={load}>{t('common.refresh')}</Button>
        <Button startIcon={<DeleteSweepIcon />} onClick={() => setLogs([])}>{t('logs.clear')}</Button>
        <Button variant={auto ? 'contained' : 'outlined'} onClick={() => setAuto(!auto)}>
          {t('logs.autoRefresh')}: {auto ? t('common.enabled') : t('common.disabled')}
        </Button>
      </Stack>
      <Card>
        <CardContent>
          {logs.length === 0 ? <Typography color="text.secondary">{t('logs.empty')}</Typography> : (
            <List dense>
              {logs.map((log, i) => (
                <ListItem key={i} sx={{ fontFamily: 'ui-monospace, monospace', fontSize: 13, alignItems: 'flex-start' }}>
                  <Typography component="span" color="text.secondary" sx={{ mr: 1 }}>
                    [{dayjs(log.timestamp).format('YYYY-MM-DD HH:mm:ss')}]
                  </Typography>
                  <Chip size="small" label={(log.level || '').toUpperCase()} color={color(log.level) as any} sx={{ mr: 1 }} />
                  <Typography component="span">{log.payload}</Typography>
                </ListItem>
              ))}
            </List>
          )}
        </CardContent>
      </Card>
    </Box>
  )
}
