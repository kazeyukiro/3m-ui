import { useEffect, useState } from 'react'
import {
  Box, Typography, Alert, Table, TableHead, TableRow, TableCell, TableBody, Chip, Card, CardContent, Grid,
} from '@mui/material'
import { fetchTrafficStatus, fetchTrafficUsers, fetchConnections } from '@shared/api/traffic'
import { formatBytes } from '@shared/utils/format'
import { useI18n } from '@shared/i18n'

export default function TrafficPage() {
  const { t } = useI18n()
  const [status, setStatus] = useState<any>()
  const [users, setUsers] = useState<any[]>([])
  const [conns, setConns] = useState<any[]>([])
  const [error, setError] = useState('')

  const load = async () => {
    try {
      const [s, u, c] = await Promise.all([fetchTrafficStatus(), fetchTrafficUsers(), fetchConnections()])
      setStatus(s); setUsers(u || []); setConns(c || [])
    } catch (e: any) { setError(e.message) }
  }
  useEffect(() => { load(); const id = setInterval(load, 5000); return () => clearInterval(id) }, [])

  return (
    <Box>
      <Typography variant="h5" fontWeight={700} sx={{ mb: 2 }}>{t('traffic.title')}</Typography>
      {error && <Alert severity="error" sx={{ mb: 2 }}>{error}</Alert>}
      <Grid container spacing={2} sx={{ mb: 2 }}>
        {[
          ['Upload', formatBytes(status?.upload_bytes || 0)],
          ['Download', formatBytes(status?.download_bytes || 0)],
          ['↑ rate', formatBytes(status?.upload_rate || 0) + '/s'],
          ['↓ rate', formatBytes(status?.download_rate || 0) + '/s'],
          ['Connections', status?.connections ?? conns.length],
        ].map(([a, b]) => (
          <Grid key={String(a)} size={{ xs: 6, md: 2.4 }}>
            <Card><CardContent>
              <Typography variant="caption" color="text.secondary">{a}</Typography>
              <Typography variant="h6">{b}</Typography>
            </CardContent></Card>
          </Grid>
        ))}
      </Grid>
      <Typography variant="h6" sx={{ mb: 1 }}>{t('traffic.users') !== 'traffic.users' ? t('traffic.users') : 'Users'}</Typography>
      <Table size="small" sx={{ mb: 3 }}>
        <TableHead>
          <TableRow>
            <TableCell>User</TableCell>
            <TableCell>Used</TableCell>
            <TableCell>Limit</TableCell>
            <TableCell>Status</TableCell>
          </TableRow>
        </TableHead>
        <TableBody>
          {users.map((u) => (
            <TableRow key={u.user_id}>
              <TableCell>{u.username}</TableCell>
              <TableCell>{formatBytes(u.traffic_used || 0)}</TableCell>
              <TableCell>{u.traffic_limit ? formatBytes(u.traffic_limit) : '∞'}</TableCell>
              <TableCell>
                <Chip size="small" color={u.online ? 'success' : 'default'} label={u.online ? 'Online' : 'Offline'} />
                {u.blocked && <Chip size="small" color="error" label="Blocked" sx={{ ml: 0.5 }} />}
              </TableCell>
            </TableRow>
          ))}
        </TableBody>
      </Table>
      <Typography variant="h6" sx={{ mb: 1 }}>{t('traffic.connections') !== 'traffic.connections' ? t('traffic.connections') : 'Connections'}</Typography>
      <Table size="small">
        <TableHead>
          <TableRow>
            <TableCell>User</TableCell>
            <TableCell>Network</TableCell>
            <TableCell>Rule</TableCell>
            <TableCell>↑</TableCell>
            <TableCell>↓</TableCell>
          </TableRow>
        </TableHead>
        <TableBody>
          {conns.slice(0, 100).map((c, i) => (
            <TableRow key={c.id || i}>
              <TableCell>{c.username || '—'}</TableCell>
              <TableCell>{c.network || c.type || '—'}</TableCell>
              <TableCell>{c.rule || '—'}</TableCell>
              <TableCell>{formatBytes(c.upload || 0)}</TableCell>
              <TableCell>{formatBytes(c.download || 0)}</TableCell>
            </TableRow>
          ))}
        </TableBody>
      </Table>
    </Box>
  )
}
