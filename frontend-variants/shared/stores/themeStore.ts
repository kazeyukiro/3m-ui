import { create } from 'zustand';
import { persist, createJSONStorage } from 'zustand/middleware';

export type ThemeMode = 'light' | 'dark' | 'system';

interface ThemeState {
  mode: ThemeMode;
  isDark: boolean;
  setMode: (mode: ThemeMode) => void;
  syncSystemTheme: () => void;
}

const getSystemDark = () =>
  typeof window !== 'undefined' && window.matchMedia('(prefers-color-scheme: dark)').matches;

export const useThemeStore = create<ThemeState>()(
  persist(
    (set, get) => ({
      mode: 'system',
      isDark: getSystemDark(),
      setMode: (mode) => {
        const isDark = mode === 'dark' || (mode === 'system' && getSystemDark());
        set({ mode, isDark });
      },
      syncSystemTheme: () => {
        if (get().mode === 'system') {
          set({ isDark: getSystemDark() });
        }
      },
    }),
    {
      name: '3m-ui-theme',
      storage: createJSONStorage(() => localStorage),
    }
  )
);

if (typeof window !== 'undefined') {
  window.matchMedia('(prefers-color-scheme: dark)').addEventListener('change', () => {
    useThemeStore.getState().syncSystemTheme();
  });
}
