import { useEffect, useState } from 'react'
import {
  Box, Typography, Button, Stack, Alert, Table, TableHead, TableRow, TableCell, TableBody,
  Dialog, DialogTitle, DialogContent, DialogActions, TextField, IconButton, Chip, Switch,
  FormControlLabel, Checkbox, Tooltip,
} from '@mui/material'
import AddIcon from '@mui/icons-material/Add'
import DeleteIcon from '@mui/icons-material/Delete'
import EditIcon from '@mui/icons-material/Edit'
import ContentCopyIcon from '@mui/icons-material/ContentCopy'
import LinkIcon from '@mui/icons-material/Link'
import RestartAltIcon from '@mui/icons-material/RestartAlt'
import HubIcon from '@mui/icons-material/Hub'
import {
  fetchUsers, createUser, updateUser, deleteUser, resetUserTraffic,
  fetchUserNodes, bindUserNodes, fetchUserSubscription, rotateUserSubscription, ProxyUser,
} from '@shared/api/users'
import { fetchListeners, Listener } from '@shared/api/nodes'
import { formatBytes } from '@shared/utils/format'
import { useI18n } from '@shared/i18n'

export default function Users() {
  const { t } = useI18n()
  const [rows, setRows] = useState<ProxyUser[]>([])
  const [error, setError] = useState('')
  const [ok, setOk] = useState('')
  const [edit, setEdit] = useState<any>(null)
  const [bindUser, setBindUser] = useState<ProxyUser | null>(null)
  const [allNodes, setAllNodes] = useState<Listener[]>([])
  const [selectedNodeIds, setSelectedNodeIds] = useState<number[]>([])
  const [subInfo, setSubInfo] = useState<{ token: string; url: string } | null>(null)

  const load = async () => { try { setRows(await fetchUsers()) } catch (e: any) { setError(e.message) } }
  useEffect(() => { load() }, [])

  const save = async () => {
    try {
      const payload: any = {
        username: edit.username, enabled: !!edit.enabled, remark: edit.remark,
        traffic_limit: edit.traffic_limit_gb ? Math.round(Number(edit.traffic_limit_gb) * 1024 * 1024 * 1024) : (edit.traffic_limit ? Number(edit.traffic_limit) : 0),
        ip_limit: edit.ip_limit ? Number(edit.ip_limit) : 0,
        expire_time: edit.expire_time || undefined,
      }
      if (edit.password) payload.password = edit.password
      if (edit.id) await updateUser(edit.id, payload)
      else await createUser(payload)
      setEdit(null); setOk('Saved'); load()
    } catch (e: any) { setError(e.message) }
  }

  const openBind = async (u: ProxyUser) => {
    setBindUser(u)
    try {
      const [nodes, bound] = await Promise.all([fetchListeners(), fetchUserNodes(u.id)])
      setAllNodes(nodes)
      setSelectedNodeIds((bound || []).map((n: any) => n.id))
    } catch (e: any) { setError(e.message) }
  }

  return (
    <Box>
      <Stack direction="row" justifyContent="space-between" sx={{ mb: 2 }}>
        <Typography variant="h5" fontWeight={700}>{t('users.title')}</Typography>
        <Button startIcon={<AddIcon />} variant="contained" onClick={() => setEdit({ enabled: true })}>{t('common.create')}</Button>
      </Stack>
      {error && <Alert severity="error" sx={{ mb: 2 }} onClose={() => setError('')}>{error}</Alert>}
      {ok && <Alert severity="success" sx={{ mb: 2 }} onClose={() => setOk('')}>{ok}</Alert>}
      <Table size="small">
        <TableHead>
          <TableRow>
            <TableCell>{t('users.username')}</TableCell>
            <TableCell>{t('users.traffic')}</TableCell>
            <TableCell>{t('users.status')}</TableCell>
            <TableCell>{t('users.remark')}</TableCell>
            <TableCell align="right">{t('common.actions')}</TableCell>
          </TableRow>
        </TableHead>
        <TableBody>
          {rows.map((u) => (
            <TableRow key={u.id}>
              <TableCell>
                <Typography fontWeight={600}>{u.username}</Typography>
                {u.uuid_masked && <Typography variant="caption" color="text.secondary">{u.uuid_masked}</Typography>}
              </TableCell>
              <TableCell>{formatBytes(u.traffic_used || 0)}{u.traffic_limit ? ` / ${formatBytes(u.traffic_limit)}` : ''}</TableCell>
              <TableCell>
                <Chip size="small" color={u.enabled ? 'success' : 'default'} label={u.enabled ? t('common.enabled') : t('common.disabled')} />
                {u.online && <Chip size="small" color="info" label="Online" sx={{ ml: 0.5 }} />}
                {u.blocked && <Chip size="small" color="error" label={t('users.blocked') !== 'users.blocked' ? t('users.blocked') : 'Blocked'} sx={{ ml: 0.5 }} />}
              </TableCell>
              <TableCell>{u.remark || '—'}</TableCell>
              <TableCell align="right">
                <Tooltip title="Bind nodes"><IconButton onClick={() => openBind(u)}><HubIcon fontSize="small" /></IconButton></Tooltip>
                <Tooltip title="Subscription"><IconButton onClick={() => fetchUserSubscription(u.id).then(setSubInfo).catch((e) => setError(e.message))}><LinkIcon fontSize="small" /></IconButton></Tooltip>
                <Tooltip title="Reset traffic"><IconButton onClick={() => resetUserTraffic(u.id).then(() => { setOk('Traffic reset'); load() }).catch((e) => setError(e.message))}><RestartAltIcon fontSize="small" /></IconButton></Tooltip>
                {u.sub_token && <IconButton onClick={() => navigator.clipboard.writeText(u.sub_token || '')}><ContentCopyIcon fontSize="small" /></IconButton>}
                <IconButton onClick={() => setEdit({
                  ...u,
                  traffic_limit_gb: u.traffic_limit ? (u.traffic_limit / (1024 * 1024 * 1024)).toFixed(2) : '',
                })}><EditIcon fontSize="small" /></IconButton>
                <IconButton color="error" onClick={() => deleteUser(u.id).then(load).catch((e) => setError(e.message))}><DeleteIcon fontSize="small" /></IconButton>
              </TableCell>
            </TableRow>
          ))}
        </TableBody>
      </Table>

      <Dialog open={!!edit} onClose={() => setEdit(null)} fullWidth maxWidth="sm">
        <DialogTitle>{edit?.id ? t('common.edit') : t('common.create')}</DialogTitle>
        <DialogContent>
          <TextField fullWidth label={t('users.username')} value={edit?.username || ''} onChange={(e) => setEdit({ ...edit, username: e.target.value })} sx={{ mt: 1, mb: 2 }} />
          <TextField fullWidth type="password" label={t('users.password')} value={edit?.password || ''} onChange={(e) => setEdit({ ...edit, password: e.target.value })} sx={{ mb: 2 }} />
          <TextField fullWidth label={t('users.remark')} value={edit?.remark || ''} onChange={(e) => setEdit({ ...edit, remark: e.target.value })} sx={{ mb: 2 }} />
          <TextField fullWidth label={t('users.trafficLimitGB') !== 'users.trafficLimitGB' ? t('users.trafficLimitGB') : 'Traffic limit (GB)'} value={edit?.traffic_limit_gb ?? ''} onChange={(e) => setEdit({ ...edit, traffic_limit_gb: e.target.value })} sx={{ mb: 2 }} />
          <TextField fullWidth label="IP limit" value={edit?.ip_limit ?? ''} onChange={(e) => setEdit({ ...edit, ip_limit: e.target.value })} sx={{ mb: 2 }} />
          <TextField fullWidth label="Expire time" value={edit?.expire_time || ''} onChange={(e) => setEdit({ ...edit, expire_time: e.target.value })} sx={{ mb: 2 }} helperText="ISO datetime" />
          <FormControlLabel control={<Switch checked={!!edit?.enabled} onChange={(e) => setEdit({ ...edit, enabled: e.target.checked })} />} label={t('common.enabled')} />
        </DialogContent>
        <DialogActions>
          <Button onClick={() => setEdit(null)}>{t('common.cancel')}</Button>
          <Button variant="contained" onClick={save}>{t('common.save')}</Button>
        </DialogActions>
      </Dialog>

      <Dialog open={!!bindUser} onClose={() => setBindUser(null)} fullWidth maxWidth="sm">
        <DialogTitle>{t('users.bind') !== 'users.bind' ? t('users.bind') : 'Bind nodes'} — {bindUser?.username}</DialogTitle>
        <DialogContent>
          <Stack>
            {allNodes.map((n) => (
              <FormControlLabel
                key={n.id}
                control={<Checkbox checked={selectedNodeIds.includes(n.id)} onChange={() => setSelectedNodeIds((ids) => ids.includes(n.id) ? ids.filter((x) => x !== n.id) : [...ids, n.id])} />}
                label={`${n.name} (${n.protocol}:${n.port})`}
              />
            ))}
          </Stack>
        </DialogContent>
        <DialogActions>
          <Button onClick={() => setBindUser(null)}>{t('common.cancel')}</Button>
          <Button variant="contained" onClick={async () => {
            if (!bindUser) return
            try { await bindUserNodes(bindUser.id, selectedNodeIds); setBindUser(null); setOk(t('users.bindSuccess') !== 'users.bindSuccess' ? t('users.bindSuccess') : 'Bound') } catch (e: any) { setError(e.message) }
          }}>{t('common.save')}</Button>
        </DialogActions>
      </Dialog>

      <Dialog open={!!subInfo} onClose={() => setSubInfo(null)} fullWidth maxWidth="sm">
        <DialogTitle>Subscription</DialogTitle>
        <DialogContent>
          <TextField fullWidth label="URL" value={subInfo?.url || ''} InputProps={{ readOnly: true }} sx={{ mt: 1, mb: 2 }} />
          <TextField fullWidth label="Token" value={subInfo?.token || ''} InputProps={{ readOnly: true }} />
        </DialogContent>
        <DialogActions>
          <Button onClick={() => subInfo && navigator.clipboard.writeText(subInfo.url)}>{t('common.copy')}</Button>
          <Button onClick={async () => {
            // find user by matching - rotate needs id; store on subInfo
          }}>Rotate</Button>
          <Button onClick={() => setSubInfo(null)}>Close</Button>
        </DialogActions>
      </Dialog>
    </Box>
  )
}
