import { useEffect, useState } from 'react'
import {
  Box, Typography, Button, Stack, Alert, Table, TableHead, TableRow, TableCell, TableBody,
  Dialog, DialogTitle, DialogContent, DialogActions, TextField, IconButton, Chip,
} from '@mui/material'
import AddIcon from '@mui/icons-material/Add'
import DeleteIcon from '@mui/icons-material/Delete'
import HealthAndSafetyIcon from '@mui/icons-material/HealthAndSafety'
import {
  fetchCluster, createClusterNode, updateClusterNode, deleteClusterNode, healthClusterNode, RemoteServer,
} from '@shared/api/cluster'
import { useI18n } from '@shared/i18n'

export default function ClusterPage() {
  const { t } = useI18n()
  const [rows, setRows] = useState<RemoteServer[]>([])
  const [error, setError] = useState('')
  const [edit, setEdit] = useState<any>(null)

  const load = async () => {
    try { setRows(await fetchCluster()) } catch (e: any) { setError(e.message) }
  }
  useEffect(() => { load() }, [])

  const save = async () => {
    try {
      if (edit.id) await updateClusterNode(edit.id, edit)
      else await createClusterNode(edit)
      setEdit(null)
      load()
    } catch (e: any) { setError(e.message) }
  }

  return (
    <Box>
      <Stack direction="row" justifyContent="space-between" sx={{ mb: 2 }}>
        <Typography variant="h5" fontWeight={700}>{t('cluster.title')}</Typography>
        <Button startIcon={<AddIcon />} variant="contained" onClick={() => setEdit({ enabled: true })}>{t('common.create')}</Button>
      </Stack>
      {error && <Alert severity="error" sx={{ mb: 2 }} onClose={() => setError('')}>{error}</Alert>}
      <Table size="small">
        <TableHead>
          <TableRow>
            <TableCell>{t('cluster.name')}</TableCell>
            <TableCell>{t('cluster.baseUrl')}</TableCell>
            <TableCell>{t('cluster.status')}</TableCell>
            <TableCell align="right">{t('common.actions')}</TableCell>
          </TableRow>
        </TableHead>
        <TableBody>
          {rows.map((r) => (
            <TableRow key={r.id}>
              <TableCell>{r.name}</TableCell>
              <TableCell>{r.base_url}</TableCell>
              <TableCell><Chip size="small" label={r.last_status || (r.enabled ? 'enabled' : 'disabled')} /></TableCell>
              <TableCell align="right">
                <IconButton onClick={() => healthClusterNode(r.id).then(load).catch((e) => setError(e.message))}><HealthAndSafetyIcon /></IconButton>
                <IconButton color="error" onClick={() => deleteClusterNode(r.id).then(load).catch((e) => setError(e.message))}><DeleteIcon /></IconButton>
              </TableCell>
            </TableRow>
          ))}
        </TableBody>
      </Table>
      <Dialog open={!!edit} onClose={() => setEdit(null)} fullWidth maxWidth="sm">
        <DialogTitle>{edit?.id ? t('common.edit') : t('common.create')}</DialogTitle>
        <DialogContent>
          <TextField fullWidth label={t('cluster.name')} value={edit?.name || ''} onChange={(e) => setEdit({ ...edit, name: e.target.value })} sx={{ mt: 1, mb: 2 }} />
          <TextField fullWidth label={t('cluster.baseUrl')} value={edit?.base_url || ''} onChange={(e) => setEdit({ ...edit, base_url: e.target.value })} sx={{ mb: 2 }} />
          <TextField fullWidth type="password" label={t('cluster.apiToken')} value={edit?.api_token || ''} onChange={(e) => setEdit({ ...edit, api_token: e.target.value })} />
        </DialogContent>
        <DialogActions>
          <Button onClick={() => setEdit(null)}>{t('common.cancel')}</Button>
          <Button variant="contained" onClick={save}>{t('common.save')}</Button>
        </DialogActions>
      </Dialog>
    </Box>
  )
}
