import React, { createContext, useContext, useState, useCallback } from 'react';
import en from './locales/en';
import zh_CN from './locales/zh-CN';
import zh_TW from './locales/zh-TW';
import ja from './locales/ja';
import ko from './locales/ko';
import es from './locales/es';
import fr from './locales/fr';
import de from './locales/de';
import ru from './locales/ru';
import pt_BR from './locales/pt-BR';
import vi from './locales/vi';
import id from './locales/id';
import th from './locales/th';
import tr from './locales/tr';
import ar from './locales/ar';
import hi from './locales/hi';
import pl from './locales/pl';
import uk from './locales/uk';

export type Locale = 'en' | 'zh-CN' | 'zh-TW' | 'ja' | 'ko' | 'es' | 'fr' | 'de' | 'ru' | 'pt-BR' | 'vi' | 'id' | 'th' | 'tr' | 'ar' | 'hi' | 'pl' | 'uk';
type Translations = Record<string, any>;

const translations: Record<Locale, Translations> = {
  'en': en,
  'zh-CN': zh_CN,
  'zh-TW': zh_TW,
  'ja': ja,
  'ko': ko,
  'es': es,
  'fr': fr,
  'de': de,
  'ru': ru,
  'pt-BR': pt_BR,
  'vi': vi,
  'id': id,
  'th': th,
  'tr': tr,
  'ar': ar,
  'hi': hi,
  'pl': pl,
  'uk': uk,
};

/** Legacy localStorage value `zh` maps to zh-CN. */
const LEGACY: Record<string, Locale> = { zh: 'zh-CN', 'zh-cn': 'zh-CN', 'zh-tw': 'zh-TW', 'pt-br': 'pt-BR', pt: 'pt-BR' };

export const LOCALE_OPTIONS: { key: Locale; label: string }[] = [
  { key: 'en' as Locale, label: 'English' },
  { key: 'zh-CN' as Locale, label: '简体中文' },
  { key: 'zh-TW' as Locale, label: '繁體中文' },
  { key: 'ja' as Locale, label: '日本語' },
  { key: 'ko' as Locale, label: '한국어' },
  { key: 'es' as Locale, label: 'Español' },
  { key: 'fr' as Locale, label: 'Français' },
  { key: 'de' as Locale, label: 'Deutsch' },
  { key: 'ru' as Locale, label: 'Русский' },
  { key: 'pt-BR' as Locale, label: 'Português' },
  { key: 'vi' as Locale, label: 'Tiếng Việt' },
  { key: 'id' as Locale, label: 'Bahasa Indonesia' },
  { key: 'th' as Locale, label: 'ไทย' },
  { key: 'tr' as Locale, label: 'Türkçe' },
  { key: 'ar' as Locale, label: 'العربية' },
  { key: 'hi' as Locale, label: 'हिन्दी' },
  { key: 'pl' as Locale, label: 'Polski' },
  { key: 'uk' as Locale, label: 'Українська' },
];

function resolveKey(dict: Translations | undefined, key: string): string | undefined {
  if (!dict || !key) return undefined;
  const parts = key.split('.');
  let value: any = dict;
  for (const k of parts) {
    if (value && typeof value === 'object' && k in value) value = value[k];
    else return undefined;
  }
  return typeof value === 'string' ? value : undefined;
}

function normalizeLocale(raw: string | null | undefined): Locale | null {
  if (!raw) return null;
  const s = raw.trim();
  if ((translations as any)[s]) return s as Locale;
  const lower = s.toLowerCase();
  if (LEGACY[lower]) return LEGACY[lower];
  const short = lower.split('-')[0];
  if (LEGACY[short]) return LEGACY[short];
  for (const opt of LOCALE_OPTIONS) {
    if (opt.key.toLowerCase() === lower || opt.key.toLowerCase().startsWith(short + '-')) return opt.key;
    if (opt.key.toLowerCase().startsWith(short) && short.length >= 2) {
      // prefer exact language match e.g. ja
      if (opt.key.toLowerCase() === short) return opt.key;
    }
  }
  const byShort = LOCALE_OPTIONS.find((o) => o.key.toLowerCase().split('-')[0] === short);
  return byShort ? byShort.key : null;
}

function detectLocale(): Locale {
  const saved = normalizeLocale(localStorage.getItem('3m-ui-locale'));
  if (saved) return saved;
  if (typeof navigator !== 'undefined') {
    const list = navigator.languages?.length ? navigator.languages : [navigator.language];
    for (const l of list) {
      const n = normalizeLocale(l);
      if (n) return n;
    }
  }
  return 'en';
}

interface I18nContextType {
  locale: Locale;
  t: (key: string, fallback?: string) => string;
  setLocale: (locale: Locale) => void;
}

const I18nContext = createContext<I18nContextType | null>(null);

export const I18nProvider: React.FC<{ children: React.ReactNode }> = ({ children }) => {
  const [locale, setLocaleState] = useState<Locale>(() => detectLocale());

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
