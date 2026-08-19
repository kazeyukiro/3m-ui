import { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { Stack, Paper, Title, PasswordInput, Button, Alert } from '@mantine/core'
import { changePassword } from '@shared/api/auth'
import { useI18n } from '@shared/i18n'

export default function ChangePassword() {
  const { t } = useI18n()
  const navigate = useNavigate()
  const [cur, setCur] = useState('')
  const [next, setNext] = useState('')
  const [confirm, setConfirm] = useState('')
  const [err, setErr] = useState('')
  return (
    <Stack justify="center" align="center" h="100vh" p="md">
      <Paper withBorder p="xl" w={400} maw="100%">
        <Title order={3} mb="md">Change Password</Title>
        {err && <Alert color="red" mb="md">{err}</Alert>}
        <PasswordInput label="Current" value={cur} onChange={(e) => setCur(e.currentTarget.value)} mb="sm" />
        <PasswordInput label="New" value={next} onChange={(e) => setNext(e.currentTarget.value)} mb="sm" />
        <PasswordInput label="Confirm" value={confirm} onChange={(e) => setConfirm(e.currentTarget.value)} mb="md" />
        <Button fullWidth onClick={async () => {
          if (next !== confirm) { setErr('Passwords do not match'); return }
          try { await changePassword(cur, next); navigate('/', { replace: true }) } catch (e: any) { setErr(e.message) }
        }}>Save</Button>
      </Paper>
    </Stack>
  )
}
