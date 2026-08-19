import { useEffect, useState } from 'react'
import { Title, SimpleGrid, Card, Text, Table, Badge, Alert, Stack } from '@mantine/core'
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
      const [s,u,c] = await Promise.all([fetchTrafficStatus(), fetchTrafficUsers(), fetchConnections()])
      setStatus(s); setUsers(u||[]); setConns(c||[])
    } catch (e: any) { setError(e.message) }
  }
  useEffect(() => { load(); const id = setInterval(load, 5000); return () => clearInterval(id) }, [])
  return (
    <Stack>
      <Title order={2}>{t('traffic.title')}</Title>
      {error && <Alert color="red">{error}</Alert>}
      <SimpleGrid cols={{ base: 2, sm: 5 }}>
        {[
          ['Upload', formatBytes(status?.upload_bytes||0)],
          ['Download', formatBytes(status?.download_bytes||0)],
          ['↑', formatBytes(status?.upload_rate||0)+'/s'],
          ['↓', formatBytes(status?.download_rate||0)+'/s'],
          ['Conn', status?.connections ?? conns.length],
        ].map(([a,b]) => <Card key={String(a)} withBorder><Text size="xs" c="dimmed">{a}</Text><Text fw={700}>{b}</Text></Card>)}
      </SimpleGrid>
      <Table>
        <Table.Thead><Table.Tr><Table.Th>User</Table.Th><Table.Th>Used</Table.Th><Table.Th>Limit</Table.Th><Table.Th>Status</Table.Th></Table.Tr></Table.Thead>
        <Table.Tbody>
          {users.map((u) => (
            <Table.Tr key={u.user_id}>
              <Table.Td>{u.username}</Table.Td>
              <Table.Td>{formatBytes(u.traffic_used||0)}</Table.Td>
              <Table.Td>{u.traffic_limit?formatBytes(u.traffic_limit):'∞'}</Table.Td>
              <Table.Td><Badge color={u.online?'green':'gray'}>{u.online?'Online':'Offline'}</Badge></Table.Td>
            </Table.Tr>
          ))}
        </Table.Tbody>
      </Table>
    </Stack>
  )
}
