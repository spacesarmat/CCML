export type AppTheme = 'midnight' | 'graphite' | 'ocean' | 'light'

export const APP_THEMES: AppTheme[] = ['midnight', 'graphite', 'ocean', 'light']

const STORAGE_KEY = 'ccml.theme.v1'
const DEFAULT_THEME: AppTheme = 'midnight'

export function loadTheme(): AppTheme {
  if (typeof window === 'undefined') return DEFAULT_THEME
  try {
    const stored = window.localStorage.getItem(STORAGE_KEY)
    return isAppTheme(stored) ? stored : DEFAULT_THEME
  } catch {
    return DEFAULT_THEME
  }
}

export function saveTheme(theme: AppTheme): void {
  if (typeof window === 'undefined') return
  try {
    window.localStorage.setItem(STORAGE_KEY, theme)
  } catch {
    // Theme remains valid for the current session when storage is unavailable.
  }
}

export function applyTheme(theme: AppTheme): void {
  if (typeof document === 'undefined') return
  document.documentElement.dataset.ccmlTheme = theme
  document.documentElement.style.colorScheme = theme === 'light' ? 'light' : 'dark'
}

export function isAppTheme(value: unknown): value is AppTheme {
  return typeof value === 'string' && APP_THEMES.includes(value as AppTheme)
}
