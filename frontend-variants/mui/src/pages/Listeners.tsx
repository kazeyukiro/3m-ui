import { useEffect, useState } from 'react'
import {
  Box, Typography, Button, Stack, Alert, Table, TableHead, TableRow, TableCell, TableBody,
  Dialog, DialogTitle, DialogContent, DialogActions, TextField, IconButton, Chip, Switch,
  FormControlLabel, MenuItem, Divider, Checkbox, Tooltip,
} from '@mui/material'
import AddIcon from '@mui/icons-material/Add'
import DeleteIcon from '@mui/icons-material/Delete'
import EditIcon from '@mui/icons-material/Edit'
import RefreshIcon from '@mui/icons-material/Refresh'
import LinkIcon from '@mui/icons-material/Link'
import ContentCopyIcon from '@mui/icons-material/ContentCopy'
import HistoryIcon from '@mui/icons-material/History'
import FileCopyIcon from '@mui/icons-material/FileCopy'
import SaveIcon from '@mui/icons-material/Save'
import {
  fetchListeners, createListener, updateListener, deleteListener, reloadListener, exportNodeURI, Listener,
} from '@shared/api/nodes'
import {
  listListenerTemplates, createListenerTemplate, deleteListenerTemplate, instantiateListenerTemplate,
  cloneListener, batchSetListenersEnabled, listListenerVersions, diffListenerVersion, rollbackListenerVersion,
  ListenerTemplate, ListenerVersion,
} from '@shared/api/listeners'
import { fetchCapabilities, protocolCapability, CapabilityManifest } from '@shared/api/capabilities'
import { configToFormValues, formValuesToConfig, protocolSupportsUDP } from '@shared/logic/listenerConfig'
import { capabilityFormToConfig } from '@shared/logic/capabilityForm'
import ListenerConfigFields from '../components/ListenerConfigFields'
import CapabilityFormFields from '../components/CapabilityFormFields'
import { useI18n } from '@shared/i18n'

const PROTOCOLS = ['shadowsocks', 'snell', 'vmess', 'vless', 'trojan', 'hysteria2', 'tuic', 'shadowquic', 'anytls', 'mieru', 'sudoku', 'trusttunnel']
const REALITY_PROTOCOLS = new Set(['vmess', 'vless', 'trojan'])

export default function Listeners() {
  const { t } = useI18n()
  const [rows, setRows] = useState<Listener[]>([])
  const [templates, setTemplates] = useState<ListenerTemplate[]>([])
  const [error, setError] = useState('')
  const [ok, setOk] = useState('')
  const [edit, setEdit] = useState<any>(null)
  const [capabilities, setCapabilities] = useState<CapabilityManifest | null>(null)
  const [selected, setSelected] = useState<number[]>([])
  const [uris, setUris] = useState<string[]>([])
  const [uriOpen, setUriOpen] = useState(false)
  const [cloneSrc, setCloneSrc] = useState<Listener | null>(null)
  const [cloneForm, setCloneForm] = useState({ name: '', port: '' })
  const [tplSrc, setTplSrc] = useState<Listener | null>(null)
  const [tplName, setTplName] = useState('')
  const [instSrc, setInstSrc] = useState<ListenerTemplate | null>(null)
  const [instForm, setInstForm] = useState({ name: '', port: '' })
  const [versions, setVersions] = useState<ListenerVersion[]>([])
  const [verListener, setVerListener] = useState<Listener | null>(null)
  const [diffText, setDiffText] = useState('')
  const [diffOpen, setDiffOpen] = useState(false)

  const load = async () => {
    try { setRows(await fetchListeners()) } catch (e: any) { setError(e.message) }
  }
  const loadTemplates = async () => {
    try { setTemplates(await listListenerTemplates()) } catch { /* optional */ }
  }
  useEffect(() => {
    load(); loadTemplates()
    fetchCapabilities().then(setCapabilities).catch(() => setCapabilities(null))
  }, [])

  const set = (k: string, v: any) => setEdit((p: any) => ({ ...p, [k]: v }))
  const cap = capabilities && edit?.protocol ? protocolCapability(capabilities, edit.protocol) : undefined

  const openCreate = () => setEdit({
    protocol: 'vless', port: '443', bind_address: '0.0.0.0', enabled: true, udp: false,
    transport_layer: 'raw', security_layer: 'reality', flow: 'xtls-rprx-vision', client_fingerprint: 'chrome',
  })
  const openEdit = (record: Listener) => setEdit({
    ...record,
    ...configToFormValues(record.config),
    public_host: (record as any).public_host || '',
    public_port: (record as any).public_port || '',
    access_sni: (record as any).access_sni || '',
    client_fingerprint: (record as any).client_fingerprint || 'chrome',
    access_alpn: (record as any).access_alpn || '',
  })

  const save = async () => {
    try {
      if (!edit?.name || !edit?.protocol || !String(edit?.port || '').trim()) {
        setError(t('listeners.portHint') !== 'listeners.portHint' ? t('listeners.portHint') : 'Name/protocol/port required')
        return
      }
      if (REALITY_PROTOCOLS.has(edit.protocol) && edit.security_layer === 'reality') {
        if (!edit.reality_dest || !edit.reality_private_key) {
          setError('Reality Dest / Private Key cannot be empty')
          return
        }
      }
      const previous = edit.id ? (() => { try { return JSON.parse(edit.config || '{}') } catch { return null } })() : null
      const config = {
        ...formValuesToConfig(edit.protocol, edit, previous),
        ...(cap ? capabilityFormToConfig(edit.protocol, edit, cap) : {}),
      }
      const payload: any = {
        name: edit.name, protocol: edit.protocol, port: String(edit.port),
        bind_address: edit.bind_address || '0.0.0.0', enabled: !!edit.enabled,
        udp: protocolSupportsUDP(edit.protocol) ? !!edit.udp : false,
        config: JSON.stringify(config),
        public_host: edit.public_host || '', public_port: edit.public_port || '',
        access_sni: edit.access_sni || '', client_fingerprint: edit.client_fingerprint || '', access_alpn: edit.access_alpn || '',
      }
      if (edit.id) await updateListener(edit.id, payload)
      else await createListener(payload)
      setEdit(null); setOk(t('common.saved') !== 'common.saved' ? t('common.saved') : 'Saved'); load()
    } catch (e: any) { setError(e.message) }
  }

  const toggleSelect = (id: number) => setSelected((s) => s.includes(id) ? s.filter((x) => x !== id) : [...s, id])

  return (
    <Box>
      <Stack direction="row" justifyContent="space-between" sx={{ mb: 2 }} flexWrap="wrap" gap={1}>
        <Typography variant="h5" fontWeight={700}>{t('listeners.title')}</Typography>
        <Stack direction="row" spacing={1}>
          {selected.length > 0 && (
            <>
              <Button size="small" onClick={() => batchSetListenersEnabled(selected, true).then(() => { setSelected([]); load() }).catch((e) => setError(e.message))}>Enable ({selected.length})</Button>
              <Button size="small" onClick={() => batchSetListenersEnabled(selected, false).then(() => { setSelected([]); load() }).catch((e) => setError(e.message))}>Disable ({selected.length})</Button>
            </>
          )}
          <Button startIcon={<AddIcon />} variant="contained" onClick={openCreate}>{t('common.create')}</Button>
        </Stack>
      </Stack>
      {error && <Alert severity="error" sx={{ mb: 2 }} onClose={() => setError('')}>{error}</Alert>}
      {ok && <Alert severity="success" sx={{ mb: 2 }} onClose={() => setOk('')}>{ok}</Alert>}

      <Table size="small">
        <TableHead>
          <TableRow>
            <TableCell padding="checkbox" />
            <TableCell>{t('listeners.name')}</TableCell>
            <TableCell>{t('listeners.protocol')}</TableCell>
            <TableCell>{t('listeners.port')}</TableCell>
            <TableCell>{t('listeners.status')}</TableCell>
            <TableCell align="right">{t('common.actions')}</TableCell>
          </TableRow>
        </TableHead>
        <TableBody>
          {rows.map((l) => (
            <TableRow key={l.id} selected={selected.includes(l.id)}>
              <TableCell padding="checkbox"><Checkbox checked={selected.includes(l.id)} onChange={() => toggleSelect(l.id)} /></TableCell>
              <TableCell>{l.name}</TableCell>
              <TableCell>{l.protocol}</TableCell>
              <TableCell>{l.bind_address || '0.0.0.0'}:{l.port}</TableCell>
              <TableCell><Chip size="small" color={l.enabled ? 'success' : 'default'} label={l.enabled ? t('common.enabled') : t('common.disabled')} /></TableCell>
              <TableCell align="right">
                <Tooltip title="URI"><IconButton onClick={() => exportNodeURI(l.id).then((d: any) => { setUris(d?.uris || d?.links || (d?.uri ? [d.uri] : [])); setUriOpen(true) }).catch((e) => setError(e.message))}><LinkIcon fontSize="small" /></IconButton></Tooltip>
                <Tooltip title="Reload"><IconButton onClick={() => reloadListener(l.id).then(load).catch((e) => setError(e.message))}><RefreshIcon fontSize="small" /></IconButton></Tooltip>
                <Tooltip title="Clone"><IconButton onClick={() => { setCloneSrc(l); setCloneForm({ name: l.name + '-copy', port: l.port }) }}><FileCopyIcon fontSize="small" /></IconButton></Tooltip>
                <Tooltip title="Template"><IconButton onClick={() => { setTplSrc(l); setTplName(l.name) }}><SaveIcon fontSize="small" /></IconButton></Tooltip>
                <Tooltip title="Versions"><IconButton onClick={() => listListenerVersions(l.id).then((v) => { setVersions(v || []); setVerListener(l) }).catch((e) => setError(e.message))}><HistoryIcon fontSize="small" /></IconButton></Tooltip>
                <IconButton onClick={() => openEdit(l)}><EditIcon fontSize="small" /></IconButton>
                <IconButton color="error" onClick={() => deleteListener(l.id).then(load).catch((e) => setError(e.message))}><DeleteIcon fontSize="small" /></IconButton>
              </TableCell>
            </TableRow>
          ))}
        </TableBody>
      </Table>

      {templates.length > 0 && (
        <Box sx={{ mt: 3 }}>
          <Typography variant="h6" sx={{ mb: 1 }}>{t('listeners.templates') !== 'listeners.templates' ? t('listeners.templates') : 'Templates'}</Typography>
          <Table size="small">
            <TableBody>
              {templates.map((tpl) => (
                <TableRow key={tpl.id}>
                  <TableCell>{tpl.name}</TableCell>
                  <TableCell>{tpl.protocol}</TableCell>
                  <TableCell align="right">
                    <Button size="small" onClick={() => { setInstSrc(tpl); setInstForm({ name: tpl.name, port: '443' }) }}>{t('listeners.instantiate') !== 'listeners.instantiate' ? t('listeners.instantiate') : 'Instantiate'}</Button>
                    <IconButton color="error" onClick={() => deleteListenerTemplate(tpl.id).then(loadTemplates).catch((e) => setError(e.message))}><DeleteIcon fontSize="small" /></IconButton>
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </Box>
      )}

      {/* Create/Edit dialog */}
      <Dialog open={!!edit} onClose={() => setEdit(null)} fullWidth maxWidth="md">
        <DialogTitle>{edit?.id ? t('common.edit') : t('common.create')} Listener</DialogTitle>
        <DialogContent dividers>
          <Stack spacing={2} sx={{ pt: 1 }}>
            <TextField label={t('listeners.name')} value={edit?.name || ''} onChange={(e) => set('name', e.target.value)} fullWidth required />
            <TextField select label={t('listeners.protocol')} value={edit?.protocol || 'vless'} onChange={(e) => {
              const protocol = e.target.value
              const next: any = { protocol }
              // Clamp security when switching away from Reality-capable protocols
              if (!REALITY_PROTOCOLS.has(protocol) && (edit?.security_layer === 'reality' || edit?.reality_enabled)) {
                next.security_layer = 'tls'
                next.reality_enabled = false
              }
              setEdit((prev: any) => ({ ...prev, ...next }))
            }} fullWidth>
              {PROTOCOLS.map((p) => <MenuItem key={p} value={p}>{p}</MenuItem>)}
            </TextField>
            <Stack direction={{ xs: 'column', sm: 'row' }} spacing={2}>
              <TextField label={t('listeners.port')} value={edit?.port || ''} onChange={(e) => set('port', e.target.value)} fullWidth required />
              <TextField label={t('listeners.bind') !== 'listeners.bind' ? t('listeners.bind') : 'Bind'} value={edit?.bind_address || '0.0.0.0'} onChange={(e) => set('bind_address', e.target.value)} fullWidth />
            </Stack>
            <FormControlLabel control={<Switch checked={!!edit?.enabled} onChange={(e) => set('enabled', e.target.checked)} />} label={t('common.enabled')} />
            {protocolSupportsUDP(edit?.protocol || '') && (
              <FormControlLabel control={<Switch checked={!!edit?.udp} onChange={(e) => set('udp', e.target.checked)} />} label="UDP" />
            )}
            <Divider>{t('settings.accessProfile')}</Divider>
            <Stack direction={{ xs: 'column', sm: 'row' }} spacing={2}>
              <TextField label={t('settings.publicHost')} value={edit?.public_host || ''} onChange={(e) => set('public_host', e.target.value)} fullWidth />
              <TextField label={t('settings.publicPort')} value={edit?.public_port || ''} onChange={(e) => set('public_port', e.target.value)} fullWidth />
            </Stack>
            <Stack direction={{ xs: 'column', sm: 'row' }} spacing={2}>
              <TextField label="SNI" value={edit?.access_sni || ''} onChange={(e) => set('access_sni', e.target.value)} fullWidth />
              <TextField label={t('settings.clientFingerprint')} value={edit?.client_fingerprint || 'chrome'} onChange={(e) => set('client_fingerprint', e.target.value)} fullWidth />
            </Stack>
            <TextField label="ALPN" helperText={t('settings.alpnHint')} value={edit?.access_alpn || ''} onChange={(e) => set('access_alpn', e.target.value)} fullWidth />
            <ListenerConfigFields protocol={edit?.protocol} values={edit || {}} onChange={set} />
            {cap && <CapabilityFormFields protocol={edit?.protocol} capability={cap} showAdvanced values={edit || {}} onChange={set} />}
          </Stack>
        </DialogContent>
        <DialogActions>
          <Button onClick={() => setEdit(null)}>{t('common.cancel')}</Button>
          <Button variant="contained" onClick={save}>{t('common.save')}</Button>
        </DialogActions>
      </Dialog>

      {/* URI */}
      <Dialog open={uriOpen} onClose={() => setUriOpen(false)} fullWidth maxWidth="sm">
        <DialogTitle>{t('listeners.urisTitle')}</DialogTitle>
        <DialogContent>
          <Stack spacing={1} sx={{ pt: 1 }}>
            {uris.length === 0 ? <Typography color="text.secondary">{t('common.empty')}</Typography> : uris.map((u, i) => (
              <TextField key={i} value={u} fullWidth InputProps={{ readOnly: true, endAdornment: (
                <IconButton onClick={() => navigator.clipboard.writeText(u)}><ContentCopyIcon fontSize="small" /></IconButton>
              ) }} />
            ))}
          </Stack>
        </DialogContent>
      </Dialog>

      {/* Clone */}
      <Dialog open={!!cloneSrc} onClose={() => setCloneSrc(null)}>
        <DialogTitle>{t('listeners.clone') !== 'listeners.clone' ? t('listeners.clone') : 'Clone'}</DialogTitle>
        <DialogContent>
          <TextField fullWidth label={t('listeners.name')} value={cloneForm.name} onChange={(e) => setCloneForm({ ...cloneForm, name: e.target.value })} sx={{ mt: 1, mb: 2 }} />
          <TextField fullWidth label={t('listeners.port')} value={cloneForm.port} onChange={(e) => setCloneForm({ ...cloneForm, port: e.target.value })} />
        </DialogContent>
        <DialogActions>
          <Button onClick={() => setCloneSrc(null)}>{t('common.cancel')}</Button>
          <Button variant="contained" onClick={async () => {
            if (!cloneSrc) return
            try { await cloneListener(cloneSrc.id, cloneForm); setCloneSrc(null); load(); setOk('Cloned') } catch (e: any) { setError(e.message) }
          }}>{t('common.save')}</Button>
        </DialogActions>
      </Dialog>

      {/* Save template */}
      <Dialog open={!!tplSrc} onClose={() => setTplSrc(null)}>
        <DialogTitle>{t('listeners.templateName') !== 'listeners.templateName' ? t('listeners.templateName') : 'Save as template'}</DialogTitle>
        <DialogContent>
          <TextField fullWidth label={t('listeners.templateName') !== 'listeners.templateName' ? t('listeners.templateName') : 'Name'} value={tplName} onChange={(e) => setTplName(e.target.value)} sx={{ mt: 1 }} />
        </DialogContent>
        <DialogActions>
          <Button onClick={() => setTplSrc(null)}>{t('common.cancel')}</Button>
          <Button variant="contained" onClick={async () => {
            if (!tplSrc) return
            try {
              await createListenerTemplate({ name: tplName, protocol: tplSrc.protocol, config: tplSrc.config })
              setTplSrc(null); loadTemplates(); setOk('Template saved')
            } catch (e: any) { setError(e.message) }
          }}>{t('common.save')}</Button>
        </DialogActions>
      </Dialog>

      {/* Instantiate */}
      <Dialog open={!!instSrc} onClose={() => setInstSrc(null)}>
        <DialogTitle>{t('listeners.instantiate') !== 'listeners.instantiate' ? t('listeners.instantiate') : 'Instantiate'}</DialogTitle>
        <DialogContent>
          <TextField fullWidth label={t('listeners.name')} value={instForm.name} onChange={(e) => setInstForm({ ...instForm, name: e.target.value })} sx={{ mt: 1, mb: 2 }} />
          <TextField fullWidth label={t('listeners.newPort') !== 'listeners.newPort' ? t('listeners.newPort') : 'Port'} value={instForm.port} onChange={(e) => setInstForm({ ...instForm, port: e.target.value })} />
        </DialogContent>
        <DialogActions>
          <Button onClick={() => setInstSrc(null)}>{t('common.cancel')}</Button>
          <Button variant="contained" onClick={async () => {
            if (!instSrc) return
            try { await instantiateListenerTemplate(instSrc.id, instForm); setInstSrc(null); load(); setOk('Created from template') } catch (e: any) { setError(e.message) }
          }}>{t('common.save')}</Button>
        </DialogActions>
      </Dialog>

      {/* Versions */}
      <Dialog open={!!verListener} onClose={() => setVerListener(null)} fullWidth maxWidth="md">
        <DialogTitle>{t('listeners.versions') !== 'listeners.versions' ? t('listeners.versions') : 'Versions'} — {verListener?.name}</DialogTitle>
        <DialogContent>
          <Table size="small">
            <TableHead>
              <TableRow>
                <TableCell>{t('listeners.version') !== 'listeners.version' ? t('listeners.version') : 'Version'}</TableCell>
                <TableCell>{t('listeners.reason') !== 'listeners.reason' ? t('listeners.reason') : 'Reason'}</TableCell>
                <TableCell>{t('listeners.createdAt') !== 'listeners.createdAt' ? t('listeners.createdAt') : 'Created'}</TableCell>
                <TableCell />
              </TableRow>
            </TableHead>
            <TableBody>
              {versions.map((v) => (
                <TableRow key={v.id}>
                  <TableCell>{v.version}</TableCell>
                  <TableCell>{v.reason || '—'}</TableCell>
                  <TableCell>{new Date(v.created_at).toLocaleString()}</TableCell>
                  <TableCell>
                    <Button size="small" onClick={() => verListener && diffListenerVersion(verListener.id, v.version).then((d) => { setDiffText(typeof d === 'string' ? d : JSON.stringify(d, null, 2)); setDiffOpen(true) }).catch((e) => setError(e.message))}>Diff</Button>
                    <Button size="small" onClick={() => verListener && rollbackListenerVersion(verListener.id, v.version).then(() => { setVerListener(null); load(); setOk('Rolled back') }).catch((e) => setError(e.message))}>Rollback</Button>
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </DialogContent>
      </Dialog>

      <Dialog open={diffOpen} onClose={() => setDiffOpen(false)} fullWidth maxWidth="md">
        <DialogTitle>Diff</DialogTitle>
        <DialogContent>
          <pre style={{ maxHeight: '65vh', overflow: 'auto', whiteSpace: 'pre-wrap', wordBreak: 'break-word', margin: 0 }}>{diffText || t('common.empty')}</pre>
        </DialogContent>
      </Dialog>
    </Box>
  )
}
