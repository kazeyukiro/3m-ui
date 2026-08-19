import { useEffect, useState } from 'react'
import { Title, Card, Button, Group, Badge, Text, Alert, Stack, ScrollArea } from '@mantine/core'
import dayjs from 'dayjs'
import { fetchLogs } from '@shared/api/system'
import { useI18n } from '@shared/i18n'

export default function Logs() {
  const { t } = useI18n()
  const [logs, setLogs] = useState<any[]>([])
  const [auto, setAuto] = useState(true)
  const [error, setError] = useState('')
  const load = async () => { try { const d = await fetchLogs(); setLogs(Array.isArray(d)?d:[]) } catch (e: any) { setError(e.message) } }
  useEffect(() => { load(); if (!auto) return; const id = setInterval(load, 3000); return () => clearInterval(id) }, [auto])
  return (
    <Stack>
      <Title order={2}>{t('logs.title')}</Title>
      {error && <Alert color="red">{error}</Alert>}
      <Group>
        <Button variant="default" onClick={load}>{t('common.refresh')}</Button>
        <Button variant="default" onClick={() => setLogs([])}>{t('logs.clear')}</Button>
        <Button variant={auto?'filled':'default'} onClick={() => setAuto(!auto)}>{t('logs.autoRefresh')}: {auto?t('common.enabled'):t('common.disabled')}</Button>
      </Group>
      <Card withBorder>
        <ScrollArea h={520}>
          {logs.length===0 ? <Text c="dimmed">{t('logs.empty')}</Text> : logs.map((log,i) => (
            <Text key={i} ff="monospace" size="sm" mb={4}>
              <Text span c="dimmed">[{dayjs(log.timestamp).format('YYYY-MM-DD HH:mm:ss')}] </Text>
              <Badge size="xs" mr={6}>{log.level}</Badge>
              {log.payload}
            </Text>
          ))}
        </ScrollArea>
      </Card>
    </Stack>
  )
}
