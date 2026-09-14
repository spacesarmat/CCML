$ErrorActionPreference = "Stop"

$ProjectRoot = "C:\Users\ANDYBUM\GolandProjects\CCML"
$Payload = Join-Path $PSScriptRoot "CCML-stage5.2-traxsource-bridge.go"

$Paths = @{
  Trax = Join-Path $ProjectRoot "internal\metadata\traxsource.go"
  TraxTest = Join-Path $ProjectRoot "internal\metadata\traxsource_test.go"
  Bridge = Join-Path $ProjectRoot "internal\metadata\traxsource_bridge.go"
  Model = Join-Path $ProjectRoot "internal\model\model.go"
  Settings = Join-Path $ProjectRoot "internal\settings\service.go"
  SettingsTest = Join-Path $ProjectRoot "internal\settings\service_test.go"
  App = Join-Path $ProjectRoot "app.go"
  Types = Join-Path $ProjectRoot "frontend\src\types.ts"
  SettingsModal = Join-Path $ProjectRoot "frontend\src\SettingsModal.tsx"
  I18n = Join-Path $ProjectRoot "frontend\src\i18n.ts"
  WailsModels = Join-Path $ProjectRoot "frontend\wailsjs\go\models.ts"
  Help = Join-Path $ProjectRoot "frontend\src\HelpModal.tsx"
}

foreach ($key in @("Trax","TraxTest","Model","Settings","SettingsTest","App","Types","SettingsModal","I18n","WailsModels")) {
  if (-not (Test-Path $Paths[$key])) { throw "Required file not found: $($Paths[$key])" }
}
if (-not (Test-Path $Payload)) { throw "Payload not found: $Payload" }

$Utf8NoBom = New-Object System.Text.UTF8Encoding($false)

function Read-Utf8([string]$Path) {
  return [System.IO.File]::ReadAllText($Path)
}
function Replace-ExactlyOnce {
  param([string]$Text,[string]$Old,[string]$New,[string]$Label)
  $first = $Text.IndexOf($Old,[System.StringComparison]::Ordinal)
  if ($first -lt 0) { throw "Stage 5.2 anchor not found: $Label. No files were changed." }
  $second = $Text.IndexOf($Old,$first+$Old.Length,[System.StringComparison]::Ordinal)
  if ($second -ge 0) { throw "Stage 5.2 anchor ambiguous: $Label. No files were changed." }
  return $Text.Substring(0,$first)+$New+$Text.Substring($first+$Old.Length)
}

$trax = Read-Utf8 $Paths.Trax
$traxTest = Read-Utf8 $Paths.TraxTest
$model = Read-Utf8 $Paths.Model
$settings = Read-Utf8 $Paths.Settings
$settingsTest = Read-Utf8 $Paths.SettingsTest
$app = Read-Utf8 $Paths.App
$types = Read-Utf8 $Paths.Types
$modal = Read-Utf8 $Paths.SettingsModal
$i18n = Read-Utf8 $Paths.I18n
$wmodels = Read-Utf8 $Paths.WailsModels
$help = if (Test-Path $Paths.Help) { Read-Utf8 $Paths.Help } else { $null }

if ($model.Contains('TraxsourceAPIKey') -and (Test-Path $Paths.Bridge)) {
  Write-Host "Stage 5.2 Traxsource resilient search is already installed."
  exit 0
}
if ($model.Contains('TraxsourceAPIKey') -or (Test-Path $Paths.Bridge)) {
  throw "Partial Stage 5.2 installation detected. No files were changed."
}

$trax = Replace-ExactlyOnce $trax @'
type TraxsourceProvider struct {
	client    *http.Client
	baseURL   string
	userAgent string
}

func NewTraxsourceProvider(userAgent string) *TraxsourceProvider {
	return &TraxsourceProvider{
		client: &http.Client{Timeout: 15 * time.Second}, baseURL: traxsourceBaseURL,
		userAgent: strings.TrimSpace(userAgent),
	}
}
'@ @'
type TraxsourceProvider struct {
	client       *http.Client
	baseURL      string
	userAgent    string
	bridgeAPIKey string
	bridgeURL    string
}

func NewTraxsourceProvider(userAgent, bridgeAPIKey string) *TraxsourceProvider {
	return &TraxsourceProvider{
		client:       &http.Client{Timeout: 15 * time.Second},
		baseURL:      traxsourceBaseURL,
		userAgent:    strings.TrimSpace(userAgent),
		bridgeAPIKey: strings.TrimSpace(bridgeAPIKey),
		bridgeURL:    traxsourceBridgeSearchURL,
	}
}
'@ "Traxsource provider constructor"

$trax = Replace-ExactlyOnce $trax @'
func (p *TraxsourceProvider) Search(ctx context.Context, query model.MetadataQuery) ([]model.MetadataCandidate, error) {
'@ @'
func (p *TraxsourceProvider) searchDirect(ctx context.Context, query model.MetadataQuery) ([]model.MetadataCandidate, error) {
'@ "direct search rename"

$model = Replace-ExactlyOnce $model @'
	TraxsourceEnabled  bool `json:"traxsourceEnabled"`

	TheAudioDBAPIKey         string `json:"theAudioDBApiKey"`
'@ @'
	TraxsourceEnabled  bool `json:"traxsourceEnabled"`

	TraxsourceAPIKey         string `json:"traxsourceApiKey"`
	TheAudioDBAPIKey         string `json:"theAudioDBApiKey"`
'@ "Traxsource API key model"

$model = Replace-ExactlyOnce $model @'
	s.YandexMusicLanguage = strings.ToLower(strings.TrimSpace(s.YandexMusicLanguage))

	if s.TheAudioDBAPIKey == "" {
'@ @'
	s.YandexMusicLanguage = strings.ToLower(strings.TrimSpace(s.YandexMusicLanguage))
	s.TraxsourceAPIKey = strings.TrimSpace(s.TraxsourceAPIKey)

	if s.TheAudioDBAPIKey == "" {
'@ "Traxsource API key normalize"

$settings = Replace-ExactlyOnce $settings @'
		TraxsourceEnabled:  false,

		TheAudioDBAPIKey:         strings.TrimSpace(os.Getenv("THEAUDIODB_API_KEY")),
'@ @'
		TraxsourceEnabled:  false,

		TraxsourceAPIKey:         strings.TrimSpace(os.Getenv("TRAXSOURCE_API_KEY")),
		TheAudioDBAPIKey:         strings.TrimSpace(os.Getenv("THEAUDIODB_API_KEY")),
'@ "Traxsource environment key"

$settingsTest = Replace-ExactlyOnce $settingsTest @'
		TraxsourceEnabled:   true,
	}
'@ @'
		TraxsourceEnabled:   true,
		TraxsourceAPIKey:    " trax-fallback-secret ",
	}
'@ "settings test key input"

$settingsTest = Replace-ExactlyOnce $settingsTest @'
	if !got.TraxsourceEnabled {
		t.Fatal("traxsource setting was not preserved")
	}

	info, err := os.Stat(filepath.Join(dir, fileName))
'@ @'
	if !got.TraxsourceEnabled {
		t.Fatal("traxsource setting was not preserved")
	}
	if got.TraxsourceAPIKey != "trax-fallback-secret" {
		t.Fatalf("traxsource fallback key = %q", got.TraxsourceAPIKey)
	}

	info, err := os.Stat(filepath.Join(dir, fileName))
'@ "settings test key assertion"

$app = Replace-ExactlyOnce $app @'
		"traxsource":  "https://www.traxsource.com/terms-of-service",
'@ @'
		"traxsource":       "https://www.traxsource.com/terms-of-service",
		"traxsourcebridge": "https://parse.bot/marketplace/0123da1a-5d2d-4970-bf6f-398f792855a1/traxsource-com-api",
'@ "trusted Traxsource bridge link"

$app = Replace-ExactlyOnce $app @'
	if config.TraxsourceEnabled {
		providers = append(providers, metadata.NewTraxsourceProvider(metadataUserAgent))
	}
'@ @'
	if config.TraxsourceEnabled {
		providers = append(providers, metadata.NewTraxsourceProvider(metadataUserAgent, config.TraxsourceAPIKey))
	}
'@ "Traxsource provider wiring"

$types = Replace-ExactlyOnce $types @'
  traxsourceEnabled: boolean
  theAudioDBApiKey: string
'@ @'
  traxsourceEnabled: boolean
  traxsourceApiKey: string
  theAudioDBApiKey: string
'@ "frontend metadata settings type"

$modal = Replace-ExactlyOnce $modal @'
              <ProviderCard language={language} title="Traxsource" health={providerHealth['Traxsource']} description={t('settings.traxsourceDescription')} enabled={settings.traxsourceEnabled} onEnabled={(v) => change('traxsourceEnabled', v)} badge={t('settings.experimentalSource')} helpLabel={t('settings.termsAndWebsite')} onHelp={() => void openProviderPage('traxsource')} />
'@ @'
              <ProviderCard language={language} title="Traxsource" health={providerHealth['Traxsource']} description={t('settings.traxsourceDescription')} enabled={settings.traxsourceEnabled} onEnabled={(v) => change('traxsourceEnabled', v)} badge={t('settings.experimentalSource')} helpLabel={t('settings.traxsourceBridge')} onHelp={() => void openProviderPage('traxsourcebridge')}>
                <SettingInput label={t('settings.traxsourceApiKey')} value={settings.traxsourceApiKey} onChange={(v) => change('traxsourceApiKey', v)} password placeholder={t('settings.optional')} />
              </ProviderCard>
'@ "Traxsource Settings UI"

$i18n = Replace-ExactlyOnce $i18n @'
  'settings.traxsourceDescription': 'Experimental lookup of the public Traxsource web catalog. Traxsource does not publish a developer/token API for this use; disabled by default.',
  'settings.yandexMusicToken': 'Yandex OAuth token (optional)',
'@ @'
  'settings.traxsourceDescription': 'Direct public-catalog lookup with an optional JSON fallback when Traxsource blocks unattended requests with Cloudflare. The fallback is an independent third-party service and requires its own API key.',
  'settings.traxsourceApiKey': 'Traxsource fallback API key (optional)',
  'settings.traxsourceBridge': 'Fallback API / get key',
  'settings.yandexMusicToken': 'Yandex OAuth token (optional)',
'@ "English Traxsource i18n"

$i18n = Replace-ExactlyOnce $i18n @'
  'settings.traxsourceDescription': 'Экспериментальный поиск по публичному веб-каталогу Traxsource. Публичной developer/token API для этого сценария у Traxsource нет; источник выключен по умолчанию.',
  'settings.yandexMusicToken': 'OAuth-токен Яндекса (необязательно)',
'@ @'
  'settings.traxsourceDescription': 'Прямой поиск по публичному каталогу с опциональным JSON fallback, когда Traxsource блокирует автоматические запросы через Cloudflare. Fallback — независимый сторонний сервис и требует отдельный API-ключ.',
  'settings.traxsourceApiKey': 'API-ключ fallback для Traxsource (необязательно)',
  'settings.traxsourceBridge': 'Fallback API / получить ключ',
  'settings.yandexMusicToken': 'OAuth-токен Яндекса (необязательно)',
'@ "Russian Traxsource i18n"

$wmodels = Replace-ExactlyOnce $wmodels @'
	    traxsourceEnabled: boolean;
	    theAudioDBApiKey: string;
'@ @'
	    traxsourceEnabled: boolean;
	    traxsourceApiKey: string;
	    theAudioDBApiKey: string;
'@ "Wails model property"

$wmodels = Replace-ExactlyOnce $wmodels @'
	        this.traxsourceEnabled = source["traxsourceEnabled"];
	        this.theAudioDBApiKey = source["theAudioDBApiKey"];
'@ @'
	        this.traxsourceEnabled = source["traxsourceEnabled"];
	        this.traxsourceApiKey = source["traxsourceApiKey"];
	        this.theAudioDBApiKey = source["theAudioDBApiKey"];
'@ "Wails model constructor"

$testAppend = @'

func TestTraxsourceBridgeCandidates(t *testing.T) {
	body := []byte(`{"status":"success","data":{"tracks":[{"track_id":"14660000","title":"Magic Carpet","version":"Extended Mix","duration":"6:48","artists":[{"name":"Alex Galvan"}],"label":{"name":"Test Label"},"genre":{"name":"Deep House"},"release_date":"2026-08-01","url":"https://www.traxsource.com/track/14660000/magic-carpet","artwork_url":"https://example.test/cover.jpg"}]}}`)
	items, err := parseTraxsourceBridgeCandidates(body)
	if err != nil {
		t.Fatalf("parse fallback: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("items = %d, want 1", len(items))
	}
	got := items[0]
	if got.Source != "Traxsource" || got.ExternalID != "14660000" || got.Title != "Magic Carpet (Extended Mix)" || got.Artist != "Alex Galvan" {
		t.Fatalf("unexpected candidate: %+v", got)
	}
	if got.Label != "Test Label" || got.Genre != "Deep House" || got.DurationMS != 408000 || got.Year != 2026 {
		t.Fatalf("unexpected fallback fields: %+v", got)
	}
}

func TestTraxsourceFallsBackAfterCloudflareForbidden(t *testing.T) {
	direct := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte("<html><title>Just a moment...</title>Cloudflare human verification</html>"))
	}))
	defer direct.Close()

	bridge := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-API-Key") != "secret" {
			t.Fatalf("missing fallback API key")
		}
		if r.URL.Query().Get("type") != "tracks" {
			t.Fatalf("type = %q", r.URL.Query().Get("type"))
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":{"tracks":[{"track_id":"55","title":"Fallback Song","artists":[{"name":"Fallback Artist"}],"duration":"4:12","url":"https://www.traxsource.com/track/55/fallback-song"}]}}`))
	}))
	defer bridge.Close()

	provider := NewTraxsourceProvider("CCML-test", "secret")
	provider.baseURL = direct.URL
	provider.bridgeURL = bridge.URL
	provider.client = &http.Client{Timeout: time.Second}

	items, err := provider.Search(context.Background(), model.MetadataQuery{Artist: "Fallback Artist", Title: "Fallback Song"})
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}
	if len(items) != 1 || items[0].ExternalID != "55" {
		t.Fatalf("unexpected fallback result: %+v", items)
	}
}
'@

# Add imports needed by the new tests.
$traxTest = Replace-ExactlyOnce $traxTest @'
import (
	"testing"
)
'@ @'
import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/spacesarmat/CCML/internal/model"
)
'@ "Traxsource test imports"
$traxTest += $testAppend

if ($help -ne $null) {
  $ruNeedle = @'
          'Вкладка Метаданные запрашивает включённые источники: например MusicBrainz, Deezer, Apple iTunes и другие настроенные провайдеры.',
'@
  $ruInsert = @'
          'Вкладка Метаданные запрашивает включённые источники: например MusicBrainz, Deezer, Apple iTunes и другие настроенные провайдеры.',
          'Traxsource сначала запрашивается напрямую. Если сайт отвечает Cloudflare/human verification, CCML может автоматически использовать настроенный JSON fallback API; ключ задаётся в Настройки → Traxsource.',
'@
  if ($help.Contains($ruNeedle)) { $help = Replace-ExactlyOnce $help $ruNeedle $ruInsert "Help RU Traxsource fallback" }

  $enNeedle = @'
          'Metadata queries enabled providers such as MusicBrainz, Deezer, Apple iTunes and other configured sources.',
'@
  $enInsert = @'
          'Metadata queries enabled providers such as MusicBrainz, Deezer, Apple iTunes and other configured sources.',
          'Traxsource is tried directly first. If the site responds with Cloudflare/human verification, CCML can automatically use the configured JSON fallback API; set its key under Settings → Traxsource.',
'@
  if ($help.Contains($enNeedle)) { $help = Replace-ExactlyOnce $help $enNeedle $enInsert "Help EN Traxsource fallback" }
}

# Validation has succeeded in memory. Only now create backups and write.
$backupSuffix = ".stage5.2.bak"
foreach ($key in @("Trax","TraxTest","Model","Settings","SettingsTest","App","Types","SettingsModal","I18n","WailsModels")) {
  Copy-Item $Paths[$key] ($Paths[$key] + $backupSuffix) -Force
}
if ($help -ne $null) { Copy-Item $Paths.Help ($Paths.Help + $backupSuffix) -Force }

[System.IO.File]::WriteAllText($Paths.Trax,$trax,$Utf8NoBom)
[System.IO.File]::WriteAllText($Paths.TraxTest,$traxTest,$Utf8NoBom)
[System.IO.File]::WriteAllText($Paths.Model,$model,$Utf8NoBom)
[System.IO.File]::WriteAllText($Paths.Settings,$settings,$Utf8NoBom)
[System.IO.File]::WriteAllText($Paths.SettingsTest,$settingsTest,$Utf8NoBom)
[System.IO.File]::WriteAllText($Paths.App,$app,$Utf8NoBom)
[System.IO.File]::WriteAllText($Paths.Types,$types,$Utf8NoBom)
[System.IO.File]::WriteAllText($Paths.SettingsModal,$modal,$Utf8NoBom)
[System.IO.File]::WriteAllText($Paths.I18n,$i18n,$Utf8NoBom)
[System.IO.File]::WriteAllText($Paths.WailsModels,$wmodels,$Utf8NoBom)
if ($help -ne $null) { [System.IO.File]::WriteAllText($Paths.Help,$help,$Utf8NoBom) }
[System.IO.File]::WriteAllText($Paths.Bridge,[System.IO.File]::ReadAllText($Payload),$Utf8NoBom)

Write-Host "Stage 5.2 Traxsource resilient search installed successfully."
Write-Host "Direct Traxsource remains first; optional JSON fallback is used when direct lookup is blocked/empty."
