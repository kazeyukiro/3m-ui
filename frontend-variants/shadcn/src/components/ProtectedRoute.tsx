import { Navigate, useLocation } from 'react-router-dom'
import { useAuthStore } from '@shared/stores/authStore'

export default function ProtectedRoute({ children }: { children: React.ReactNode }) {
  const location = useLocation()
  const { isAuthenticated, mustChangePassword } = useAuthStore()
  if (!isAuthenticated()) return <Navigate to="/login" state={{ from: location }} replace />
  if (mustChangePassword && location.pathname !== '/change-password') return <Navigate to="/change-password" replace />
  return <>{children}</>
}
