import { useEffect, useState } from 'react'
import { Title, Text, Button, Alert, Stack, Textarea, Paper } from '@mantine/core'
import { fetchGroups, saveGroups, fetchRules, saveRules, GroupEntry } from '@shared/api/routing'
import { useI18n } from '@shared/i18n'

export default function RoutingPage() {
  const { t } = useI18n()
  const [groups, setGroups] = useState<GroupEntry[]>([])
  const [rulesText, setRulesText] = useState('')
  const [error, setError] = useState('')
  const [ok, setOk] = useState('')
  const load = async () => {
    try {
      const [g, r] = await Promise.all([fetchGroups(), fetchRules()])
      setGroups(g||[]); setRulesText((r||[]).join('\n'))
    } catch (e: any) { setError(e.message) }
  }
  useEffect(() => { load() }, [])
  return (
    <Stack>
      <Title order={2}>{t('routing.title')}</Title>
      <Text c="dimmed">{t('routing.subtitle')}</Text>
      {error && <Alert color="red" onClose={() => setError('')}>{error}</Alert>}
      {ok && <Alert color="green" onClose={() => setOk('')}>{ok}</Alert>}
      <Paper withBorder p="md">
        <Text fw={600} mb="xs">{t('routing.groups')}</Text>
        <Textarea minRows={8} value={JSON.stringify(groups, null, 2)} onChange={(e) => { try { setGroups(JSON.parse(e.currentTarget.value)) } catch {} }} styles={{ input: { fontFamily: 'monospace' } }} />
        <Button mt="sm" onClick={async () => { try { await saveGroups(groups); setOk(t('routing.groupSaved')); load() } catch(e:any){ setError(e.message) } }}>{t('common.save')}</Button>
      </Paper>
      <Paper withBorder p="md">
        <Text fw={600} mb="xs">{t('routing.rules')}</Text>
        <Text size="xs" c="dimmed" mb="xs">{t('routing.rulesHint')}</Text>
        <Textarea minRows={12} value={rulesText} onChange={(e) => setRulesText(e.currentTarget.value)} styles={{ input: { fontFamily: 'monospace' } }} />
        <Button mt="sm" onClick={async () => { try { await saveRules(rulesText.split('\n').map(s=>s.trim()).filter(Boolean)); setOk(t('routing.rulesSaved')); load() } catch(e:any){ setError(e.message) } }}>{t('common.save')}</Button>
      </Paper>
    </Stack>
  )
}
