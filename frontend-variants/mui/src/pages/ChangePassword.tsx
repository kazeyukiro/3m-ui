import { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { Box, Card, CardContent, TextField, Button, Typography, Alert } from '@mui/material'
import { changePassword } from '@shared/api/auth'
import { useI18n } from '@shared/i18n'

export default function ChangePassword() {
  const { t } = useI18n()
  const navigate = useNavigate()
  const [oldPassword, setOld] = useState('')
  const [newPassword, setNew] = useState('')
  const [confirm, setConfirm] = useState('')
  const [error, setError] = useState('')
  const [loading, setLoading] = useState(false)

  const onSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    if (newPassword !== confirm) {
      setError('Passwords do not match')
      return
    }
    setLoading(true)
    setError('')
    try {
      await changePassword(oldPassword, newPassword)
      navigate('/', { replace: true })
    } catch (err: any) {
      setError(err.message)
    } finally {
      setLoading(false)
    }
  }

  return (
    <Box sx={{ minHeight: '100vh', display: 'grid', placeItems: 'center', p: 2 }}>
      <Card sx={{ width: '100%', maxWidth: 420 }}>
        <CardContent sx={{ p: 4 }}>
          <Typography variant="h5" fontWeight={700} sx={{ mb: 2 }}>
            {t('password.title') !== 'password.title' ? t('password.title') : 'Change Password'}
          </Typography>
          {error && <Alert severity="error" sx={{ mb: 2 }}>{error}</Alert>}
          <Box component="form" onSubmit={onSubmit}>
            <TextField fullWidth type="password" label="Current password" value={oldPassword} onChange={(e) => setOld(e.target.value)} sx={{ mb: 2 }} required />
            <TextField fullWidth type="password" label="New password" value={newPassword} onChange={(e) => setNew(e.target.value)} sx={{ mb: 2 }} required />
            <TextField fullWidth type="password" label="Confirm" value={confirm} onChange={(e) => setConfirm(e.target.value)} sx={{ mb: 3 }} required />
            <Button fullWidth type="submit" variant="contained" disabled={loading}>Save</Button>
          </Box>
        </CardContent>
      </Card>
    </Box>
  )
}
