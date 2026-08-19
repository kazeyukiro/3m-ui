import { useEffect, useState } from 'react'
import { Title, Button, Group, Alert, Stack, Box } from '@mantine/core'
import Editor from '@monaco-editor/react'
import { fetchConfigYAML, validateConfigYAML, generateConfig } from '@shared/api/config'
import { useI18n } from '@shared/i18n'

export default function ConfigPage() {
  const { t } = useI18n()
  const [yaml, setYaml] = useState('')
  const [error, setError] = useState('')
  const [ok, setOk] = useState('')
  const load = async () => { try { setYaml((await fetchConfigYAML())?.config || '') } catch (e: any) { setError(e.message) } }
  useEffect(() => { load() }, [])
  return (
    <Stack>
      <Title order={2}>{t('config.title')}</Title>
      {error && <Alert color="red" onClose={() => setError('')}>{error}</Alert>}
      {ok && <Alert color="green" onClose={() => setOk('')}>{ok}</Alert>}
      <Group>
        <Button onClick={async () => { try { await generateConfig(); await load(); setOk('Generated') } catch (e: any) { setError(e.message) } }}>{t('config.generate')}</Button>
        <Button variant="default" onClick={async () => { try { const r = await validateConfigYAML(yaml); if (r?.valid===false) setError(r.error||'Invalid'); else { setError(''); setOk('Valid') } } catch (e: any) { setError(e.message) } }}>{t('config.validate')}</Button>
        <Button variant="default" onClick={load}>{t('common.refresh')}</Button>
      </Group>
      <Box style={{ border: '1px solid var(--mantine-color-gray-3)', borderRadius: 8, overflow: 'hidden', height: 560 }}>
        <Editor language="yaml" value={yaml} onChange={(v) => setYaml(v||'')} options={{ minimap: { enabled: false }, fontSize: 13 }} />
      </Box>
    </Stack>
  )
}
