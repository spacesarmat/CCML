import type {TableColumnID, TableSort} from './tableSort'

export type TablePresetID = 'dj' | 'metadata' | 'technical' | 'compact'

export type TablePreset = {
  layout: {
    order: TableColumnID[]
    visible: TableColumnID[]
    widths: Partial<Record<TableColumnID, number>>
  }
  sort: TableSort[]
}

export const TABLE_PRESET_IDS: TablePresetID[] = [
  'dj',
  'metadata',
  'technical',
  'compact',
]

const ALL_COLUMNS: TableColumnID[] = [
  'cover', 'trackNumber', 'artist', 'title', 'album', 'albumArtist',
  'year', 'genre', 'label', 'catalogNumber', 'releaseDate',
  'duration', 'codec', 'sampleRate', 'bitRate', 'channels',
  'lufs', 'truePeak', 'bpmKey', 'isrc', 'fileName', 'path',
]

export function tablePreset(id: TablePresetID): TablePreset {
  switch (id) {
    case 'dj':
      return preset(
        ['cover', 'artist', 'title', 'bpmKey', 'genre', 'lufs', 'truePeak', 'duration', 'codec'],
        {
          cover: 54,
          artist: 180,
          title: 260,
          bpmKey: 112,
          genre: 150,
          lufs: 72,
          truePeak: 90,
          duration: 76,
          codec: 72,
        },
        [
          {column: 'artist', direction: 'asc'},
          {column: 'title', direction: 'asc'},
        ],
      )

    case 'metadata':
      return preset(
        ['cover', 'artist', 'title', 'album', 'albumArtist', 'label', 'catalogNumber', 'releaseDate', 'isrc', 'genre', 'year'],
        {
          cover: 54,
          artist: 170,
          title: 220,
          album: 180,
          albumArtist: 170,
          label: 150,
          catalogNumber: 120,
          releaseDate: 108,
          isrc: 132,
          genre: 130,
          year: 66,
        },
        [
          {column: 'artist', direction: 'asc'},
          {column: 'album', direction: 'asc'},
          {column: 'trackNumber', direction: 'asc'},
        ],
      )

    case 'technical':
      return preset(
        ['artist', 'title', 'codec', 'sampleRate', 'bitRate', 'channels', 'lufs', 'truePeak', 'bpmKey', 'duration', 'path'],
        {
          artist: 170,
          title: 220,
          codec: 82,
          sampleRate: 96,
          bitRate: 94,
          channels: 78,
          lufs: 72,
          truePeak: 90,
          bpmKey: 110,
          duration: 76,
          path: 360,
        },
        [
          {column: 'codec', direction: 'asc'},
          {column: 'artist', direction: 'asc'},
          {column: 'title', direction: 'asc'},
        ],
      )

    case 'compact':
      return preset(
        ['cover', 'artist', 'title', 'bpmKey', 'duration'],
        {
          cover: 52,
          artist: 180,
          title: 280,
          bpmKey: 108,
          duration: 74,
        },
        [
          {column: 'artist', direction: 'asc'},
          {column: 'title', direction: 'asc'},
        ],
      )
  }
}

function preset(
  visible: TableColumnID[],
  widths: Partial<Record<TableColumnID, number>>,
  sort: TableSort[],
): TablePreset {
  const order = [...visible, ...ALL_COLUMNS.filter((column) => !visible.includes(column))]
  return {
    layout: {
      order,
      visible: [...visible],
      widths: {...widths},
    },
    sort: sort.map((rule) => ({...rule})),
  }
}
