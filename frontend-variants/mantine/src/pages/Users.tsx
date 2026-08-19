import { useEffect, useState } from 'react'
import { Title, Button, Group, Alert, Table, Modal, TextInput, PasswordInput, Switch, Badge, ActionIcon, Stack, Text, Checkbox } from '@mantine/core'
import { IconPlus, IconTrash, IconEdit, IconCopy, IconLink, IconRefresh, IconAffiliate } from '@tabler/icons-react'
import { fetchUsers, createUser, updateUser, deleteUser, resetUserTraffic, fetchUserNodes, bindUserNodes, fetchUserSubscription, ProxyUser } from '@shared/api/users'
import { fetchListeners, Listener } from '@shared/api/nodes'
import { formatBytes } from '@shared/utils/format'
import { useI18n } from '@shared/i18n'

export default function Users() {
  const { t } = useI18n()
  const [rows, setRows] = useState<ProxyUser[]>([])
  const [error, setError] = useState('')
  const [ok, setOk] = useState('')
  const [edit, setEdit] = useState<any>(null)
  const [bindUser, setBindUser] = useState<ProxyUser|null>(null)
  const [allNodes, setAllNodes] = useState<Listener[]>([])
  const [selectedNodeIds, setSelectedNodeIds] = useState<number[]>([])
  const [subInfo, setSubInfo] = useState<any>(null)

  const load = async () => { try { setRows(await fetchUsers()) } catch (e: any) { setError(e.message) } }
  useEffect(() => { load() }, [])

  const save = async () => {
    try {
      const payload: any = {
        username: edit.username, enabled: !!edit.enabled, remark: edit.remark,
        traffic_limit: edit.traffic_limit_gb ? Math.round(Number(edit.traffic_limit_gb)*1024*1024*1024) : 0,
        ip_limit: edit.ip_limit ? Number(edit.ip_limit) : 0, expire_time: edit.expire_time || undefined,
      }
      if (edit.password) payload.password = edit.password
      if (edit.id) await updateUser(edit.id, payload); else await createUser(payload)
      setEdit(null); setOk('Saved'); load()
    } catch (e: any) { setError(e.message) }
  }

  return (
    <Stack>
      <Group justify="space-between"><Title order={2}>{t('users.title')}</Title><Button leftSection={<IconPlus size={16} />} onClick={()=>setEdit({enabled:true})}>{t('common.create')}</Button></Group>
      {error && <Alert color="red" onClose={()=>setError('')}>{error}</Alert>}
      {ok && <Alert color="green" onClose={()=>setOk('')}>{ok}</Alert>}
      <Table>
        <Table.Thead><Table.Tr><Table.Th>{t('users.username')}</Table.Th><Table.Th>{t('users.traffic')}</Table.Th><Table.Th>{t('users.status')}</Table.Th><Table.Th>{t('users.remark')}</Table.Th><Table.Th/></Table.Tr></Table.Thead>
        <Table.Tbody>
          {rows.map((u)=>(
            <Table.Tr key={u.id}>
              <Table.Td><Text fw={600}>{u.username}</Text></Table.Td>
              <Table.Td>{formatBytes(u.traffic_used||0)}{u.traffic_limit?` / ${formatBytes(u.traffic_limit)}`:''}</Table.Td>
              <Table.Td><Badge color={u.enabled?'green':'gray'}>{u.enabled?t('common.enabled'):t('common.disabled')}</Badge>{u.blocked && <Badge color="red" ml={4}>Blocked</Badge>}</Table.Td>
              <Table.Td>{u.remark||'—'}</Table.Td>
              <Table.Td>
                <Group gap={4} justify="flex-end">
                  <ActionIcon variant="subtle" onClick={async ()=>{ setBindUser(u); const [nodes,bound]=await Promise.all([fetchListeners(),fetchUserNodes(u.id)]); setAllNodes(nodes); setSelectedNodeIds((bound||[]).map((n:any)=>n.id)) }}><IconAffiliate size={16}/></ActionIcon>
                  <ActionIcon variant="subtle" onClick={()=>fetchUserSubscription(u.id).then(setSubInfo).catch((e)=>setError(e.message))}><IconLink size={16}/></ActionIcon>
                  <ActionIcon variant="subtle" onClick={()=>resetUserTraffic(u.id).then(()=>{setOk('Reset');load()}).catch((e)=>setError(e.message))}><IconRefresh size={16}/></ActionIcon>
                  {u.sub_token && <ActionIcon variant="subtle" onClick={()=>navigator.clipboard.writeText(u.sub_token||'')}><IconCopy size={16}/></ActionIcon>}
                  <ActionIcon variant="subtle" onClick={()=>setEdit({...u, traffic_limit_gb: u.traffic_limit?(u.traffic_limit/1024/1024/1024).toFixed(2):''})}><IconEdit size={16}/></ActionIcon>
                  <ActionIcon color="red" variant="subtle" onClick={()=>deleteUser(u.id).then(load).catch((e)=>setError(e.message))}><IconTrash size={16}/></ActionIcon>
                </Group>
              </Table.Td>
            </Table.Tr>
          ))}
        </Table.Tbody>
      </Table>
      <Modal opened={!!edit} onClose={()=>setEdit(null)} title={edit?.id?t('common.edit'):t('common.create')}>
        <Stack>
          <TextInput label={t('users.username')} value={edit?.username||''} onChange={(e)=>setEdit({...edit,username:e.currentTarget.value})} />
          <PasswordInput label={t('users.password')} value={edit?.password||''} onChange={(e)=>setEdit({...edit,password:e.currentTarget.value})} />
          <TextInput label={t('users.remark')} value={edit?.remark||''} onChange={(e)=>setEdit({...edit,remark:e.currentTarget.value})} />
          <TextInput label="Traffic limit (GB)" value={edit?.traffic_limit_gb??''} onChange={(e)=>setEdit({...edit,traffic_limit_gb:e.currentTarget.value})} />
          <TextInput label="IP limit" value={edit?.ip_limit??''} onChange={(e)=>setEdit({...edit,ip_limit:e.currentTarget.value})} />
          <Switch label={t('common.enabled')} checked={!!edit?.enabled} onChange={(e)=>setEdit({...edit,enabled:e.currentTarget.checked})} />
          <Button onClick={save}>{t('common.save')}</Button>
        </Stack>
      </Modal>
      <Modal opened={!!bindUser} onClose={()=>setBindUser(null)} title={`Bind — ${bindUser?.username}`}>
        <Stack>
          {allNodes.map((n)=>(
            <Checkbox key={n.id} label={`${n.name} (${n.protocol}:${n.port})`} checked={selectedNodeIds.includes(n.id)}
              onChange={()=>setSelectedNodeIds((ids)=>ids.includes(n.id)?ids.filter(x=>x!==n.id):[...ids,n.id])} />
          ))}
          <Button onClick={async ()=>{ if(!bindUser)return; try{await bindUserNodes(bindUser.id,selectedNodeIds);setBindUser(null);setOk('Bound')}catch(e:any){setError(e.message)}}}>{t('common.save')}</Button>
        </Stack>
      </Modal>
      <Modal opened={!!subInfo} onClose={()=>setSubInfo(null)} title="Subscription">
        <Stack>
          <TextInput label="URL" value={subInfo?.url||''} readOnly />
          <TextInput label="Token" value={subInfo?.token||''} readOnly />
          <Button onClick={()=>subInfo&&navigator.clipboard.writeText(subInfo.url)}>{t('common.copy')}</Button>
        </Stack>
      </Modal>
    </Stack>
  )
}
