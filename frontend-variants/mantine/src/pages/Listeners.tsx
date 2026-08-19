import { useEffect, useState } from 'react'
import {
  Title, Button, Group, Alert, Table, Modal, TextInput, Select, Switch, Badge, ActionIcon, Stack, Text, Divider, Checkbox, Tooltip,
} from '@mantine/core'
import { IconPlus, IconTrash, IconEdit, IconRefresh, IconLink, IconCopy, IconHistory, IconFiles, IconDeviceFloppy } from '@tabler/icons-react'
import { fetchListeners, createListener, updateListener, deleteListener, reloadListener, exportNodeURI, Listener } from '@shared/api/nodes'
import {
  listListenerTemplates, createListenerTemplate, deleteListenerTemplate, instantiateListenerTemplate,
  cloneListener, batchSetListenersEnabled, listListenerVersions, diffListenerVersion, rollbackListenerVersion,
  ListenerTemplate, ListenerVersion,
} from '@shared/api/listeners'
import { fetchCapabilities, protocolCapability, CapabilityManifest } from '@shared/api/capabilities'
import { configToFormValues, formValuesToConfig, protocolSupportsUDP } from '@shared/logic/listenerConfig'
import { capabilityFormToConfig } from '@shared/logic/capabilityForm'
import ListenerConfigFields from '../components/ListenerConfigFields'
import { useI18n } from '@shared/i18n'

const PROTOCOLS = ['shadowsocks','snell','vmess','vless','trojan','hysteria2','tuic','shadowquic','anytls','mieru','sudoku','trusttunnel']
const REALITY = new Set(['vmess','vless','trojan'])

export default function Listeners() {
  const { t } = useI18n()
  const [rows, setRows] = useState<Listener[]>([])
  const [templates, setTemplates] = useState<ListenerTemplate[]>([])
  const [error, setError] = useState('')
  const [ok, setOk] = useState('')
  const [edit, setEdit] = useState<any>(null)
  const [capabilities, setCapabilities] = useState<CapabilityManifest|null>(null)
  const [selected, setSelected] = useState<number[]>([])
  const [uris, setUris] = useState<string[]>([])
  const [uriOpen, setUriOpen] = useState(false)
  const [cloneSrc, setCloneSrc] = useState<Listener|null>(null)
  const [cloneForm, setCloneForm] = useState({ name: '', port: '' })
  const [tplSrc, setTplSrc] = useState<Listener|null>(null)
  const [tplName, setTplName] = useState('')
  const [instSrc, setInstSrc] = useState<ListenerTemplate|null>(null)
  const [instForm, setInstForm] = useState({ name: '', port: '' })
  const [versions, setVersions] = useState<ListenerVersion[]>([])
  const [verListener, setVerListener] = useState<Listener|null>(null)
  const [diffText, setDiffText] = useState('')
  const [diffOpen, setDiffOpen] = useState(false)

  const load = async () => { try { setRows(await fetchListeners()) } catch (e: any) { setError(e.message) } }
  const loadTemplates = async () => { try { setTemplates(await listListenerTemplates()) } catch {} }
  useEffect(() => { load(); loadTemplates(); fetchCapabilities().then(setCapabilities).catch(()=>null) }, [])
  const set = (k: string, v: any) => setEdit((p: any) => ({ ...p, [k]: v }))
  const cap = capabilities && edit?.protocol ? protocolCapability(capabilities, edit.protocol) : undefined

  const save = async () => {
    try {
      if (!edit?.name || !edit?.protocol || !String(edit?.port||'').trim()) { setError('Name/protocol/port required'); return }
      if (REALITY.has(edit.protocol) && (edit.security_layer==='reality'||!edit.security_layer) && (!edit.reality_dest||!edit.reality_private_key)) {
        setError('Reality Dest / Private Key required'); return
      }
      const previous = edit.id ? (()=>{ try{return JSON.parse(edit.config||'{}')}catch{return null}})() : null
      const config = { ...formValuesToConfig(edit.protocol, edit, previous), ...(cap ? capabilityFormToConfig(edit.protocol, edit, cap) : {}) }
      const payload: any = {
        name: edit.name, protocol: edit.protocol, port: String(edit.port), bind_address: edit.bind_address||'0.0.0.0',
        enabled: !!edit.enabled, udp: protocolSupportsUDP(edit.protocol)?!!edit.udp:false, config: JSON.stringify(config),
        public_host: edit.public_host||'', public_port: edit.public_port||'', access_sni: edit.access_sni||'',
        client_fingerprint: edit.client_fingerprint||'', access_alpn: edit.access_alpn||'',
      }
      if (edit.id) await updateListener(edit.id, payload); else await createListener(payload)
      setEdit(null); setOk('Saved'); load()
    } catch (e: any) { setError(e.message) }
  }

  return (
    <Stack>
      <Group justify="space-between">
        <Title order={2}>{t('listeners.title')}</Title>
        <Group>
          {selected.length>0 && <>
            <Button size="xs" variant="light" onClick={()=>batchSetListenersEnabled(selected,true).then(()=>{setSelected([]);load()}).catch((e)=>setError(e.message))}>Enable</Button>
            <Button size="xs" variant="light" onClick={()=>batchSetListenersEnabled(selected,false).then(()=>{setSelected([]);load()}).catch((e)=>setError(e.message))}>Disable</Button>
          </>}
          <Button leftSection={<IconPlus size={16} />} onClick={()=>setEdit({ protocol:'vless', port:'443', bind_address:'0.0.0.0', enabled:true, transport_layer:'raw', security_layer:'reality', flow:'xtls-rprx-vision', client_fingerprint:'chrome' })}>{t('common.create')}</Button>
        </Group>
      </Group>
      {error && <Alert color="red" onClose={()=>setError('')}>{error}</Alert>}
      {ok && <Alert color="green" onClose={()=>setOk('')}>{ok}</Alert>}
      <Table>
        <Table.Thead><Table.Tr><Table.Th/><Table.Th>{t('listeners.name')}</Table.Th><Table.Th>{t('listeners.protocol')}</Table.Th><Table.Th>{t('listeners.port')}</Table.Th><Table.Th>{t('listeners.status')}</Table.Th><Table.Th/></Table.Tr></Table.Thead>
        <Table.Tbody>
          {rows.map((l)=>(
            <Table.Tr key={l.id}>
              <Table.Td><Checkbox checked={selected.includes(l.id)} onChange={()=>setSelected((s)=>s.includes(l.id)?s.filter(x=>x!==l.id):[...s,l.id])} /></Table.Td>
              <Table.Td>{l.name}</Table.Td><Table.Td>{l.protocol}</Table.Td>
              <Table.Td>{l.bind_address||'0.0.0.0'}:{l.port}</Table.Td>
              <Table.Td><Badge color={l.enabled?'green':'gray'}>{l.enabled?t('common.enabled'):t('common.disabled')}</Badge></Table.Td>
              <Table.Td>
                <Group gap={4} justify="flex-end">
                  <ActionIcon variant="subtle" onClick={()=>exportNodeURI(l.id).then((d:any)=>{setUris(d?.uris||d?.links||(d?.uri?[d.uri]:[]));setUriOpen(true)}).catch((e)=>setError(e.message))}><IconLink size={16}/></ActionIcon>
                  <ActionIcon variant="subtle" onClick={()=>reloadListener(l.id).then(load).catch((e)=>setError(e.message))}><IconRefresh size={16}/></ActionIcon>
                  <ActionIcon variant="subtle" onClick={()=>{setCloneSrc(l);setCloneForm({name:l.name+'-copy',port:l.port})}}><IconFiles size={16}/></ActionIcon>
                  <ActionIcon variant="subtle" onClick={()=>{setTplSrc(l);setTplName(l.name)}}><IconDeviceFloppy size={16}/></ActionIcon>
                  <ActionIcon variant="subtle" onClick={()=>listListenerVersions(l.id).then((v)=>{setVersions(v||[]);setVerListener(l)}).catch((e)=>setError(e.message))}><IconHistory size={16}/></ActionIcon>
                  <ActionIcon variant="subtle" onClick={()=>setEdit({...l,...configToFormValues(l.config),public_host:(l as any).public_host||'',public_port:(l as any).public_port||'',access_sni:(l as any).access_sni||'',client_fingerprint:(l as any).client_fingerprint||'chrome',access_alpn:(l as any).access_alpn||''})}><IconEdit size={16}/></ActionIcon>
                  <ActionIcon color="red" variant="subtle" onClick={()=>deleteListener(l.id).then(load).catch((e)=>setError(e.message))}><IconTrash size={16}/></ActionIcon>
                </Group>
              </Table.Td>
            </Table.Tr>
          ))}
        </Table.Tbody>
      </Table>

      {templates.length>0 && (
        <Stack>
          <Title order={4}>Templates</Title>
          {templates.map((tpl)=>(
            <Group key={tpl.id} justify="space-between">
              <Text>{tpl.name} · {tpl.protocol}</Text>
              <Group>
                <Button size="xs" variant="light" onClick={()=>{setInstSrc(tpl);setInstForm({name:tpl.name,port:'443'})}}>Instantiate</Button>
                <ActionIcon color="red" variant="subtle" onClick={()=>deleteListenerTemplate(tpl.id).then(loadTemplates).catch((e)=>setError(e.message))}><IconTrash size={16}/></ActionIcon>
              </Group>
            </Group>
          ))}
        </Stack>
      )}

      <Modal opened={!!edit} onClose={()=>setEdit(null)} title={edit?.id?t('common.edit'):t('common.create')} size="xl">
        <Stack>
          <TextInput label={t('listeners.name')} value={edit?.name||''} onChange={(e)=>set('name',e.currentTarget.value)} required />
          <Select label={t('listeners.protocol')} data={PROTOCOLS} value={edit?.protocol||'vless'} onChange={(v)=>set('protocol',v)} />
          <Group grow>
            <TextInput label={t('listeners.port')} value={edit?.port||''} onChange={(e)=>set('port',e.currentTarget.value)} required />
            <TextInput label="Bind" value={edit?.bind_address||'0.0.0.0'} onChange={(e)=>set('bind_address',e.currentTarget.value)} />
          </Group>
          <Switch label={t('common.enabled')} checked={!!edit?.enabled} onChange={(e)=>set('enabled',e.currentTarget.checked)} />
          {protocolSupportsUDP(edit?.protocol||'') && <Switch label="UDP" checked={!!edit?.udp} onChange={(e)=>set('udp',e.currentTarget.checked)} />}
          <Divider label={t('settings.accessProfile')} />
          <Group grow>
            <TextInput label={t('settings.publicHost')} value={edit?.public_host||''} onChange={(e)=>set('public_host',e.currentTarget.value)} />
            <TextInput label={t('settings.publicPort')} value={edit?.public_port||''} onChange={(e)=>set('public_port',e.currentTarget.value)} />
          </Group>
          <Group grow>
            <TextInput label="SNI" value={edit?.access_sni||''} onChange={(e)=>set('access_sni',e.currentTarget.value)} />
            <TextInput label={t('settings.clientFingerprint')} value={edit?.client_fingerprint||'chrome'} onChange={(e)=>set('client_fingerprint',e.currentTarget.value)} />
          </Group>
          <TextInput label="ALPN" value={edit?.access_alpn||''} onChange={(e)=>set('access_alpn',e.currentTarget.value)} />
          <ListenerConfigFields protocol={edit?.protocol} values={edit||{}} onChange={set} />
          <Button onClick={save}>{t('common.save')}</Button>
        </Stack>
      </Modal>

      <Modal opened={uriOpen} onClose={()=>setUriOpen(false)} title={t('listeners.urisTitle')}>
        <Stack>{uris.length===0?<Text c="dimmed">{t('common.empty')}</Text>:uris.map((u,i)=><TextInput key={i} value={u} readOnly rightSection={<ActionIcon onClick={()=>navigator.clipboard.writeText(u)}><IconCopy size={16}/></ActionIcon>} />)}</Stack>
      </Modal>
      <Modal opened={!!cloneSrc} onClose={()=>setCloneSrc(null)} title="Clone">
        <Stack>
          <TextInput label={t('listeners.name')} value={cloneForm.name} onChange={(e)=>setCloneForm({...cloneForm,name:e.currentTarget.value})} />
          <TextInput label={t('listeners.port')} value={cloneForm.port} onChange={(e)=>setCloneForm({...cloneForm,port:e.currentTarget.value})} />
          <Button onClick={async ()=>{ if(!cloneSrc)return; try{await cloneListener(cloneSrc.id,cloneForm);setCloneSrc(null);load();setOk('Cloned')}catch(e:any){setError(e.message)}}}>{t('common.save')}</Button>
        </Stack>
      </Modal>
      <Modal opened={!!tplSrc} onClose={()=>setTplSrc(null)} title="Template">
        <Stack>
          <TextInput label="Name" value={tplName} onChange={(e)=>setTplName(e.currentTarget.value)} />
          <Button onClick={async ()=>{ if(!tplSrc)return; try{await createListenerTemplate({name:tplName,protocol:tplSrc.protocol,config:tplSrc.config});setTplSrc(null);loadTemplates();setOk('Template saved')}catch(e:any){setError(e.message)}}}>{t('common.save')}</Button>
        </Stack>
      </Modal>
      <Modal opened={!!instSrc} onClose={()=>setInstSrc(null)} title="Instantiate">
        <Stack>
          <TextInput label={t('listeners.name')} value={instForm.name} onChange={(e)=>setInstForm({...instForm,name:e.currentTarget.value})} />
          <TextInput label="Port" value={instForm.port} onChange={(e)=>setInstForm({...instForm,port:e.currentTarget.value})} />
          <Button onClick={async ()=>{ if(!instSrc)return; try{await instantiateListenerTemplate(instSrc.id,instForm);setInstSrc(null);load();setOk('Created')}catch(e:any){setError(e.message)}}}>{t('common.save')}</Button>
        </Stack>
      </Modal>
      <Modal opened={!!verListener} onClose={()=>setVerListener(null)} title={`Versions — ${verListener?.name||''}`} size="lg">
        <Table>
          <Table.Thead><Table.Tr><Table.Th>Ver</Table.Th><Table.Th>Reason</Table.Th><Table.Th>At</Table.Th><Table.Th/></Table.Tr></Table.Thead>
          <Table.Tbody>
            {versions.map((v)=>(
              <Table.Tr key={v.id}>
                <Table.Td>{v.version}</Table.Td><Table.Td>{v.reason||'—'}</Table.Td><Table.Td>{new Date(v.created_at).toLocaleString()}</Table.Td>
                <Table.Td>
                  <Button size="xs" variant="light" onClick={()=>verListener&&diffListenerVersion(verListener.id,v.version).then((d)=>{setDiffText(typeof d==='string'?d:JSON.stringify(d,null,2));setDiffOpen(true)}).catch((e)=>setError(e.message))}>Diff</Button>
                  <Button size="xs" ml="xs" onClick={()=>verListener&&rollbackListenerVersion(verListener.id,v.version).then(()=>{setVerListener(null);load();setOk('Rolled back')}).catch((e)=>setError(e.message))}>Rollback</Button>
                </Table.Td>
              </Table.Tr>
            ))}
          </Table.Tbody>
        </Table>
      </Modal>
      <Modal opened={diffOpen} onClose={()=>setDiffOpen(false)} title="Diff" size="lg">
        <pre style={{maxHeight:'65vh',overflow:'auto',whiteSpace:'pre-wrap'}}>{diffText||t('common.empty')}</pre>
      </Modal>
    </Stack>
  )
}
