import { useEffect, useState } from 'react'
import { Title, Card, Text, Group, Button, Badge, Alert, Stack } from '@mantine/core'
import { IconPlayerPlay, IconPlayerStop, IconRefresh } from '@tabler/icons-react'
import { fetchDashboard, startMihomo, stopMihomo, restartMihomo } from '@shared/api/system'
import { useI18n } from '@shared/i18n'

export default function Core() {
  const { t } = useI18n()
  const [data, setData] = useState<any>()
  const [error, setError] = useState('')
  const [busy, setBusy] = useState(false)
  const load = async () => { try { setData(await fetchDashboard()) } catch (e: any) { setError(e.message) } }
  useEffect(() => { load() }, [])
  const act = async (fn: () => Promise<any>) => { setBusy(true); try { await fn(); await load() } catch (e: any) { setError(e.message) } finally { setBusy(false) } }
  const m = data?.mihomo
  return (
    <Stack>
      <Title order={2}>{t('core.title')}</Title>
      {error && <Alert color="red">{error}</Alert>}
      <Card withBorder>
        <Group mb="md"><Badge color={m?.running ? 'green' : 'gray'}>{m?.running ? t('core.running') : t('core.stopped')}</Badge><Text>{m?.version} · PID {m?.pid} · {m?.uptime}</Text></Group>
        <Group>
          <Button leftSection={<IconPlayerPlay size={16} />} loading={busy} onClick={() => act(startMihomo)}>{t('core.start')}</Button>
          <Button variant="light" leftSection={<IconRefresh size={16} />} loading={busy} onClick={() => act(restartMihomo)}>{t('core.restart')}</Button>
          <Button color="red" variant="light" leftSection={<IconPlayerStop size={16} />} loading={busy} onClick={() => act(stopMihomo)}>{t('core.stop')}</Button>
        </Group>
      </Card>
    </Stack>
  )
}
