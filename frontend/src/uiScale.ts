export type AppUIScale = 80 | 90 | 100 | 110 | 125

export const UI_SCALES: AppUIScale[] = [80, 90, 100, 110, 125]

const STORAGE_KEY = 'ccml.ui-scale.v1'
const DEFAULT_SCALE: AppUIScale = 100

export function loadUIScale(): AppUIScale {
  if (typeof window === 'undefined') return DEFAULT_SCALE
  try {
    const raw = window.localStorage.getItem(STORAGE_KEY)
    const value = raw === null ? NaN : Number(raw)
    return isAppUIScale(value) ? value : DEFAULT_SCALE
  } catch {
    return DEFAULT_SCALE
  }
}

export function saveUIScale(scale: AppUIScale): void {
  if (typeof window === 'undefined') return
  try {
    window.localStorage.setItem(STORAGE_KEY, String(scale))
  } catch {
    // Scaling still works for this session if localStorage is unavailable.
  }
}

export function applyUIScale(scale: AppUIScale): void {
  if (typeof document === 'undefined') return
  document.documentElement.dataset.ccmlScale = String(scale)
}

export function isAppUIScale(value: unknown): value is AppUIScale {
  return typeof value === 'number' && UI_SCALES.includes(value as AppUIScale)
}
