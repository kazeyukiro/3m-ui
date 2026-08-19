import { useEffect, useState } from 'react'
import {
  Box, Drawer, AppBar, Toolbar, IconButton, Typography, List, ListItemButton,
  ListItemIcon, ListItemText, Divider, Menu, MenuItem, Chip, useMediaQuery, useTheme,
} from '@mui/material'
import MenuIcon from '@mui/icons-material/Menu'
import DashboardIcon from '@mui/icons-material/Dashboard'
import HubIcon from '@mui/icons-material/Hub'
import PeopleIcon from '@mui/icons-material/People'
import TimelineIcon from '@mui/icons-material/Timeline'
import DnsIcon from '@mui/icons-material/Dns'
import RouteIcon from '@mui/icons-material/Route'
import MemoryIcon from '@mui/icons-material/Memory'
import ArticleIcon from '@mui/icons-material/Article'
import CodeIcon from '@mui/icons-material/Code'
import SettingsIcon from '@mui/icons-material/Settings'
import LogoutIcon from '@mui/icons-material/Logout'
import TranslateIcon from '@mui/icons-material/Translate'
import Brightness6Icon from '@mui/icons-material/Brightness6'
import { useNavigate, useLocation } from 'react-router-dom'
import { useAuthStore } from '@shared/stores/authStore'
import { useThemeStore, ThemeMode } from '@shared/stores/themeStore'
import { useI18n } from '@shared/i18n'

const DRAWER = 240

const NAV = [
  { path: '/', key: 'dashboard', icon: DashboardIcon },
  { path: '/listeners', key: 'listeners', icon: HubIcon },
  { path: '/users', key: 'users', icon: PeopleIcon },
  { path: '/traffic', key: 'traffic', icon: TimelineIcon },
  { path: '/cluster', key: 'cluster', icon: DnsIcon },
  { path: '/routing', key: 'routing', icon: RouteIcon },
  { path: '/core', key: 'core', icon: MemoryIcon },
  { path: '/logs', key: 'logs', icon: ArticleIcon },
  { path: '/config', key: 'config', icon: CodeIcon },
  { path: '/settings', key: 'settings', icon: SettingsIcon },
] as const

function NavList({ onNavigate }: { onNavigate?: () => void }) {
  const navigate = useNavigate()
  const location = useLocation()
  const { t } = useI18n()
  const logout = useAuthStore((s) => s.logout)

  return (
    <Box sx={{ display: 'flex', flexDirection: 'column', height: '100%' }}>
      <Toolbar>
        <Typography variant="h6" fontWeight={800}>3M-UI</Typography>
      </Toolbar>
      <Divider />
      <List sx={{ flex: 1, py: 1 }}>
        {NAV.map((item) => {
          const Icon = item.icon
          const active = item.path === '/' ? location.pathname === '/' : location.pathname.startsWith(item.path)
          return (
            <ListItemButton
              key={item.path}
              selected={active}
              onClick={() => { navigate(item.path); onNavigate?.() }}
            >
              <ListItemIcon sx={{ minWidth: 40 }}><Icon fontSize="small" /></ListItemIcon>
              <ListItemText primary={t(`nav.${item.key}`) !== `nav.${item.key}` ? t(`nav.${item.key}`) : item.key} />
            </ListItemButton>
          )
        })}
      </List>
      <Divider />
      <List>
        <ListItemButton onClick={() => { logout(); navigate('/login') }}>
          <ListItemIcon sx={{ minWidth: 40 }}><LogoutIcon fontSize="small" /></ListItemIcon>
          <ListItemText primary={t('common.logout') !== 'common.logout' ? t('common.logout') : 'Logout'} />
        </ListItemButton>
      </List>
    </Box>
  )
}

export default function AppLayout({ children }: { children: React.ReactNode }) {
  const theme = useTheme()
  const isMobile = useMediaQuery(theme.breakpoints.down('md'))
  const [mobileOpen, setMobileOpen] = useState(false)
  const { t, locale, setLocale } = useI18n()
  const { mode, setMode } = useThemeStore()
  const username = useAuthStore((s) => s.username)
  const [langAnchor, setLangAnchor] = useState<null | HTMLElement>(null)
  const [themeAnchor, setThemeAnchor] = useState<null | HTMLElement>(null)

  useEffect(() => {
    if (!isMobile) setMobileOpen(false)
  }, [isMobile])

  const drawer = <NavList onNavigate={() => setMobileOpen(false)} />

  return (
    <Box sx={{ display: 'flex', minHeight: '100vh' }}>
      <AppBar position="fixed" color="inherit" elevation={0} sx={{ borderBottom: 1, borderColor: 'divider', zIndex: (t) => t.zIndex.drawer + 1 }}>
        <Toolbar>
          {isMobile && (
            <IconButton edge="start" onClick={() => setMobileOpen(true)} sx={{ mr: 1 }}>
              <MenuIcon />
            </IconButton>
          )}
          <Box sx={{ flex: 1 }} />
          <IconButton onClick={(e) => setThemeAnchor(e.currentTarget)}><Brightness6Icon /></IconButton>
          <Menu anchorEl={themeAnchor} open={!!themeAnchor} onClose={() => setThemeAnchor(null)}>
            {(['light', 'dark', 'system'] as ThemeMode[]).map((m) => (
              <MenuItem key={m} selected={mode === m} onClick={() => { setMode(m); setThemeAnchor(null) }}>
                {t(`settings.${m}`) !== `settings.${m}` ? t(`settings.${m}`) : m}
              </MenuItem>
            ))}
          </Menu>
          <IconButton onClick={(e) => setLangAnchor(e.currentTarget)}><TranslateIcon /></IconButton>
          <Menu anchorEl={langAnchor} open={!!langAnchor} onClose={() => setLangAnchor(null)}>
            <MenuItem selected={locale === 'zh'} onClick={() => { setLocale('zh'); setLangAnchor(null) }}>中文</MenuItem>
            <MenuItem selected={locale === 'en'} onClick={() => { setLocale('en'); setLangAnchor(null) }}>English</MenuItem>
          </Menu>
          <Chip label={username || 'Admin'} size="small" sx={{ ml: 1 }} />
        </Toolbar>
      </AppBar>

      <Box component="nav" sx={{ width: { md: DRAWER }, flexShrink: { md: 0 } }}>
        {isMobile ? (
          <Drawer open={mobileOpen} onClose={() => setMobileOpen(false)} ModalProps={{ keepMounted: true }} sx={{ '& .MuiDrawer-paper': { width: DRAWER } }}>
            {drawer}
          </Drawer>
        ) : (
          <Drawer variant="permanent" open sx={{ '& .MuiDrawer-paper': { width: DRAWER, boxSizing: 'border-box' } }}>
            {drawer}
          </Drawer>
        )}
      </Box>

      <Box component="main" sx={{ flexGrow: 1, p: { xs: 1.5, md: 3 }, mt: 8, minWidth: 0 }}>
        <Box sx={{ bgcolor: 'background.paper', borderRadius: 2, p: { xs: 1.5, md: 3 }, minHeight: '70vh' }}>
          {children}
        </Box>
      </Box>
    </Box>
  )
}
