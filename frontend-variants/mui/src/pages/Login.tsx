import { useState } from 'react'
import { useNavigate, useLocation } from 'react-router-dom'
import { Box, Card, CardContent, TextField, Button, Typography, Alert } from '@mui/material'
import { login } from '@shared/api/auth'
import { useI18n } from '@shared/i18n'

export default function Login() {
  const navigate = useNavigate()
  const location = useLocation()
  const { t } = useI18n()
  const [username, setUsername] = useState('admin')
  const [password, setPassword] = useState('')
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState('')
  const from = (location.state as any)?.from?.pathname || '/'

  const onSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    setLoading(true)
    setError('')
    try {
      const data = await login({ username, password })
      if ((data as any)?.must_change_password) {
        navigate('/change-password', { replace: true })
      } else {
        navigate(from, { replace: true })
      }
    } catch (err: any) {
      setError(err.message || t('login.failed'))
    } finally {
      setLoading(false)
    }
  }

  return (
    <Box sx={{ minHeight: '100vh', display: 'grid', placeItems: 'center', bgcolor: '#f0f2f5', p: 2 }}>
      <Card sx={{ width: '100%', maxWidth: 420 }}>
        <CardContent sx={{ p: 4 }}>
          <Typography variant="h5" fontWeight={700} align="center">{t('login.title')}</Typography>
          <Typography color="text.secondary" align="center" sx={{ mb: 3 }}>{t('login.subtitle')}</Typography>
          {error && <Alert severity="error" sx={{ mb: 2 }}>{error}</Alert>}
          <Box component="form" onSubmit={onSubmit}>
            <TextField fullWidth label={t('login.username')} value={username} onChange={(e) => setUsername(e.target.value)} sx={{ mb: 2 }} required />
            <TextField fullWidth type="password" label={t('login.password')} value={password} onChange={(e) => setPassword(e.target.value)} sx={{ mb: 3 }} required />
            <Button fullWidth type="submit" variant="contained" size="large" disabled={loading}>{t('login.button')}</Button>
          </Box>
        </CardContent>
      </Card>
    </Box>
  )
}
