import { useEffect, useState } from 'react'
import { Title, Button, Group, Alert, Table, Modal, TextInput, PasswordInput, Badge, ActionIcon, Stack } from '@mantine/core'
import { IconPlus, IconTrash, IconHeartRateMonitor } from '@tabler/icons-react'
import { fetchCluster, createClusterNode, updateClusterNode, deleteClusterNode, healthClusterNode, RemoteServer } from '@shared/api/cluster'
import { useI18n } from '@shared/i18n'

export default function ClusterPage() {
  const { t } = useI18n()
  const [rows, setRows] = useState<RemoteServer[]>([])
  const [error, setError] = useState('')
  const [edit, setEdit] = useState<any>(null)
  const load = async () => { try { setRows(await fetchCluster()) } catch (e: any) { setError(e.message) } }
  useEffect(() => { load() }, [])
  return (
    <Stack>
      <Group justify="space-between"><Title order={2}>{t('cluster.title')}</Title><Button leftSection={<IconPlus size={16} />} onClick={() => setEdit({ enabled: true })}>{t('common.create')}</Button></Group>
      {error && <Alert color="red" onClose={() => setError('')}>{error}</Alert>}
      <Table>
        <Table.Thead><Table.Tr><Table.Th>{t('cluster.name')}</Table.Th><Table.Th>{t('cluster.baseUrl')}</Table.Th><Table.Th>{t('cluster.status')}</Table.Th><Table.Th /></Table.Tr></Table.Thead>
        <Table.Tbody>
          {rows.map((r) => (
            <Table.Tr key={r.id}>
              <Table.Td>{r.name}</Table.Td><Table.Td>{r.base_url}</Table.Td>
              <Table.Td><Badge>{r.last_status || (r.enabled?'enabled':'disabled')}</Badge></Table.Td>
              <Table.Td>
                <Group gap={4} justify="flex-end">
                  <ActionIcon variant="subtle" onClick={() => healthClusterNode(r.id).then(load).catch((e)=>setError(e.message))}><IconHeartRateMonitor size={16} /></ActionIcon>
                  <ActionIcon color="red" variant="subtle" onClick={() => deleteClusterNode(r.id).then(load).catch((e)=>setError(e.message))}><IconTrash size={16} /></ActionIcon>
                </Group>
              </Table.Td>
            </Table.Tr>
          ))}
        </Table.Tbody>
      </Table>
      <Modal opened={!!edit} onClose={() => setEdit(null)} title={edit?.id?t('common.edit'):t('common.create')}>
        <Stack>
          <TextInput label={t('cluster.name')} value={edit?.name||''} onChange={(e)=>setEdit({...edit,name:e.currentTarget.value})} />
          <TextInput label={t('cluster.baseUrl')} value={edit?.base_url||''} onChange={(e)=>setEdit({...edit,base_url:e.currentTarget.value})} />
          <PasswordInput label={t('cluster.apiToken')} value={edit?.api_token||''} onChange={(e)=>setEdit({...edit,api_token:e.currentTarget.value})} />
          <Button onClick={async () => { try { if (edit.id) await updateClusterNode(edit.id, edit); else await createClusterNode(edit); setEdit(null); load() } catch(e:any){ setError(e.message) } }}>{t('common.save')}</Button>
        </Stack>
      </Modal>
    </Stack>
  )
}
