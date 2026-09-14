$ErrorActionPreference = "Stop"

$ProjectRoot = "C:\Users\ANDYBUM\GolandProjects\CCML"
$AppPath = Join-Path $ProjectRoot "frontend\src\App.tsx"
$CssPath = Join-Path $ProjectRoot "frontend\src\styles.css"
$HelpPath = Join-Path $ProjectRoot "frontend\src\HelpModal.tsx"
$ComponentPath = Join-Path $ProjectRoot "frontend\src\TableColumns.tsx"
$PayloadPath = Join-Path $PSScriptRoot "CCML-stage7-table-columns.payload.txt"

foreach ($path in @($AppPath, $CssPath, $PayloadPath)) {
    if (-not (Test-Path $path)) { throw "Required file not found: $path" }
}

# Safe cleanup of payload files accidentally left by older hotfix archives.
Get-ChildItem $ProjectRoot -File -Filter "CCML-stage*.go" | Remove-Item -Force -ErrorAction SilentlyContinue

$Utf8NoBom = New-Object System.Text.UTF8Encoding($false)
$app = [System.IO.File]::ReadAllText($AppPath)
$css = [System.IO.File]::ReadAllText($CssPath)
$help = if (Test-Path $HelpPath) { [System.IO.File]::ReadAllText($HelpPath) } else { $null }

if ($app.Contains("TableColumnControls") -and (Test-Path $ComponentPath)) {
    Write-Host "Stage 7 table columns is already installed."
    exit 0
}
if ($app.Contains("TableColumnControls") -or (Test-Path $ComponentPath)) {
    throw "Partial Stage 7 state detected. No source files were changed."
}

function Find-One {
    param(
        [string]$Text,
        [string]$Pattern,
        [string]$Label,
        [System.Text.RegularExpressions.RegexOptions]$Options
    )
    $rx = New-Object System.Text.RegularExpressions.Regex($Pattern, $Options)
    $matches = $rx.Matches($Text)
    if ($matches.Count -ne 1) {
        throw "Stage 7 expected exactly one match for $Label, found $($matches.Count). No source files were changed."
    }
    return $matches[0]
}

function Replace-One {
    param(
        [string]$Text,
        [string]$Pattern,
        [string]$Replacement,
        [string]$Label,
        [System.Text.RegularExpressions.RegexOptions]$Options
    )
    $m = Find-One $Text $Pattern $Label $Options
    return $Text.Substring(0,$m.Index) + $Replacement + $Text.Substring($m.Index+$m.Length)
}

$multi = [System.Text.RegularExpressions.RegexOptions]::Multiline
$singleMulti = [System.Text.RegularExpressions.RegexOptions]::Multiline -bor [System.Text.RegularExpressions.RegexOptions]::Singleline

# Import the table-layout module after JobsPanel, regardless of HelpModal being present locally.
$m = Find-One $app '^import JobsPanel from ''\.\/JobsPanel''[ \t]*$' "JobsPanel import" $multi
$import = @'
import {
  TableColumnControls,
  isTableColumnID,
  loadTableLayout,
  renderTableColumn,
  reorderTableColumns,
  resetTableLayout,
  saveTableLayout,
  tableColumnCellClass,
  tableColumnLabel,
  type TableColumnID,
  type TableLayout,
} from './TableColumns'
'@
$app = $app.Substring(0,$m.Index+$m.Length) + "`r`n" + $import + $app.Substring($m.Index+$m.Length)

# Add table layout state near the existing selection state.
$m = Find-One $app '^[ \t]*const \[activeTrackID,[ \t]*setActiveTrackID\][ \t]*=[ \t]*useState<number \| null>\(null\)[ \t]*$' "active track state" $multi
$state = @'

  const [tableLayout, setTableLayout] = useState<TableLayout>(() => loadTableLayout())
  const [draggedTableColumn, setDraggedTableColumn] = useState<TableColumnID | null>(null)
'@
$app = $app.Substring(0,$m.Index+$m.Length) + $state + $app.Substring($m.Index+$m.Length)

# Add derived visible-column list after filteredTracks memo.
$filtered = Find-One $app '(?ms)^[ \t]*const filteredTracks = useMemo\(.*?^[ \t]*\)[ \t]*$' "filteredTracks memo" $singleMulti
$derived = @'

  const visibleTableColumns = useMemo(
    () => tableLayout.order.filter((id) => tableLayout.visible.includes(id)),
    [tableLayout],
  )
'@
$app = $app.Substring(0,$filtered.Index+$filtered.Length) + $derived + $app.Substring($filtered.Index+$filtered.Length)

# Add layout actions immediately before openMetadataInspector().
$openInspector = Find-One $app '^[ \t]*async function openMetadataInspector\(\)[ \t]*\{' "openMetadataInspector function" $multi
$actions = @'
  function updateTableLayout(next: TableLayout) {
    setTableLayout(next)
    saveTableLayout(next)
  }

  function toggleTableColumn(id: TableColumnID) {
    const currentlyVisible = tableLayout.visible.includes(id)
    if (currentlyVisible && tableLayout.visible.length === 1) return
    const visible = currentlyVisible
      ? tableLayout.visible.filter((column) => column !== id)
      : [...tableLayout.visible, id]
    updateTableLayout({...tableLayout, visible})
  }

  function moveTableColumn(source: TableColumnID, target: TableColumnID) {
    updateTableLayout({
      ...tableLayout,
      order: reorderTableColumns(tableLayout.order, source, target),
    })
  }

  function restoreDefaultTableLayout() {
    setTableLayout(resetTableLayout())
  }

'@
$app = $app.Substring(0,$openInspector.Index) + $actions + $app.Substring($openInspector.Index)

# Replace filter legend span with a right-side controls container that includes Columns.
$filterPattern = '(?ms)<span className="filter-legend">.*?</span>'
$filterReplacement = @'
<div className="library-filter-actions">
                  <span className="filter-legend">
                    {unchangedAfterEnrichment > 0 && <i className="legend-dot warn" title={t('library.enrichmentLegendUnchanged')} />}
                    {failedAfterEnrichment > 0 && <i className="legend-dot bad" title={t('library.enrichmentLegendFailed')} />}
                  </span>
                  <TableColumnControls
                    language={language}
                    layout={tableLayout}
                    onToggle={toggleTableColumn}
                    onMove={moveTableColumn}
                    onReset={restoreDefaultTableLayout}
                  />
                </div>
'@
$app = Replace-One $app $filterPattern $filterReplacement "library filter legend" $singleMulti

# Replace only the main library table. This is bounded by workspace-table-wrap and its closing div.
$tablePattern = '(?ms)<div className="workspace-table-wrap">\s*<table className="workspace-table">.*?</table>\s*</div>'
$tableReplacement = @'
<div className="workspace-table-wrap">
                <table className="workspace-table">
                  <thead>
                    <tr>
                      <th className="selection-col">
                        <input
                          type="checkbox"
                          aria-label={t('library.selectAll')}
                          checked={filteredTracks.length > 0 && filteredTracks.every((track) => selectedIDs.includes(track.id))}
                          onChange={toggleAllVisible}
                        />
                      </th>
                      {visibleTableColumns.map((column) => (
                        <th
                          key={column}
                          className={`table-column-header${draggedTableColumn === column ? ' dragging' : ''}`}
                          draggable
                          data-column-id={column}
                          title={language === 'ru' ? 'Перетащите для изменения порядка' : 'Drag to reorder'}
                          onDragStart={(event) => {
                            setDraggedTableColumn(column)
                            event.dataTransfer.effectAllowed = 'move'
                            event.dataTransfer.setData('text/plain', column)
                          }}
                          onDragEnd={() => setDraggedTableColumn(null)}
                          onDragOver={(event) => {
                            event.preventDefault()
                            event.dataTransfer.dropEffect = 'move'
                          }}
                          onDrop={(event) => {
                            event.preventDefault()
                            const source = event.dataTransfer.getData('text/plain') || draggedTableColumn || ''
                            if (isTableColumnID(source)) moveTableColumn(source, column)
                            setDraggedTableColumn(null)
                          }}
                        >
                          <span>{tableColumnLabel(column, language)}</span>
                          <i className="column-drag-handle" aria-hidden="true">⋮⋮</i>
                        </th>
                      ))}
                    </tr>
                  </thead>
                  <tbody>
                    {filteredTracks.map((track) => (
                      <tr
                        key={track.id}
                        data-track-id={track.id}
                        className={metadataRowClass(track)}
                        title={metadataRowTitle(track)}
                        aria-current={activeTrackID === track.id ? 'true' : undefined}
                        onClick={(event) => handleTrackRowClick(track.id, event.shiftKey, event.ctrlKey || event.metaKey)}
                      >
                        <td className="selection-col" onClick={(event: { stopPropagation(): void }) => event.stopPropagation()}>
                          <input
                            type="checkbox"
                            checked={selectedIDs.includes(track.id)}
                            aria-label={t('library.selectTrack', {title: track.title || track.fileName})}
                            onChange={() => toggleTrackSelection(track.id)}
                          />
                        </td>
                        {visibleTableColumns.map((column) => (
                          <td
                            key={column}
                            className={tableColumnCellClass(column)}
                            title={column === 'path' ? track.path : undefined}
                          >
                            {renderTableColumn(column, track, language, t('library.unknownArtist'))}
                          </td>
                        ))}
                      </tr>
                    ))}
                    {filteredTracks.length === 0 && (
                      <tr className="empty-row">
                        <td colSpan={visibleTableColumns.length + 1}>{t('library.filterEmpty')}</td>
                      </tr>
                    )}
                  </tbody>
                </table>
              </div>
'@
$app = Replace-One $app $tablePattern $tableReplacement "library table" $singleMulti

# Append Stage 7 CSS; avoid modifying older CSS rules so it layers cleanly over local hotfixes.
$cssAppend = @'

/* Stage 7 — configurable/reorderable library columns. */
.library-filter-actions { display: flex; align-items: center; gap: 8px; }
.table-column-picker { position: relative; }
.table-column-picker > summary { min-width: 84px; padding: 5px 8px; display: flex; align-items: center; gap: 6px; list-style: none; border: 1px solid #2a394d; border-radius: 6px; background: #101923; color: #91a1b6; font-size: 8px; cursor: pointer; user-select: none; }
.table-column-picker > summary::-webkit-details-marker { display: none; }
.table-column-picker > summary:hover { border-color: #40536d; color: #d1dbe7; background: #14202d; }
.table-column-picker > summary b { min-width: 16px; margin-left: auto; padding: 1px 4px; border-radius: 8px; background: #1e2c40; color: #9eb5d5; font-size: 7px; text-align: center; }
.table-column-picker[open] > summary { border-color: #5572a7; color: #dce6f2; background: #172337; }
.table-column-popover { position: absolute; z-index: 40; top: calc(100% + 6px); right: 0; width: 330px; max-height: min(540px, calc(100vh - 190px)); display: grid; grid-template-rows: auto minmax(0,1fr); border: 1px solid #314158; border-radius: 9px; background: #0e151f; box-shadow: 0 18px 45px rgba(0,0,0,.38); overflow: hidden; }
.table-column-popover-head { padding: 9px 10px; display: flex; align-items: flex-start; gap: 10px; border-bottom: 1px solid #253347; background: #111a25; }
.table-column-popover-head > div { min-width: 0; display: grid; gap: 3px; }
.table-column-popover-head strong { color: #d5dfeb; font-size: 9px; }
.table-column-popover-head span { color: #75869b; font-size: 7px; line-height: 1.4; }
.table-column-popover-head button { margin-left: auto; padding: 4px 7px; font-size: 7px; }
.table-column-list { min-height: 0; overflow: auto; padding: 5px; }
.table-column-option { min-height: 31px; display: grid; grid-template-columns: 20px minmax(0,1fr) auto; align-items: center; gap: 5px; padding: 3px 6px; border: 1px solid transparent; border-radius: 6px; user-select: none; }
.table-column-option:hover { background: #141f2c; border-color: #24354a; }
.table-column-option.dragging { opacity: .45; background: #18263a; border-color: #526f9f; }
.column-grip { color: #596b82; font-size: 12px; cursor: grab; letter-spacing: -3px; }
.table-column-option:active .column-grip { cursor: grabbing; }
.table-column-option label { min-width: 0; display: flex; align-items: center; gap: 7px; color: #aebccc; font-size: 8px; cursor: pointer; }
.table-column-option label span { white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
.table-column-option input { width: 13px; height: 13px; margin: 0; }
.table-column-option small { min-width: 38px; color: #59708d; font-size: 6px; text-align: right; text-transform: uppercase; letter-spacing: .05em; }

.workspace-table th.table-column-header { min-width: 54px; position: sticky; cursor: grab; user-select: none; -webkit-user-select: none; }
.workspace-table th.table-column-header:active { cursor: grabbing; }
.workspace-table th.table-column-header > span { display: inline-block; max-width: calc(100% - 14px); vertical-align: middle; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
.workspace-table th.table-column-header .column-drag-handle { margin-left: 5px; color: #485a70; font-style: normal; font-size: 9px; letter-spacing: -3px; opacity: .6; }
.workspace-table th.table-column-header:hover .column-drag-handle { color: #8296b0; opacity: 1; }
.workspace-table th.table-column-header.dragging { opacity: .45; background: #182438; }
.workspace-table td.numeric-cell { font-variant-numeric: tabular-nums; white-space: nowrap; }
.workspace-table td.file-cell { max-width: 320px; color: #7f91a8; }
.workspace-table td.file-cell[title] { cursor: help; }

@media (max-width: 900px) {
  .table-column-popover { width: min(310px, calc(100vw - 32px)); }
}
'@
$css += $cssAppend

# Best-effort Help update. It never blocks the feature if Help text differs locally.
if ($help -ne $null -and -not $help.Contains("кнопка «Колонки»")) {
    $ruNeedle = "          'При перемещении клавишами активная строка автоматически прокручивается в видимую область.',"
    if ($help.Contains($ruNeedle)) {
        $help = $help.Replace($ruNeedle, $ruNeedle + "`r`n          'Кнопка «Колонки» позволяет включать и скрывать поля таблицы. Порядок можно менять перетаскиванием строк в панели или самих заголовков таблицы; раскладка сохраняется между запусками.',")
    }
    $enNeedle = "          'Keyboard navigation automatically scrolls the active row into view.',"
    if ($help.Contains($enNeedle)) {
        $help = $help.Replace($enNeedle, $enNeedle + "`r`n          'The Columns button controls visible table fields. Reorder them by dragging rows in the picker or table headers; the layout persists between launches.',")
    }
}

# All required source transformations succeeded in memory. Back up, then write.
Copy-Item $AppPath "$AppPath.stage7.bak" -Force
Copy-Item $CssPath "$CssPath.stage7.bak" -Force
if ($help -ne $null) { Copy-Item $HelpPath "$HelpPath.stage7.bak" -Force }

[System.IO.File]::WriteAllText($AppPath, $app, $Utf8NoBom)
[System.IO.File]::WriteAllText($CssPath, $css, $Utf8NoBom)
if ($help -ne $null) { [System.IO.File]::WriteAllText($HelpPath, $help, $Utf8NoBom) }
[System.IO.File]::WriteAllText($ComponentPath, [System.IO.File]::ReadAllText($PayloadPath), $Utf8NoBom)

Write-Host "Stage 7 configurable table columns installed successfully."
Write-Host "Column visibility and order are persisted in localStorage."
