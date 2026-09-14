import {useMemo, useState} from 'react'
import type {AppLanguage} from './i18n'
import type {Track} from './types'
import {
  buildLibraryFilterOptions,
  countActiveLibraryFilters,
  type LibraryFilters,
} from './libraryFilters'

type Props = {
  language: AppLanguage
  locale: string
  tracks: Track[]
  filters: LibraryFilters
  onChange: (filters: LibraryFilters) => void
  onClear: () => void
}

type MultiFilterKey = 'artists' | 'genres' | 'labels' | 'codecs' | 'keys' | 'camelotKeys'

function LibraryFilterPanel({language, locale, tracks, filters, onChange, onClear}: Props) {
  const [artistSearch, setArtistSearch] = useState('')
  const [genreSearch, setGenreSearch] = useState('')
  const [labelSearch, setLabelSearch] = useState('')
  const options = useMemo(() => buildLibraryFilterOptions(tracks, locale), [tracks, locale])
  const activeCount = countActiveLibraryFilters(filters)
  const indexedCoverCount = tracks.filter((track) => track.coverIndexed).length
  const coverIndexComplete = indexedCoverCount === tracks.length

  const copy = language === 'ru'
    ? {
        filters: 'Фильтры',
        title: 'Фильтры библиотеки',
        hint: 'Категории объединяются по AND, выбранные значения внутри категории — по OR.',
        reset: 'Сбросить фильтры',
        artist: 'Исполнитель',
        genre: 'Жанр',
        label: 'Лейбл',
        codec: 'Кодек',
        key: 'Тональность',
        camelot: 'Camelot',
        cover: 'Обложка',
        coverAny: 'Любая',
        coverWith: 'Есть',
        coverWithout: 'Нет',
        coverIndexed: 'Индексировано',
        coverRescan: 'Запустите сканирование библиотеки один раз для завершения индекса.',
        year: 'Год',
        bpm: 'BPM',
        lufs: 'LUFS',
        from: 'от',
        to: 'до',
        findValue: 'Найти значение…',
        noValues: 'Нет значений',
        selected: 'выбрано',
      }
    : {
        filters: 'Filters',
        title: 'Library filters',
        hint: 'Categories combine with AND; selected values inside one category combine with OR.',
        reset: 'Clear filters',
        artist: 'Artist',
        genre: 'Genre',
        label: 'Label',
        codec: 'Codec',
        key: 'Key',
        camelot: 'Camelot',
        cover: 'Cover',
        coverAny: 'Any',
        coverWith: 'With cover',
        coverWithout: 'Without cover',
        coverIndexed: 'Indexed',
        coverRescan: 'Run one library scan to finish the cover index.',
        year: 'Year',
        bpm: 'BPM',
        lufs: 'LUFS',
        from: 'from',
        to: 'to',
        findValue: 'Find value…',
        noValues: 'No values',
        selected: 'selected',
      }

  function toggleValue(key: MultiFilterKey, value: string) {
    const current = filters[key]
    const exists = current.some((item) => equalFold(item, value))
    onChange({
      ...filters,
      [key]: exists
        ? current.filter((item) => !equalFold(item, value))
        : [...current, value],
    })
  }

  function updateNumber(
    key: 'yearMin' | 'yearMax' | 'bpmMin' | 'bpmMax' | 'lufsMin' | 'lufsMax',
    value: string,
  ) {
    onChange({...filters, [key]: parseNullableNumber(value)})
  }

  return (
    <div className="library-filter-controls">
      <details className="library-filter-picker">
        <summary title={copy.title}>
          <span>≡ {copy.filters}</span>
          {activeCount > 0 && <b>{activeCount}</b>}
        </summary>

        <div className="library-filter-panel">
          <header>
            <div>
              <strong>{copy.title}</strong>
              <small>{copy.hint}</small>
            </div>
            <button type="button" onClick={onClear} disabled={activeCount === 0}>{copy.reset}</button>
          </header>

          <div className="library-filter-grid">
            <FilterChecklist
              title={copy.artist}
              values={options.artists}
              selected={filters.artists}
              query={artistSearch}
              onQueryChange={setArtistSearch}
              onToggle={(value) => toggleValue('artists', value)}
              searchPlaceholder={copy.findValue}
              emptyLabel={copy.noValues}
              selectedLabel={copy.selected}
            />

            <FilterChecklist
              title={copy.genre}
              values={options.genres}
              selected={filters.genres}
              query={genreSearch}
              onQueryChange={setGenreSearch}
              onToggle={(value) => toggleValue('genres', value)}
              searchPlaceholder={copy.findValue}
              emptyLabel={copy.noValues}
              selectedLabel={copy.selected}
            />

            <FilterChecklist
              title={copy.label}
              values={options.labels}
              selected={filters.labels}
              query={labelSearch}
              onQueryChange={setLabelSearch}
              onToggle={(value) => toggleValue('labels', value)}
              searchPlaceholder={copy.findValue}
              emptyLabel={copy.noValues}
              selectedLabel={copy.selected}
            />

            <FilterChecklist
              title={copy.codec}
              values={options.codecs}
              selected={filters.codecs}
              onToggle={(value) => toggleValue('codecs', value)}
              emptyLabel={copy.noValues}
              selectedLabel={copy.selected}
            />

            <FilterChecklist
              title={copy.key}
              values={options.keys}
              selected={filters.keys}
              onToggle={(value) => toggleValue('keys', value)}
              emptyLabel={copy.noValues}
              selectedLabel={copy.selected}
            />

            <FilterChecklist
              title={copy.camelot}
              values={options.camelotKeys}
              selected={filters.camelotKeys}
              onToggle={(value) => toggleValue('camelotKeys', value)}
              emptyLabel={copy.noValues}
              selectedLabel={copy.selected}
            />

            <section className="library-cover-filter">
              <div className="library-filter-section-head">
                <strong>{copy.cover}</strong>
                <small>{copy.coverIndexed} {indexedCoverCount}/{tracks.length}</small>
              </div>
              <div className="library-cover-filter-options" role="radiogroup" aria-label={copy.cover}>
                {([
                  ['any', copy.coverAny],
                  ['with', copy.coverWith],
                  ['without', copy.coverWithout],
                ] as const).map(([mode, label]) => (
                  <button
                    type="button"
                    role="radio"
                    aria-checked={filters.cover === mode}
                    className={filters.cover === mode ? 'active' : ''}
                    key={mode}
                    onClick={() => onChange({...filters, cover: mode})}
                  >
                    {label}
                  </button>
                ))}
              </div>
              {!coverIndexComplete && tracks.length > 0 && (
                <small className="library-cover-filter-hint">{copy.coverRescan}</small>
              )}
            </section>

            <div className="library-range-stack">
              <RangeFilter
                title={copy.year}
                min={filters.yearMin}
                max={filters.yearMax}
                minPlaceholder={copy.from}
                maxPlaceholder={copy.to}
                step={1}
                onMin={(value) => updateNumber('yearMin', value)}
                onMax={(value) => updateNumber('yearMax', value)}
              />
              <RangeFilter
                title={copy.bpm}
                min={filters.bpmMin}
                max={filters.bpmMax}
                minPlaceholder={copy.from}
                maxPlaceholder={copy.to}
                step={0.1}
                onMin={(value) => updateNumber('bpmMin', value)}
                onMax={(value) => updateNumber('bpmMax', value)}
              />
              <RangeFilter
                title={copy.lufs}
                min={filters.lufsMin}
                max={filters.lufsMax}
                minPlaceholder={copy.from}
                maxPlaceholder={copy.to}
                step={0.1}
                onMin={(value) => updateNumber('lufsMin', value)}
                onMax={(value) => updateNumber('lufsMax', value)}
              />
            </div>
          </div>
        </div>
      </details>

      {activeCount > 0 && (
        <button type="button" className="library-filter-clear" onClick={onClear}>
          {copy.reset}
        </button>
      )}
    </div>
  )
}

type ChecklistProps = {
  title: string
  values: string[]
  selected: string[]
  query?: string
  onQueryChange?: (value: string) => void
  onToggle: (value: string) => void
  searchPlaceholder?: string
  emptyLabel: string
  selectedLabel: string
}

function FilterChecklist({
  title,
  values,
  selected,
  query = '',
  onQueryChange,
  onToggle,
  searchPlaceholder,
  emptyLabel,
  selectedLabel,
}: ChecklistProps) {
  const needle = query.trim().toLocaleLowerCase()
  const filtered = needle
    ? values.filter((value) => value.toLocaleLowerCase().includes(needle))
    : values

  return (
    <section className="library-filter-section">
      <div className="library-filter-section-head">
        <strong>{title}</strong>
        {selected.length > 0 && <small>{selected.length} {selectedLabel}</small>}
      </div>

      {onQueryChange && values.length > 8 && (
        <input
          className="library-filter-value-search"
          value={query}
          onChange={(event) => onQueryChange(event.target.value)}
          placeholder={searchPlaceholder}
        />
      )}

      <div className="library-filter-options">
        {filtered.length === 0 && <span className="library-filter-empty">{emptyLabel}</span>}
        {filtered.map((value) => {
          const checked = selected.some((item) => equalFold(item, value))
          return (
            <label key={value} className={checked ? 'active' : ''}>
              <input type="checkbox" checked={checked} onChange={() => onToggle(value)} />
              <span title={value}>{value}</span>
            </label>
          )
        })}
      </div>
    </section>
  )
}

type RangeProps = {
  title: string
  min: number | null
  max: number | null
  minPlaceholder: string
  maxPlaceholder: string
  step: number
  onMin: (value: string) => void
  onMax: (value: string) => void
}

function RangeFilter({
  title,
  min,
  max,
  minPlaceholder,
  maxPlaceholder,
  step,
  onMin,
  onMax,
}: RangeProps) {
  return (
    <section className="library-range-filter">
      <strong>{title}</strong>
      <div>
        <input
          type="number"
          value={min ?? ''}
          step={step}
          placeholder={minPlaceholder}
          onChange={(event) => onMin(event.target.value)}
        />
        <span>—</span>
        <input
          type="number"
          value={max ?? ''}
          step={step}
          placeholder={maxPlaceholder}
          onChange={(event) => onMax(event.target.value)}
        />
      </div>
    </section>
  )
}

function parseNullableNumber(value: string): number | null {
  const trimmed = value.trim()
  if (!trimmed) return null
  const number = Number(trimmed)
  return Number.isFinite(number) ? number : null
}

function equalFold(a: string, b: string): boolean {
  return a.trim().toLocaleLowerCase() === b.trim().toLocaleLowerCase()
}

export default LibraryFilterPanel
