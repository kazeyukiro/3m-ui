import React, { createContext, useContext, useState, useCallback } from 'react';
import en from './locales/en';
import zh from './locales/zh';

type Locale = 'en' | 'zh';
type Translations = Record<string, any>;

const translations: Record<Locale, Translations> = { en, zh };

function resolveKey(dict: Translations | undefined, key: string): string | undefined {
  if (!dict || !key) return undefined;
  const parts = key.split('.');
  let value: any = dict;
  for (const k of parts) {
    if (value && typeof value === 'object' && k in value) {
      value = value[k];
    } else {
      return undefined;
    }
  }
  return typeof value === 'string' ? value : undefined;
}

interface I18nContextType {
  locale: Locale;
  /** Resolve a dotted key. Falls back to English, then optional fallback, then the key itself. */
  t: (key: string, fallback?: string) => string;
  setLocale: (locale: Locale) => void;
}

const I18nContext = createContext<I18nContextType | null>(null);

export const I18nProvider: React.FC<{ children: React.ReactNode }> = ({ children }) => {
  const [locale, setLocaleState] = useState<Locale>(() => {
    const saved = localStorage.getItem('3m-ui-locale');
    if (saved === 'en' || saved === 'zh') return saved;
    return navigator.language.toLowerCase().startsWith('zh') ? 'zh' : 'en';
  });

  const setLocale = useCallback((l: Locale) => {
    localStorage.setItem('3m-ui-locale', l);
    setLocaleState(l);
  }, []);

  const t = useCallback(
    (key: string, fallback?: string) => {
      return (
        resolveKey(translations[locale], key) ??
        (locale !== 'en' ? resolveKey(translations.en, key) : undefined) ??
        fallback ??
        key
      );
    },
    [locale],
  );

  return (
    <I18nContext.Provider value={{ locale, t, setLocale }}>
      {children}
    </I18nContext.Provider>
  );
};

export const useI18n = () => {
  const ctx = useContext(I18nContext);
  if (!ctx) throw new Error('useI18n must be used within I18nProvider');
  return ctx;
};
