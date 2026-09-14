import { useEffect, useState } from 'react'
import { translate, type AppLanguage, type TranslationKey } from './i18n'
import {APP_THEMES, type AppTheme} from './theme'
import {UI_SCALES, type AppUIScale} from './uiScale'
import type { MetadataProviderReport, MetadataSettings, SystemStatus } from './types'

type Props = {
  language: AppLanguage
  theme: AppTheme
  uiScale: AppUIScale
  open: boolean
  status: SystemStatus | null
  onClose: () => void
  onSaved: (settings: MetadataSettings) => Promise<void> | void
  onMessage: (message: string) => void
  onThemeChange: (theme: AppTheme) => void
  onUIScaleChange: (scale: AppUIScale) => void
}

function SettingsModal({language, theme, uiScale, open, status, onClose, onSaved, onMessage, onThemeChange, onUIScaleChange}: Props) {
  const t = (key: Parameters<typeof translate>[1], params?: Parameters<typeof translate>[2]) => translate(language, key, params)
  const [settings, setSettings] = useState<MetadataSettings | null>(null)
  const [loading, setLoading] = useState(false)
  const [saving, setSaving] = useState(false)
  const [error, setError] = useState('')
  const [testing, setTesting] = useState(false)
  const [providerHealth, setProviderHealth] = useState<Record<string, MetadataProviderReport>>({})

  useEffect(() => {
    if (!open) return
    let cancelled = false
    setLoading(true)
    setError('')
    void window.go.main.App.GetMetadataSettings()
      .then((value) => {
        if (!cancelled) setSettings(value)
      })
      .catch((err) => {
        if (!cancelled) setError(err instanceof Error ? err.message : String(err))
      })
      .finally(() => {
        if (!cancelled) setLoading(false)
      })
    return () => { cancelled = true }
  }, [open])

  if (!open) return null

  function change<K extends keyof MetadataSettings>(key: K, value: MetadataSettings[K]) {
    setSettings((current) => current ? {...current, [key]: value} : current)
  }

  async function openProviderPage(key: string) {
    setError('')
    try {
      await window.go.main.App.OpenMetadataLink(key)
    } catch (err) {
      setError(t('settings.openProviderFailed', {error: err instanceof Error ? err.message : String(err)}))
    }
  }


  async function testProviders() {
    if (!settings) return
    setTesting(true)
    setError('')
    try {
      const reports = await window.go.main.App.TestMetadataProviders(settings)
      const next: Record<string, MetadataProviderReport> = {}
      for (const report of reports ?? []) next[report.name] = report
      setProviderHealth(next)
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err))
    } finally {
      setTesting(false)
    }
  }

  async function save() {
    if (!settings) return
    setSaving(true)
    setError('')
    try {
      const saved = await window.go.main.App.SaveMetadataSettings(settings)
      setSettings(saved)
      await onSaved(saved)
      const refreshed = await window.go.main.App.SystemStatus()
      onMessage(t('settings.saved', {count: refreshed.metadataProviders?.length ?? 0}))
      onClose()
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err))
    } finally {
      setSaving(false)
    }
  }

  return (
    <div className="settings-backdrop" role="presentation" onMouseDown={(event) => event.target === event.currentTarget && onClose()}>
      <section className="settings-modal panel" role="dialog" aria-modal="true" aria-label={t('settings.title')}>
        <div className="settings-header">
          <div>
            <h2>{t('settings.title')}</h2>
            <span>{t('settings.activeProviders', {count: status?.metadataProviders?.length ?? 0})}</span>
          </div>
          <button onClick={onClose} disabled={saving}>×</button>
        </div>

        <div className="settings-body">
          <section className="settings-appearance-section">
            <div className="settings-section-title">
              <h3>{t('settings.appearanceTitle')}</h3>
              <p>{t('settings.appearanceHint')}</p>
              <small>{t('settings.themeAppliesImmediately')}</small>
            </div>

            <div className="theme-choice-grid" role="radiogroup" aria-label={t('settings.theme')}>
              {APP_THEMES.map((themeID) => {
                const nameKey = (`settings.theme.${themeID}`) as TranslationKey
                const descriptionKey = (`settings.theme.${themeID}.description`) as TranslationKey
                return (
                  <button
                    type="button"
                    role="radio"
                    aria-checked={theme === themeID}
                    className={`theme-choice ${theme === themeID ? 'active' : ''}`}
                    key={themeID}
                    onClick={() => onThemeChange(themeID)}
                  >
                    <span className={`theme-choice-preview theme-choice-preview-${themeID}`} aria-hidden="true">
                      <i />
                      <i />
                      <i />
                      <i />
                    </span>
                    <span className="theme-choice-copy">
                      <strong>{t(nameKey)}</strong>
                      <small>{t(descriptionKey)}</small>
                    </span>
                    <b aria-hidden="true">{theme === themeID ? '✓' : ''}</b>
                  </button>
                )
              })}
            </div>

            <div className="ui-scale-setting">
              <div className="ui-scale-copy">
                <strong>{t('settings.uiScale')}</strong>
                <small>{t('settings.uiScaleHint')}</small>
              </div>
              <div className="ui-scale-options" role="radiogroup" aria-label={t('settings.uiScale')}>
                {UI_SCALES.map((scale) => (
                  <button
                    type="button"
                    role="radio"
                    aria-checked={uiScale === scale}
                    className={uiScale === scale ? 'active' : ''}
                    key={scale}
                    onClick={() => onUIScaleChange(scale)}
                  >
                    {scale}%
                  </button>
                ))}
              </div>
            </div>
          </section>

          <div className="settings-section-divider" />

          <div className="settings-section-title">
            <h3>{t('settings.metadataTitle')}</h3>
            <p>{t('settings.metadataHint')}</p>
            <small>{t('settings.localStorageHint')}</small>
            <div className="settings-test-row">
              <button type="button" onClick={() => void testProviders()} disabled={testing || saving || loading}>{testing ? t('settings.testingProviders') : t('settings.testProviders')}</button>
              <span>{t('settings.testProvidersHint')}</span>
            </div>
            {settings && (
              <div className="settings-concurrency-row">
                <label>
                  <span>{t('settings.metadataConcurrency')}</span>
                  <input
                    type="number"
                    min={1}
                    max={16}
                    step={1}
                    value={settings.metadataEnrichmentConcurrency}
                    onChange={(event) => {
                      const value = Number(event.target.value)
                      change('metadataEnrichmentConcurrency', Number.isFinite(value) ? Math.min(16, Math.max(1, Math.round(value))) : 10)
                    }}
                  />
                </label>
                <small>{t('settings.metadataConcurrencyHint')}</small>
              </div>
            )}
          </div>

          {loading && <p className="settings-state">{t('settings.loading')}</p>}
          {error && <p className="warning">{error}</p>}

          {settings && (
            <div className="provider-settings-grid">
              <ProviderCard language={language} title="MusicBrainz" health={providerHealth['MusicBrainz']} description={t('settings.musicBrainzDescription')} enabled={settings.musicBrainzEnabled} onEnabled={(v) => change('musicBrainzEnabled', v)} badge={t('settings.freeSource')} helpLabel={t('settings.documentation')} onHelp={() => void openProviderPage('musicbrainz')} />
              <ProviderCard language={language} title="Deezer" health={providerHealth['Deezer']} description={t('settings.deezerDescription')} enabled={settings.deezerEnabled} onEnabled={(v) => change('deezerEnabled', v)} badge={t('settings.freeSource')} helpLabel={t('settings.documentation')} onHelp={() => void openProviderPage('deezer')} />

              <ProviderCard language={language} title="Apple iTunes" health={providerHealth['Apple iTunes']} description={t('settings.iTunesDescription')} enabled={settings.iTunesEnabled} onEnabled={(v) => change('iTunesEnabled', v)} badge={t('settings.freeSource')} helpLabel={t('settings.documentation')} onHelp={() => void openProviderPage('itunes')}>
                <SettingInput label={t('settings.iTunesCountry')} value={settings.iTunesCountry} onChange={(v) => change('iTunesCountry', v)} placeholder="US" />
              </ProviderCard>

              <ProviderCard language={language} title="TheAudioDB" health={providerHealth['TheAudioDB']} description={t('settings.theAudioDBDescription')} enabled={settings.theAudioDBEnabled} onEnabled={(v) => change('theAudioDBEnabled', v)} badge={t('settings.freeSource')} helpLabel={t('settings.getKey')} onHelp={() => void openProviderPage('theaudiodb')}>
                <SettingInput label={t('settings.theAudioDBKey')} value={settings.theAudioDBApiKey} onChange={(v) => change('theAudioDBApiKey', v)} placeholder="123" password />
              </ProviderCard>

              <ProviderCard language={language} title="Yandex Music" health={providerHealth['Yandex Music']} description={t('settings.yandexMusicDescription')} enabled={settings.yandexMusicEnabled} onEnabled={(v) => change('yandexMusicEnabled', v)} badge={t('settings.experimentalSource')} helpLabel={t('settings.openYandexOAuth')} onHelp={() => void openProviderPage('yandexmusic')}>
                <SettingInput label={t('settings.yandexMusicToken')} value={settings.yandexMusicToken} onChange={(v) => change('yandexMusicToken', v)} password placeholder={t('settings.optional')} />
                <SettingInput label={t('settings.yandexMusicLanguage')} value={settings.yandexMusicLanguage} onChange={(v) => change('yandexMusicLanguage', v)} placeholder="ru" />
              </ProviderCard>

              <ProviderCard language={language} title="Traxsource" health={providerHealth['Traxsource']} description={t('settings.traxsourceDescription')} enabled={settings.traxsourceEnabled} onEnabled={(v) => change('traxsourceEnabled', v)} badge={t('settings.experimentalSource')} helpLabel={t('settings.traxsourceBridge')} onHelp={() => void openProviderPage('traxsourcebridge')}>
                <SettingInput label={t('settings.traxsourceApiKey')} value={settings.traxsourceApiKey} onChange={(v) => change('traxsourceApiKey', v)} password placeholder={t('settings.optional')} />
              </ProviderCard>

              <ProviderCard language={language} title="MUZVIZOR" health={providerHealth['MUZVIZOR']} description={t('settings.muzvizorDescription')} enabled={settings.muzvizorEnabled ?? false} onEnabled={(v) => change('muzvizorEnabled', v)} badge={t('settings.djPoolSource')} helpLabel={t('settings.termsAndWebsite')} onHelp={() => void openProviderPage('muzvizor')} />

              <ProviderCard language={language} title="RemixPool" health={providerHealth['RemixPool']} description={t('settings.remixPoolDescription')} enabled={settings.remixPoolEnabled ?? false} onEnabled={(v) => change('remixPoolEnabled', v)} badge={t('settings.djPoolSource')} helpLabel={t('settings.termsAndWebsite')} onHelp={() => void openProviderPage('remixpool')} />

              <ProviderCard language={language} title="Bananastreet" health={providerHealth['Bananastreet']} description={t('settings.bananaStreetDescription')} enabled={settings.bananaStreetEnabled ?? false} onEnabled={(v) => change('bananaStreetEnabled', v)} badge={t('settings.djPoolSource')} helpLabel={t('settings.termsAndWebsite')} onHelp={() => void openProviderPage('bananastreet')} />

              <ProviderCard language={language} title="Mixcloud" health={providerHealth['Mixcloud']} description={t('settings.mixcloudDescription')} enabled={settings.mixcloudEnabled ?? false} onEnabled={(v) => change('mixcloudEnabled', v)} badge={t('settings.djPoolSource')} helpLabel={t('settings.documentation')} onHelp={() => void openProviderPage('mixcloud')} />

              <ProviderCard language={language} title="Jestei Pool" health={providerHealth['Jestei Pool']} description={t('settings.jesteiDescription')} enabled={settings.jesteiEnabled ?? false} onEnabled={(v) => change('jesteiEnabled', v)} badge={t('settings.djPoolSource')} helpLabel={t('settings.termsAndWebsite')} onHelp={() => void openProviderPage('jestei')} />

              <ProviderCard language={language} title="Discogs" health={providerHealth['Discogs']} description={t('settings.discogsDescription')} enabled={settings.discogsEnabled} onEnabled={(v) => change('discogsEnabled', v)} badge={t('settings.credentialsRequired')} helpLabel={t('settings.getCredentials')} onHelp={() => void openProviderPage('discogs')}>
                <SettingInput label={t('settings.discogsToken')} value={settings.discogsToken} onChange={(v) => change('discogsToken', v)} password />
              </ProviderCard>

              <ProviderCard language={language} title="Spotify" health={providerHealth['Spotify']} description={t('settings.spotifyDescription')} enabled={settings.spotifyEnabled} onEnabled={(v) => change('spotifyEnabled', v)} badge={t('settings.credentialsRequired')} helpLabel={t('settings.openDeveloperDashboard')} onHelp={() => void openProviderPage('spotify')}>
                <SettingInput label={t('settings.spotifyClientId')} value={settings.spotifyClientId} onChange={(v) => change('spotifyClientId', v)} />
                <SettingInput label={t('settings.spotifyClientSecret')} value={settings.spotifyClientSecret} onChange={(v) => change('spotifyClientSecret', v)} password />
                <SettingInput label={t('settings.spotifyAccessToken')} value={settings.spotifyAccessToken} onChange={(v) => change('spotifyAccessToken', v)} password />
                <SettingInput label={t('settings.spotifyMarket')} value={settings.spotifyMarket} onChange={(v) => change('spotifyMarket', v)} placeholder="US" />
              </ProviderCard>

              <ProviderCard language={language} title="Apple Music" health={providerHealth['Apple Music']} description={t('settings.appleMusicDescription')} enabled={settings.appleMusicEnabled} onEnabled={(v) => change('appleMusicEnabled', v)} badge={t('settings.credentialsRequired')} helpLabel={t('settings.getCredentials')} onHelp={() => void openProviderPage('applemusic')}>
                <SettingInput label={t('settings.appleMusicToken')} value={settings.appleMusicDeveloperToken} onChange={(v) => change('appleMusicDeveloperToken', v)} password />
                <SettingInput label={t('settings.appleMusicStorefront')} value={settings.appleMusicStorefront} onChange={(v) => change('appleMusicStorefront', v)} placeholder="us" />
              </ProviderCard>

              <ProviderCard language={language} title="YouTube" health={providerHealth['YouTube']} description={t('settings.youtubeDescription')} enabled={settings.youTubeEnabled} onEnabled={(v) => change('youTubeEnabled', v)} badge={t('settings.credentialsRequired')} helpLabel={t('settings.getCredentials')} onHelp={() => void openProviderPage('youtube')}>
                <SettingInput label={t('settings.youtubeKey')} value={settings.youTubeApiKey} onChange={(v) => change('youTubeApiKey', v)} password />
              </ProviderCard>

              <ProviderCard language={language} title="SoundCloud" health={providerHealth['SoundCloud']} description={t('settings.soundCloudDescription')} enabled={settings.soundCloudEnabled} onEnabled={(v) => change('soundCloudEnabled', v)} badge={t('settings.credentialsRequired')} helpLabel={t('settings.registerApplication')} onHelp={() => void openProviderPage('soundcloud')}>
                <SettingInput label={t('settings.soundCloudToken')} value={settings.soundCloudAccessToken} onChange={(v) => change('soundCloudAccessToken', v)} password />
              </ProviderCard>
            </div>
          )}
        </div>

        <div className="settings-footer">
          <button onClick={onClose} disabled={saving}>{t('settings.cancel')}</button>
          <button className="primary" onClick={() => void save()} disabled={!settings || loading || saving}>{saving ? t('settings.saving') : t('settings.save')}</button>
        </div>
      </section>
    </div>
  )
}

function ProviderCard({language, title, description, enabled, onEnabled, badge, helpLabel, onHelp, health, children}: {
  language: AppLanguage
  title: string
  description: string
  enabled: boolean
  onEnabled: (enabled: boolean) => void
  badge: string
  helpLabel?: string
  onHelp?: () => void
  health?: MetadataProviderReport
  children?: any
}) {
  const t = (key: Parameters<typeof translate>[1], params?: Parameters<typeof translate>[2]) => translate(language, key, params)
  return (
    <article className={`provider-setting ${enabled ? 'enabled' : ''}`}>
      <div className="provider-setting-head">
        <div><strong>{title}</strong><small>{badge}</small></div>
        <label className="provider-toggle"><input type="checkbox" checked={enabled} onChange={(e) => onEnabled(e.target.checked)} /> <span /></label>
      </div>
      <p>{description}</p>
      {health && (
        <div className={`provider-health ${health.status}`}>
          <strong>{health.status === 'ok' ? '✓' : health.status === 'empty' ? '○' : '!'}</strong>
          <span>{health.status === 'ok' ? t('settings.providerHealthOk', {count: health.candidates, time: (health.durationMs / 1000).toFixed(1)}) : health.status === 'empty' ? t('settings.providerHealthEmpty', {time: (health.durationMs / 1000).toFixed(1)}) : (health.error || t('settings.providerHealthError'))}</span>
        </div>
      )}
      {children && <div className="provider-setting-fields">{children}</div>}
      {onHelp && helpLabel && <div className="provider-setting-links"><button type="button" onClick={onHelp}>{helpLabel} ↗</button></div>}
    </article>
  )
}

function SettingInput({label, value, onChange, placeholder, password = false}: {
  label: string
  value: string
  onChange: (value: string) => void
  placeholder?: string
  password?: boolean
}) {
  return <label><span>{label}</span><input type={password ? 'password' : 'text'} value={value} onChange={(e) => onChange(e.target.value)} placeholder={placeholder} autoComplete="off" /></label>
}

export default SettingsModal
