import { useEffect, useState } from 'react'
import { Box, Typography, Button, Stack, Alert, TextField, Paper } from '@mui/material'
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
      setGroups(g || [])
      setRulesText((r || []).join('\n'))
    } catch (e: any) { setError(e.message) }
  }
  useEffect(() => { load() }, [])

  return (
    <Box>
      <Typography variant="h5" fontWeight={700} sx={{ mb: 1 }}>{t('routing.title')}</Typography>
      <Typography color="text.secondary" sx={{ mb: 2 }}>{t('routing.subtitle')}</Typography>
      {error && <Alert severity="error" sx={{ mb: 2 }} onClose={() => setError('')}>{error}</Alert>}
      {ok && <Alert severity="success" sx={{ mb: 2 }} onClose={() => setOk('')}>{ok}</Alert>}
      <Paper sx={{ p: 2, mb: 2 }}>
        <Typography variant="h6" sx={{ mb: 1 }}>{t('routing.groups')}</Typography>
        <TextField
          fullWidth multiline minRows={8}
          value={JSON.stringify(groups, null, 2)}
          onChange={(e) => { try { setGroups(JSON.parse(e.target.value)) } catch { /* keep typing */ } }}
          inputProps={{ style: { fontFamily: 'ui-monospace, monospace', fontSize: 13 } }}
        />
        <Button sx={{ mt: 1 }} variant="contained" onClick={async () => {
          try { await saveGroups(groups); setOk(t('routing.groupSaved')); load() } catch (e: any) { setError(e.message) }
        }}>{t('common.save')}</Button>
      </Paper>
      <Paper sx={{ p: 2 }}>
        <Typography variant="h6" sx={{ mb: 1 }}>{t('routing.rules')}</Typography>
        <Typography variant="caption" color="text.secondary" display="block" sx={{ mb: 1 }}>{t('routing.rulesHint')}</Typography>
        <TextField
          fullWidth multiline minRows={12}
          value={rulesText}
          onChange={(e) => setRulesText(e.target.value)}
          inputProps={{ style: { fontFamily: 'ui-monospace, monospace', fontSize: 13 } }}
        />
        <Button sx={{ mt: 1 }} variant="contained" onClick={async () => {
          try {
            const rules = rulesText.split('\n').map((s) => s.trim()).filter(Boolean)
            await saveRules(rules)
            setOk(t('routing.rulesSaved'))
            load()
          } catch (e: any) { setError(e.message) }
        }}>{t('common.save')}</Button>
      </Paper>
    </Box>
  )
}
