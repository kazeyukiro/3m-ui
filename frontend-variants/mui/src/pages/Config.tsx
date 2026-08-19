import { useEffect, useState } from 'react'
import { Box, Typography, Button, Stack, Alert } from '@mui/material'
import Editor from '@monaco-editor/react'
import { fetchConfigYAML, validateConfigYAML, generateConfig } from '@shared/api/config'
import { useI18n } from '@shared/i18n'

export default function ConfigPage() {
  const { t } = useI18n()
  const [yaml, setYaml] = useState('')
  const [error, setError] = useState('')
  const [ok, setOk] = useState('')
  const load = async () => {
    try {
      const d = await fetchConfigYAML()
      setYaml(d?.config || '')
    } catch (e: any) { setError(e.message) }
  }
  useEffect(() => { load() }, [])
  return (
    <Box>
      <Typography variant="h5" fontWeight={700} sx={{ mb: 2 }}>{t('config.title')}</Typography>
      {error && <Alert severity="error" sx={{ mb: 2 }} onClose={() => setError('')}>{error}</Alert>}
      {ok && <Alert severity="success" sx={{ mb: 2 }} onClose={() => setOk('')}>{ok}</Alert>}
      <Stack direction="row" spacing={1} sx={{ mb: 2 }}>
        <Button variant="contained" onClick={async () => {
          try { await generateConfig(); await load(); setOk('Generated') } catch (e: any) { setError(e.message) }
        }}>{t('config.generate')}</Button>
        <Button onClick={async () => {
          try {
            const r = await validateConfigYAML(yaml)
            if (r?.valid === false) setError(r?.error || 'Invalid')
            else { setError(''); setOk('Valid') }
          } catch (e: any) { setError(e.message) }
        }}>{t('config.validate')}</Button>
        <Button onClick={load}>{t('common.refresh')}</Button>
      </Stack>
      <Box sx={{ border: 1, borderColor: 'divider', borderRadius: 1, overflow: 'hidden', height: 560 }}>
        <Editor language="yaml" value={yaml} onChange={(v) => setYaml(v || '')} options={{ minimap: { enabled: false }, fontSize: 13 }} />
      </Box>
    </Box>
  )
}
