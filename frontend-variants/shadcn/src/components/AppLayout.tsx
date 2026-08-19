import { useState } from 'react'
import { useNavigate, useLocation } from 'react-router-dom'
import {
  LayoutDashboard, Network, Users, Activity, Server, Route, Cpu, FileText, Code, Settings, LogOut, Menu, Languages, Moon,
} from 'lucide-react'
import { useAuthStore } from '@shared/stores/authStore'
import { useThemeStore, ThemeMode } from '@shared/stores/themeStore'
import { useI18n } from '@shared/i18n'

const NAV = [
  { path: '/', key: 'dashboard', icon: LayoutDashboard },
  { path: '/listeners', key: 'listeners', icon: Network },
  { path: '/users', key: 'users', icon: Users },
  { path: '/traffic', key: 'traffic', icon: Activity },
  { path: '/cluster', key: 'cluster', icon: Server },
  { path: '/routing', key: 'routing', icon: Route },
  { path: '/core', key: 'core', icon: Cpu },
  { path: '/logs', key: 'logs', icon: FileText },
  { path: '/config', key: 'config', icon: Code },
  { path: '/settings', key: 'settings', icon: Settings },
] as const

export default function AppLayout({ children }: { children: React.ReactNode }) {
  const [open, setOpen] = useState(false)
  const navigate = useNavigate()
  const location = useLocation()
  const { t, locale, setLocale } = useI18n()
  const { mode, setMode } = useThemeStore()
  const username = useAuthStore((s) => s.username)
  const logout = useAuthStore((s) => s.logout)
  const label = (k: string) => { const v = t(`nav.${k}`); return v !== `nav.${k}` ? v : k }

  const side = (
    <aside className={`sidebar ${open ? 'open' : ''}`}>
      <div className="logo">3M-UI</div>
      {NAV.map((n) => {
        const active = n.path === '/' ? location.pathname === '/' : location.pathname.startsWith(n.path)
        return (
          <button key={n.path} className={`nav-item ${active ? 'active' : ''}`} onClick={() => { navigate(n.path); setOpen(false) }}>
            <n.icon size={18} />{label(n.key)}
          </button>
        )
      })}
      <div style={{ flex: 1 }} />
      <button className="nav-item" onClick={() => { logout(); navigate('/login') }}>
        <LogOut size={18} />{t('common.logout') !== 'common.logout' ? t('common.logout') : 'Logout'}
      </button>
    </aside>
  )

  return (
    <div className="shell">
      {side}
      <div className="main">
        <header className="topbar">
          <button className="icon-btn" style={{ marginRight: 'auto' }} onClick={() => setOpen(!open)}><Menu size={20} /></button>
          <button className="btn ghost" onClick={() => setMode((mode === 'dark' ? 'light' : 'dark') as ThemeMode)}><Moon size={16} /> {t('settings.theme')}</button>
          <button className="btn ghost" onClick={() => setLocale(locale === 'zh' ? 'en' : 'zh')}><Languages size={16} /> {locale === 'zh' ? '中文' : 'EN'}</button>
          <span className="badge">{username || 'Admin'}</span>
        </header>
        <div className="content">{children}</div>
      </div>
    </div>
  )
}
