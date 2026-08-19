import { BrowserRouter, Routes, Route, Navigate } from 'react-router-dom'
import { I18nProvider } from '@shared/i18n'
import AppLayout from './components/AppLayout'
import ProtectedRoute from './components/ProtectedRoute'
import Login from './pages/Login'
import ChangePassword from './pages/ChangePassword'
import Dashboard from './pages/Dashboard'
import Listeners from './pages/Listeners'
import Users from './pages/Users'
import Core from './pages/Core'
import Logs from './pages/Logs'
import ConfigPage from './pages/Config'
import Settings from './pages/Settings'
import TrafficPage from './pages/Traffic'
import ClusterPage from './pages/Cluster'
import RoutingPage from './pages/Routing'

export default function App() {
  return (
    <I18nProvider>
      <BrowserRouter>
        <Routes>
          <Route path="/login" element={<Login />} />
          <Route path="/change-password" element={<ChangePassword />} />
          <Route path="/*" element={
            <ProtectedRoute>
              <AppLayout>
                <Routes>
                  <Route path="/" element={<Dashboard />} />
                  <Route path="/listeners" element={<Listeners />} />
                  <Route path="/users" element={<Users />} />
                  <Route path="/traffic" element={<TrafficPage />} />
                  <Route path="/cluster" element={<ClusterPage />} />
                  <Route path="/routing" element={<RoutingPage />} />
                  <Route path="/core" element={<Core />} />
                  <Route path="/logs" element={<Logs />} />
                  <Route path="/config" element={<ConfigPage />} />
                  <Route path="/settings" element={<Settings />} />
                  <Route path="*" element={<Navigate to="/" replace />} />
                </Routes>
              </AppLayout>
            </ProtectedRoute>
          } />
        </Routes>
      </BrowserRouter>
    </I18nProvider>
  )
}
