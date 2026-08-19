import { useEffect, useState } from 'react'
import { Title, Text, SimpleGrid, Card, Button, Group, Progress, Alert, Stack } from '@mantine/core'
import { IconPlayerPlay, IconPlayerStop, IconRefresh } from '@tabler/icons-react'
import { fetchDashboard, startMihomo, stopMihomo, restartMihomo } from '@shared/api/system'
import { formatBytes } from '@shared/utils/format'
import { useI18n } from '@shared/i18n'

export default function Dashboard() {
  const { t } = useI18n()
  const [data, setData] = useState<any>()
  const [error, setError] = useState('')
  const [busy, setBusy] = useState(false)
  const load = async () => { try { setData(await fetchDashboard()) } catch (e: any) { setError(e.message) } }
  useEffect(() => { load(); const id = setInterval(load, 5000); return () => clearInterval(id) }, [])
  const act = async (fn: () => Promise<any>) => { setBusy(true); try { await fn(); await load() } catch (e: any) { setError(e.message) } finally { setBusy(false) } }
  const sys = data?.system || {}
  const m = data?.mihomo || {}
  return (
    <Stack>
      <Title order={2}>{t('dashboard.title')}</Title>
      {error && <Alert color="red" onClose={() => setError('')}>{error}</Alert>}
      <SimpleGrid cols={{ base: 1, sm: 3 }}>
        <Card withBorder>
          <Text fw={600} mb="xs">{t('dashboard.mihomoStatus')}</Text>
          <Text size="sm" mb="md">{m.running ? 'Running' : 'Stopped'} · {m.version || '—'} · PID {m.pid || '—'}</Text>
          <Group>
            <Button leftSection={<IconPlayerPlay size={16} />} loading={busy} onClick={() => act(startMihomo)}>{t('dashboard.start')}</Button>
            <Button variant="light" leftSection={<IconRefresh size={16} />} loading={busy} onClick={() => act(restartMihomo)}>{t('dashboard.restart')}</Button>
            <Button color="red" variant="light" leftSection={<IconPlayerStop size={16} />} loading={busy} onClick={() => act(stopMihomo)}>{t('dashboard.stop')}</Button>
          </Group>
        </Card>
        <Card withBorder>
          <Text fw={600} mb="xs">{t('dashboard.listeners')}</Text>
          <Group grow>
            <div><Text size="xl" fw={700}>{data?.listeners?.total || 0}</Text><Text size="xs" c="dimmed">{t('dashboard.total')}</Text></div>
            <div><Text size="xl" fw={700} c="green">{data?.listeners?.enabled || 0}</Text><Text size="xs" c="dimmed">{t('dashboard.enabled')}</Text></div>
            <div><Text size="xl" fw={700} c="red">{data?.listeners?.disabled || 0}</Text><Text size="xs" c="dimmed">{t('dashboard.disabled')}</Text></div>
          </Group>
        </Card>
        <Card withBorder>
          <Text fw={600} mb="xs">{t('dashboard.traffic')}</Text>
          <Text size="sm">↑ {formatBytes(data?.traffic?.uploadRate || 0)}/s · ↓ {formatBytes(data?.traffic?.downloadRate || 0)}/s</Text>
          <Text size="sm">{t('dashboard.onlineUsers')}: {data?.traffic?.onlineUsers || 0}</Text>
        </Card>
      </SimpleGrid>
      <SimpleGrid cols={{ base: 1, sm: 3 }}>
        <Card withBorder><Text size="sm">{t('dashboard.cpu')} {sys.cpu?.percent || 0}%</Text><Progress value={sys.cpu?.percent || 0} mt="xs" /></Card>
        <Card withBorder><Text size="sm">{t('dashboard.memory')} {sys.memory?.percent || 0}%</Text><Progress value={sys.memory?.percent || 0} mt="xs" color={(sys.memory?.percent||0)>90?'red':'blue'} /><Text size="xs" c="dimmed">{formatBytes(sys.memory?.used||0)} / {formatBytes(sys.memory?.total||0)}</Text></Card>
        <Card withBorder><Text size="sm">{t('dashboard.disk')} {sys.disk?.percent || 0}%</Text><Progress value={sys.disk?.percent || 0} mt="xs" /><Text size="xs" c="dimmed">{formatBytes(sys.disk?.used||0)} / {formatBytes(sys.disk?.total||0)}</Text></Card>
      </SimpleGrid>
    </Stack>
  )
}
