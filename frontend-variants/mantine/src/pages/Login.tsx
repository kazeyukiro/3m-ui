import { useState } from 'react'
import { useNavigate, useLocation } from 'react-router-dom'
import { Stack, Paper, Title, Text, TextInput, PasswordInput, Button, Alert } from '@mantine/core'
import { login } from '@shared/api/auth'
import { useI18n } from '@shared/i18n'

export default function Login() {
  const { t } = useI18n()
  const navigate = useNavigate()
  const location = useLocation()
  const [u, setU] = useState('admin')
  const [p, setP] = useState('')
  const [err, setErr] = useState('')
  const [loading, setLoading] = useState(false)
  const from = (location.state as any)?.from?.pathname || '/'
  return (
    <Stack justify="center" align="center" h="100vh" bg="gray.0" p="md">
      <Paper withBorder shadow="sm" p="xl" w={400} maw="100%">
        <Title order={2} ta="center">{t('login.title')}</Title>
        <Text c="dimmed" ta="center" mb="lg">{t('login.subtitle')}</Text>
        {err && <Alert color="red" mb="md">{err}</Alert>}
        <form onSubmit={async (e) => {
          e.preventDefault(); setLoading(true); setErr('')
          try { const data = await login({ username: u, password: p }); navigate(data.must_change_password ? '/change-password' : from, { replace: true }) }
          catch (ex: any) { setErr(ex.message) } finally { setLoading(false) }
        }}>
          <TextInput label={t('login.username')} value={u} onChange={(e) => setU(e.currentTarget.value)} mb="sm" required />
          <PasswordInput label={t('login.password')} value={p} onChange={(e) => setP(e.currentTarget.value)} mb="lg" required />
          <Button type="submit" fullWidth loading={loading}>{t('login.button')}</Button>
        </form>
      </Paper>
    </Stack>
  )
}
