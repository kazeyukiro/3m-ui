import { useState } from 'react'
import { AppShell, NavLink, Group, Text, Button, Menu, Burger, Badge } from '@mantine/core'
import { useDisclosure, useMediaQuery } from '@mantine/hooks'
import {
  IconDashboard, IconNetwork, IconUsers, IconChartLine, IconServer, IconRoute,
  IconCpu, IconFileText, IconCode, IconSettings, IconLogout, IconLanguage, IconMoon,
} from '@tabler/icons-react'
import { useNavigate, useLocation } from 'react-router-dom'
import { useAuthStore } from '@shared/stores/authStore'
import { useThemeStore, ThemeMode } from '@shared/stores/themeStore'
import { useI18n } from '@shared/i18n'

const NAV = [
  { path: '/', key: 'dashboard', icon: IconDashboard },
  { path: '/listeners', key: 'listeners', icon: IconNetwork },
  { path: '/users', key: 'users', icon: IconUsers },
  { path: '/traffic', key: 'traffic', icon: IconChartLine },
  { path: '/cluster', key: 'cluster', icon: IconServer },
  { path: '/routing', key: 'routing', icon: IconRoute },
  { path: '/core', key: 'core', icon: IconCpu },
  { path: '/logs', key: 'logs', icon: IconFileText },
  { path: '/config', key: 'config', icon: IconCode },
  { path: '/settings', key: 'settings', icon: IconSettings },
] as const

export default function AppLayout({ children }: { children: React.ReactNode }) {
  const [opened, { toggle, close }] = useDisclosure()
  const isMobile = useMediaQuery('(max-width: 768px)')
  const navigate = useNavigate()
  const location = useLocation()
  const { t, locale, setLocale } = useI18n()
  const { mode, setMode } = useThemeStore()
  const username = useAuthStore((s) => s.username)
  const logout = useAuthStore((s) => s.logout)

  const label = (key: string) => {
    const v = t(`nav.${key}`)
    return v !== `nav.${key}` ? v : key
  }

  return (
    <AppShell
      header={{ height: 56 }}
      navbar={{ width: 240, breakpoint: 'sm', collapsed: { mobile: !opened } }}
      padding="md"
    >
      <AppShell.Header px="md">
        <Group h="100%" justify="space-between">
          <Group>
            {isMobile && <Burger opened={opened} onClick={toggle} size="sm" />}
            <Text fw={700}>3M-UI</Text>
          </Group>
          <Group gap="xs">
            <Menu>
              <Menu.Target><Button variant="subtle" size="compact-sm" leftSection={<IconMoon size={16} />}>{t('settings.theme')}</Button></Menu.Target>
              <Menu.Dropdown>
                {(['light', 'dark', 'system'] as ThemeMode[]).map((m) => (
                  <Menu.Item key={m} onClick={() => setMode(m)}>{t(`settings.${m}`)}</Menu.Item>
                ))}
              </Menu.Dropdown>
            </Menu>
            <Menu>
              <Menu.Target><Button variant="subtle" size="compact-sm" leftSection={<IconLanguage size={16} />}>{locale === 'zh' ? '中文' : 'EN'}</Button></Menu.Target>
              <Menu.Dropdown>
                <Menu.Item onClick={() => setLocale('zh')}>中文</Menu.Item>
                <Menu.Item onClick={() => setLocale('en')}>English</Menu.Item>
              </Menu.Dropdown>
            </Menu>
            <Badge variant="light">{username || 'Admin'}</Badge>
          </Group>
        </Group>
      </AppShell.Header>
      <AppShell.Navbar p="md">
        {NAV.map((n) => (
          <NavLink
            key={n.path}
            label={label(n.key)}
            leftSection={<n.icon size={18} />}
            active={n.path === '/' ? location.pathname === '/' : location.pathname.startsWith(n.path)}
            onClick={() => { navigate(n.path); close() }}
          />
        ))}
        <NavLink
          mt="auto"
          label={t('common.logout') !== 'common.logout' ? t('common.logout') : 'Logout'}
          leftSection={<IconLogout size={18} />}
          onClick={() => { logout(); navigate('/login') }}
        />
      </AppShell.Navbar>
      <AppShell.Main>{children}</AppShell.Main>
    </AppShell>
  )
}
