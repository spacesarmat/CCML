$ErrorActionPreference = "Stop"

$ProjectRoot = "C:\Users\ANDYBUM\GolandProjects\CCML"
$BridgePayload = Join-Path $PSScriptRoot "CCML-stage5.2-v2-traxsource_bridge.go"
$TestPayload = Join-Path $PSScriptRoot "CCML-stage5.2-v2-traxsource_bridge_test.go"

$P = @{
  Trax = Join-Path $ProjectRoot "internal\metadata\traxsource.go"
  Bridge = Join-Path $ProjectRoot "internal\metadata\traxsource_bridge.go"
  BridgeTest = Join-Path $ProjectRoot "internal\metadata\traxsource_bridge_test.go"
  Model = Join-Path $ProjectRoot "internal\model\model.go"
  Settings = Join-Path $ProjectRoot "internal\settings\service.go"
  App = Join-Path $ProjectRoot "app.go"
  Types = Join-Path $ProjectRoot "frontend\src\types.ts"
  Modal = Join-Path $ProjectRoot "frontend\src\SettingsModal.tsx"
  I18n = Join-Path $ProjectRoot "frontend\src\i18n.ts"
  WModels = Join-Path $ProjectRoot "frontend\wailsjs\go\models.ts"
  Help = Join-Path $ProjectRoot "frontend\src\HelpModal.tsx"
}

foreach ($key in @("Trax","Model","Settings","App","Types","Modal","I18n","WModels")) {
  if (-not (Test-Path $P[$key])) { throw "Required file not found: $($P[$key])" }
}
if (-not (Test-Path $BridgePayload)) { throw "Bridge payload not found: $BridgePayload" }
if (-not (Test-Path $TestPayload)) { throw "Test payload not found: $TestPayload" }

$Utf8NoBom = New-Object System.Text.UTF8Encoding($false)

function Read-Text([string]$Path) {
  return [System.IO.File]::ReadAllText($Path)
}

function Replace-RegexOnce {
  param(
    [string]$Text,
    [string]$Pattern,
    [string]$Replacement,
    [string]$Label,
    [System.Text.RegularExpressions.RegexOptions]$Options = [System.Text.RegularExpressions.RegexOptions]::Multiline
  )
  $rx = New-Object System.Text.RegularExpressions.Regex($Pattern, $Options)
  $matches = $rx.Matches($Text)
  if ($matches.Count -ne 1) {
    throw "Stage 5.2 v2 expected exactly one match for $Label, found $($matches.Count). No files were changed."
  }
  $m = $matches[0]
  return $Text.Substring(0, $m.Index) + $Replacement + $Text.Substring($m.Index + $m.Length)
}

function Insert-AfterRegexOnce {
  param(
    [string]$Text,
    [string]$Pattern,
    [string]$Insert,
    [string]$Label,
    [System.Text.RegularExpressions.RegexOptions]$Options = [System.Text.RegularExpressions.RegexOptions]::Multiline
  )
  $rx = New-Object System.Text.RegularExpressions.Regex($Pattern, $Options)
  $matches = $rx.Matches($Text)
  if ($matches.Count -ne 1) {
    throw "Stage 5.2 v2 expected exactly one match for $Label, found $($matches.Count). No files were changed."
  }
  $m = $matches[0]
  return $Text.Substring(0, $m.Index) + $m.Value + $Insert + $Text.Substring($m.Index + $m.Length)
}

$trax = Read-Text $P.Trax
$model = Read-Text $P.Model
$settings = Read-Text $P.Settings
$app = Read-Text $P.App
$types = Read-Text $P.Types
$modal = Read-Text $P.Modal
$i18n = Read-Text $P.I18n
$wmodels = Read-Text $P.WModels
$help = if (Test-Path $P.Help) { Read-Text $P.Help } else { $null }

if ($model.Contains("TraxsourceAPIKey") -and (Test-Path $P.Bridge)) {
  Write-Host "Stage 5.2 v2 is already installed."
  exit 0
}
if ($model.Contains("TraxsourceAPIKey") -or (Test-Path $P.Bridge) -or (Test-Path $P.BridgeTest)) {
  throw "Partial Stage 5.2 state detected. No files were changed."
}

# Traxsource provider: match Go syntax, not an exact multiline text block.
$trax = Insert-AfterRegexOnce $trax '^[ \t]*userAgent[ \t]+string[ \t]*$' "`r`n`tbridgeAPIKey string`r`n`tbridgeURL    string" "Traxsource provider fields"

$constructor = @'
func NewTraxsourceProvider(userAgent string) *TraxsourceProvider {
	return NewTraxsourceProviderWithFallback(userAgent, "")
}

func NewTraxsourceProviderWithFallback(userAgent, bridgeAPIKey string) *TraxsourceProvider {
	return &TraxsourceProvider{
		client:       &http.Client{Timeout: 15 * time.Second},
		baseURL:      traxsourceBaseURL,
		userAgent:    strings.TrimSpace(userAgent),
		bridgeAPIKey: strings.TrimSpace(bridgeAPIKey),
		bridgeURL:    traxsourceBridgeSearchURL,
	}
}
'@
$trax = Replace-RegexOnce $trax '(?ms)^func[ \t]+NewTraxsourceProvider\([ \t]*userAgent[ \t]+string[ \t]*\)[ \t]+\*TraxsourceProvider[ \t]*\{.*?^\}' $constructor "Traxsource provider constructor" ([System.Text.RegularExpressions.RegexOptions]::Multiline -bor [System.Text.RegularExpressions.RegexOptions]::Singleline)

$trax = Replace-RegexOnce $trax '^[ \t]*func[ \t]+\(p[ \t]+\*TraxsourceProvider\)[ \t]+Search\(' 'func (p *TraxsourceProvider) searchDirect(' "Traxsource direct Search rename"

# Go settings/model wiring.
$model = Insert-AfterRegexOnce $model '^[ \t]*TraxsourceEnabled[ \t]+bool[ \t]+`json:"traxsourceEnabled"`[ \t]*$' "`r`n`r`n`tTraxsourceAPIKey string ``json:`"traxsourceApiKey`"``" "MetadataSettings Traxsource API key"
$model = Insert-AfterRegexOnce $model '^[ \t]*s\.YandexMusicLanguage[ \t]*=[ \t]*strings\.ToLower\(strings\.TrimSpace\(s\.YandexMusicLanguage\)\)[ \t]*$' "`r`n`ts.TraxsourceAPIKey = strings.TrimSpace(s.TraxsourceAPIKey)" "MetadataSettings Normalize"

$settings = Insert-AfterRegexOnce $settings '^[ \t]*TraxsourceEnabled:[ \t]*false,[ \t]*$' "`r`n`r`n`t`tTraxsourceAPIKey:         strings.TrimSpace(os.Getenv(`"TRAXSOURCE_API_KEY`"))," "Traxsource env fallback key"

$app = Insert-AfterRegexOnce $app '^[ \t]*"traxsource":[ \t]*"https://www\.traxsource\.com/terms-of-service",[ \t]*$' "`r`n`t`t`"traxsourcebridge`": `"https://parse.bot/marketplace/0123da1a-5d2d-4970-bf6f-398f792855a1/traxsource-com-api`"," "Traxsource fallback help link"
$app = Replace-RegexOnce $app 'metadata\.NewTraxsourceProvider\([ \t]*metadataUserAgent[ \t]*\)' 'metadata.NewTraxsourceProviderWithFallback(metadataUserAgent, config.TraxsourceAPIKey)' "Traxsource provider wiring"

# Frontend types/UI.
$types = Insert-AfterRegexOnce $types '^[ \t]*traxsourceEnabled:[ \t]*boolean[ \t]*$' "`r`n  traxsourceApiKey: string" "frontend MetadataSettings type"

$traxCard = @'
              <ProviderCard language={language} title="Traxsource" health={providerHealth['Traxsource']} description={t('settings.traxsourceDescription')} enabled={settings.traxsourceEnabled} onEnabled={(v) => change('traxsourceEnabled', v)} badge={t('settings.experimentalSource')} helpLabel={t('settings.traxsourceBridge')} onHelp={() => void openProviderPage('traxsourcebridge')}>
                <SettingInput label={t('settings.traxsourceApiKey')} value={settings.traxsourceApiKey} onChange={(v) => change('traxsourceApiKey', v)} password placeholder={t('settings.optional')} />
              </ProviderCard>
'@
$modal = Replace-RegexOnce $modal '^[ \t]*<ProviderCard[^\r\n]*title="Traxsource"[^\r\n]*/>[ \t]*$' $traxCard "Traxsource Settings card"

$oldEn = "  'settings.traxsourceDescription': 'Experimental lookup of the public Traxsource web catalog. Traxsource does not publish a developer/token API for this use; disabled by default.',"
$newEn = @"
  'settings.traxsourceDescription': 'Direct public-catalog lookup with an optional JSON fallback when Traxsource blocks unattended requests with Cloudflare. The fallback is an independent third-party service and requires its own API key.',
  'settings.traxsourceApiKey': 'Traxsource fallback API key (optional)',
  'settings.traxsourceBridge': 'Fallback API / get key',
"@
if (-not $i18n.Contains($oldEn)) { throw "Stage 5.2 v2 English i18n anchor not found. No files were changed." }
$i18n = $i18n.Replace($oldEn, $newEn.TrimEnd("`r","`n"))

$oldRu = "  'settings.traxsourceDescription': 'Экспериментальный поиск по публичному веб-каталогу Traxsource. Публичной developer/token API для этого сценария у Traxsource нет; источник выключен по умолчанию.',"
$newRu = @"
  'settings.traxsourceDescription': 'Прямой поиск по публичному каталогу с опциональным JSON fallback, когда Traxsource блокирует автоматические запросы через Cloudflare. Fallback — независимый сторонний сервис и требует отдельный API-ключ.',
  'settings.traxsourceApiKey': 'API-ключ fallback для Traxsource (необязательно)',
  'settings.traxsourceBridge': 'Fallback API / получить ключ',
"@
if (-not $i18n.Contains($oldRu)) { throw "Stage 5.2 v2 Russian i18n anchor not found. No files were changed." }
$i18n = $i18n.Replace($oldRu, $newRu.TrimEnd("`r","`n"))

$wmodels = Insert-AfterRegexOnce $wmodels '^[ \t]*traxsourceEnabled:[ \t]*boolean;[ \t]*$' "`r`n`t    traxsourceApiKey: string;" "Wails MetadataSettings property"
$wmodels = Insert-AfterRegexOnce $wmodels '^[ \t]*this\.traxsourceEnabled[ \t]*=[ \t]*source\["traxsourceEnabled"\];[ \t]*$' "`r`n`t        this.traxsourceApiKey = source[`"traxsourceApiKey`"];" "Wails MetadataSettings constructor"

# Help update is optional/best-effort so an older/newer Help text cannot block this provider fix.
if ($help -ne $null) {
  $ruNeedle = "          'Вкладка Метаданные запрашивает включённые источники: например MusicBrainz, Deezer, Apple iTunes и другие настроенные провайдеры.',"
  if ($help.Contains($ruNeedle) -and -not $help.Contains("Traxsource сначала запрашивается напрямую")) {
    $help = $help.Replace($ruNeedle, $ruNeedle + "`r`n          'Traxsource сначала запрашивается напрямую. Если сайт отвечает Cloudflare/human verification, CCML может автоматически использовать настроенный JSON fallback API; ключ задаётся в Настройки → Traxsource.',")
  }
  $enNeedle = "          'Metadata queries enabled providers such as MusicBrainz, Deezer, Apple iTunes and other configured sources.',"
  if ($help.Contains($enNeedle) -and -not $help.Contains("Traxsource is tried directly first")) {
    $help = $help.Replace($enNeedle, $enNeedle + "`r`n          'Traxsource is tried directly first. If the site responds with Cloudflare/human verification, CCML can automatically use the configured JSON fallback API; set its key under Settings → Traxsource.',")
  }
}

# Every required transform succeeded in memory. Only now touch project files.
$backup = ".stage5.2v2.bak"
foreach ($key in @("Trax","Model","Settings","App","Types","Modal","I18n","WModels")) {
  Copy-Item $P[$key] ($P[$key] + $backup) -Force
}
if ($help -ne $null) { Copy-Item $P.Help ($P.Help + $backup) -Force }

[System.IO.File]::WriteAllText($P.Trax, $trax, $Utf8NoBom)
[System.IO.File]::WriteAllText($P.Model, $model, $Utf8NoBom)
[System.IO.File]::WriteAllText($P.Settings, $settings, $Utf8NoBom)
[System.IO.File]::WriteAllText($P.App, $app, $Utf8NoBom)
[System.IO.File]::WriteAllText($P.Types, $types, $Utf8NoBom)
[System.IO.File]::WriteAllText($P.Modal, $modal, $Utf8NoBom)
[System.IO.File]::WriteAllText($P.I18n, $i18n, $Utf8NoBom)
[System.IO.File]::WriteAllText($P.WModels, $wmodels, $Utf8NoBom)
if ($help -ne $null) { [System.IO.File]::WriteAllText($P.Help, $help, $Utf8NoBom) }

[System.IO.File]::WriteAllText($P.Bridge, [System.IO.File]::ReadAllText($BridgePayload), $Utf8NoBom)
[System.IO.File]::WriteAllText($P.BridgeTest, [System.IO.File]::ReadAllText($TestPayload), $Utf8NoBom)

Write-Host "Stage 5.2 v2 Traxsource resilient search installed successfully."
Write-Host "The old one-argument NewTraxsourceProvider constructor remains available for compatibility."
