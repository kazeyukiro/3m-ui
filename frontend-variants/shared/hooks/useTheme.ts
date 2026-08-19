import { useState, useEffect } from 'react';

type Theme = 'light' | 'dark';

export function useTheme() {
  const [theme, setTheme] = useState<Theme>(() => {
    const saved = localStorage.getItem('3m-ui-theme');
    return (saved as Theme) || 'light';
  });

  useEffect(() => {
    localStorage.setItem('3m-ui-theme', theme);
    document.documentElement.setAttribute('data-theme', theme);
  }, [theme]);

  const toggle = () => setTheme((prev) => (prev === 'light' ? 'dark' : 'light'));

  return { theme, toggle, isDark: theme === 'dark' };
}
