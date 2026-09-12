export type AppLanguage = 'en' | 'ru'

const STORAGE_KEY = 'ccml.language'

const en = {
  'app.subtitle': 'Cross-platform music library & mastering workspace',
  'language.label': 'Interface language',
  'language.english': 'English',
  'language.russian': 'Russian',

  'status.ffmpeg.ready': 'ready',
  'status.ffmpeg.missing': 'missing',
  'status.ffmpeg.updating': 'updating…',
  'status.essentia.ready': 'ready',
  'status.essentia.optional': 'optional',
  'status.metadata': 'Metadata',

  'toolbar.chooseFolder': 'Choose folder',
  'toolbar.folderPlaceholder': 'Music folder',
  'toolbar.scan': 'Scan',
  'toolbar.cancel': 'Cancel',
  'toolbar.updateFFmpeg': 'Update FFmpeg',
  'toolbar.updatingFFmpeg': 'Updating FFmpeg…',

  'warning.ffmpegUpdateFailed': 'FFmpeg update check failed; CCML will keep using the installed version.',

  'roots.title': 'Library folders',
  'roots.managed': '{count} managed',
  'roots.lastScan': 'Last scan {date}',
  'roots.notScanned': 'Not scanned yet',
  'roots.remove': 'Remove',
  'roots.removeConfirm': 'Remove indexed tracks from the CCML database too? Audio files on disk will not be deleted.',

  'stats.tracks': 'Tracks',
  'stats.artists': 'Artists',
  'stats.albums': 'Albums',
  'stats.duration': 'Duration',
  'stats.size': 'Size',
  'stats.duplicateGroups': 'Duplicate groups',

  'scan.scanning': 'Scanning library',
  'scan.cancelled': 'Scan cancelled',
  'scan.finished': 'Scan finished',
  'scan.processed': 'Processed: {count}',
  'scan.added': 'Added',
  'scan.updated': 'Updated',
  'scan.skipped': 'Skipped',
  'scan.removed': 'Removed',
  'scan.failed': 'Failed',

  'library.title': 'Library',
  'library.shown': '{count} shown',
  'library.searchPlaceholder': 'Search artist, title, album…',
  'library.search': 'Search',
  'library.duplicates': 'Duplicates',
  'library.unknownArtist': 'Unknown artist',

  'table.artist': 'Artist',
  'table.title': 'Title',
  'table.album': 'Album',
  'table.time': 'Time',
  'table.codec': 'Codec',
  'table.bpmKey': 'BPM / Key',

  'details.selectTrack': 'Select a track',
  'details.toolsHere': 'Track tools appear here',
  'details.path': 'Path',
  'details.format': 'Format',
  'details.loudness': 'Loudness',
  'details.notAnalyzed': 'Not analyzed',
  'details.channelsShort': 'ch',

  'actions.loudnessAnalysis': 'Loudness analysis',
  'actions.findMetadata': 'Find metadata',
  'actions.bpmKey': 'BPM & Key',
  'actions.replayGain': 'ReplayGain tags',

  'audio.title': 'Audio processing',
  'audio.targetLUFS': 'Target LUFS',
  'audio.truePeak': 'True peak dBTP',
  'audio.preGain': 'Pre-gain dB',
  'audio.pitch': 'Pitch semitones',
  'audio.clippingRepair': 'Clipping repair',
  'audio.multiband': 'Multiband compression',
  'audio.limiter': 'Limiter',
  'audio.keepOriginal': 'Keep original if replacing',
  'audio.createProcessedCopy': 'Create processed copy',

  'organize.title': 'Organize / rename',
  'organize.rootFolder': 'Root folder',
  'organize.rootPlaceholder': 'Empty = current folder',
  'organize.template': 'Template',
  'organize.regex': 'Regex',
  'organize.regexPlaceholder': 'Optional pattern',
  'organize.replace': 'Replace',
  'organize.replacePlaceholder': '$1 etc.',
  'organize.preview': 'Preview',
  'organize.apply': 'Apply move/rename',

  'metadata.title': 'Metadata candidates',
  'metadata.artworkAlt': 'Artwork',
  'metadata.unknownAlbum': 'Unknown album',
  'metadata.match': '{percent}% match',

  'duplicates.title': 'Probable duplicates',
  'duplicates.files': '{count} files',

  'footer.scanning': 'Scanning…',
  'footer.working': 'Working…',
  'footer.version': 'CCML v0.3 · EN/RU',

  'message.ready': 'Ready',
  'message.backendUnavailable': 'Wails backend is not available. Run the app with "wails dev".',
  'message.updatingBundledFFmpeg': 'Updating bundled FFmpeg…',
  'message.ffmpegUpdateFailed': 'FFmpeg update failed: {error}',
  'message.checkingAudioTools': 'Checking audio tools…',
  'message.ffmpegDetected': 'FFmpeg {version} · {source}',
  'message.ffmpegNotFound': 'FFmpeg/ffprobe not found',
  'message.checkingFFmpegUpdates': 'Checking FFmpeg updates…',
  'message.ffmpegUpdated': 'FFmpeg updated to {version}',
  'message.ffmpegUpToDate': 'FFmpeg {version} is up to date',
  'message.loadingLibrary': 'Loading library…',
  'message.tracksLoaded': '{count} tracks loaded',
  'message.chooseMusicFolder': 'Choose a music folder…',
  'message.chooseFolderFirst': 'Choose a music folder first',
  'message.scanningMusic': 'Scanning music…',
  'message.scanCancelled': 'Scan cancelled: {count} files processed',
  'message.scanComplete': 'Scan complete: +{added}, updated {updated}, skipped {skipped}, removed {removed}, failed {failed}',
  'message.cancellingScan': 'Cancelling scan…',
  'message.libraryFolderRemoved': 'Library folder removed',
  'message.analyzingLoudness': 'Analyzing loudness…',
  'message.loudnessResult': 'Loudness {lufs} LUFS, true peak {peak} dBTP',
  'message.writingReplayGain': 'Writing ReplayGain metadata…',
  'message.replayGainWritten': 'ReplayGain written from {lufs} LUFS analysis',
  'message.renderingProcessed': 'Rendering processed copy…',
  'message.processed': 'Processed: {path}',
  'message.analyzingBpmKey': 'Analyzing BPM and key with Essentia…',
  'message.bpmKeyResult': '{bpm} BPM · {key} {scale}',
  'message.searchingMetadata': 'Searching metadata providers…',
  'message.metadataCandidatesFound': '{count} metadata candidates found',
  'message.buildingTargetPath': 'Building target path…',
  'message.renamePreviewUpdated': 'Rename preview updated',
  'message.movingTrack': 'Moving/renaming track…',
  'message.moved': 'Moved: {path}',
  'message.searchingDuplicates': 'Searching probable duplicates…',
  'message.duplicateGroupsFound': '{count} duplicate groups found',

  'duration.hourShort': 'h',
  'duration.dayShort': 'd',
} as const

export type TranslationKey = keyof typeof en

const ru: Record<TranslationKey, string> = {
  'app.subtitle': 'Кроссплатформенная медиатека и мастеринг музыки',
  'language.label': 'Язык интерфейса',
  'language.english': 'Английский',
  'language.russian': 'Русский',

  'status.ffmpeg.ready': 'готов',
  'status.ffmpeg.missing': 'не найден',
  'status.ffmpeg.updating': 'обновляется…',
  'status.essentia.ready': 'готова',
  'status.essentia.optional': 'опционально',
  'status.metadata': 'Метаданные',

  'toolbar.chooseFolder': 'Выбрать папку',
  'toolbar.folderPlaceholder': 'Папка с музыкой',
  'toolbar.scan': 'Сканировать',
  'toolbar.cancel': 'Отмена',
  'toolbar.updateFFmpeg': 'Обновить FFmpeg',
  'toolbar.updatingFFmpeg': 'Обновление FFmpeg…',

  'warning.ffmpegUpdateFailed': 'Не удалось проверить обновление FFmpeg; CCML продолжит использовать установленную версию.',

  'roots.title': 'Папки медиатеки',
  'roots.managed': 'Подключено: {count}',
  'roots.lastScan': 'Последнее сканирование: {date}',
  'roots.notScanned': 'Ещё не сканировалась',
  'roots.remove': 'Удалить',
  'roots.removeConfirm': 'Также удалить проиндексированные треки из базы CCML? Аудиофайлы на диске удалены не будут.',

  'stats.tracks': 'Треки',
  'stats.artists': 'Исполнители',
  'stats.albums': 'Альбомы',
  'stats.duration': 'Длительность',
  'stats.size': 'Размер',
  'stats.duplicateGroups': 'Группы дубликатов',

  'scan.scanning': 'Сканирование медиатеки',
  'scan.cancelled': 'Сканирование отменено',
  'scan.finished': 'Сканирование завершено',
  'scan.processed': 'Обработано: {count}',
  'scan.added': 'Добавлено',
  'scan.updated': 'Обновлено',
  'scan.skipped': 'Пропущено',
  'scan.removed': 'Удалено из базы',
  'scan.failed': 'Ошибки',

  'library.title': 'Медиатека',
  'library.shown': 'Показано: {count}',
  'library.searchPlaceholder': 'Поиск по исполнителю, названию, альбому…',
  'library.search': 'Поиск',
  'library.duplicates': 'Дубликаты',
  'library.unknownArtist': 'Неизвестный исполнитель',

  'table.artist': 'Исполнитель',
  'table.title': 'Название',
  'table.album': 'Альбом',
  'table.time': 'Время',
  'table.codec': 'Кодек',
  'table.bpmKey': 'BPM / Тональность',

  'details.selectTrack': 'Выберите трек',
  'details.toolsHere': 'Здесь появятся инструменты трека',
  'details.path': 'Путь',
  'details.format': 'Формат',
  'details.loudness': 'Громкость',
  'details.notAnalyzed': 'Не анализировалось',
  'details.channelsShort': 'кан.',

  'actions.loudnessAnalysis': 'Анализ громкости',
  'actions.findMetadata': 'Найти метаданные',
  'actions.bpmKey': 'BPM и тональность',
  'actions.replayGain': 'Теги ReplayGain',

  'audio.title': 'Обработка аудио',
  'audio.targetLUFS': 'Целевая громкость LUFS',
  'audio.truePeak': 'True Peak, dBTP',
  'audio.preGain': 'Предусиление, dB',
  'audio.pitch': 'Pitch, полутоны',
  'audio.clippingRepair': 'Исправление клиппинга',
  'audio.multiband': 'Многополосная компрессия',
  'audio.limiter': 'Лимитер',
  'audio.keepOriginal': 'Сохранять оригинал при замене',
  'audio.createProcessedCopy': 'Создать обработанную копию',

  'organize.title': 'Организация / переименование',
  'organize.rootFolder': 'Корневая папка',
  'organize.rootPlaceholder': 'Пусто = текущая папка',
  'organize.template': 'Шаблон',
  'organize.regex': 'Регулярное выражение',
  'organize.regexPlaceholder': 'Необязательный шаблон',
  'organize.replace': 'Замена',
  'organize.replacePlaceholder': '$1 и т. п.',
  'organize.preview': 'Предпросмотр',
  'organize.apply': 'Переместить / переименовать',

  'metadata.title': 'Варианты метаданных',
  'metadata.artworkAlt': 'Обложка',
  'metadata.unknownAlbum': 'Неизвестный альбом',
  'metadata.match': 'совпадение {percent}%',

  'duplicates.title': 'Возможные дубликаты',
  'duplicates.files': 'Файлов: {count}',

  'footer.scanning': 'Сканирование…',
  'footer.working': 'Выполняется…',
  'footer.version': 'CCML v0.3 · RU/EN',

  'message.ready': 'Готово',
  'message.backendUnavailable': 'Backend Wails недоступен. Запустите приложение командой "wails dev".',
  'message.updatingBundledFFmpeg': 'Обновление встроенного FFmpeg…',
  'message.ffmpegUpdateFailed': 'Ошибка обновления FFmpeg: {error}',
  'message.checkingAudioTools': 'Проверка аудиоинструментов…',
  'message.ffmpegDetected': 'FFmpeg {version} · {source}',
  'message.ffmpegNotFound': 'FFmpeg/ffprobe не найдены',
  'message.checkingFFmpegUpdates': 'Проверка обновлений FFmpeg…',
  'message.ffmpegUpdated': 'FFmpeg обновлён до {version}',
  'message.ffmpegUpToDate': 'FFmpeg {version} уже актуален',
  'message.loadingLibrary': 'Загрузка медиатеки…',
  'message.tracksLoaded': 'Загружено треков: {count}',
  'message.chooseMusicFolder': 'Выберите папку с музыкой…',
  'message.chooseFolderFirst': 'Сначала выберите папку с музыкой',
  'message.scanningMusic': 'Сканирование музыки…',
  'message.scanCancelled': 'Сканирование отменено. Обработано файлов: {count}',
  'message.scanComplete': 'Готово: добавлено {added}, обновлено {updated}, пропущено {skipped}, удалено из базы {removed}, ошибок {failed}',
  'message.cancellingScan': 'Отмена сканирования…',
  'message.libraryFolderRemoved': 'Папка удалена из медиатеки',
  'message.analyzingLoudness': 'Анализ громкости…',
  'message.loudnessResult': 'Громкость {lufs} LUFS, true peak {peak} dBTP',
  'message.writingReplayGain': 'Запись метаданных ReplayGain…',
  'message.replayGainWritten': 'ReplayGain записан по результату {lufs} LUFS',
  'message.renderingProcessed': 'Создание обработанной копии…',
  'message.processed': 'Обработано: {path}',
  'message.analyzingBpmKey': 'Анализ BPM и тональности через Essentia…',
  'message.bpmKeyResult': '{bpm} BPM · {key} {scale}',
  'message.searchingMetadata': 'Поиск по источникам метаданных…',
  'message.metadataCandidatesFound': 'Найдено вариантов метаданных: {count}',
  'message.buildingTargetPath': 'Формирование целевого пути…',
  'message.renamePreviewUpdated': 'Предпросмотр переименования обновлён',
  'message.movingTrack': 'Перемещение / переименование трека…',
  'message.moved': 'Перемещено: {path}',
  'message.searchingDuplicates': 'Поиск возможных дубликатов…',
  'message.duplicateGroupsFound': 'Найдено групп дубликатов: {count}',

  'duration.hourShort': 'ч',
  'duration.dayShort': 'д',
}

export type TranslateParams = Record<string, string | number>

export function translate(language: AppLanguage, key: TranslationKey, params?: TranslateParams): string {
  const template = language === 'ru' ? ru[key] : en[key]
  if (!params) return template

  return template.replace(/\{([a-zA-Z0-9_]+)\}/g, (match, name: string) => {
    const value = params[name]
    return value === undefined ? match : String(value)
  })
}

export function detectInitialLanguage(): AppLanguage {
  if (typeof window !== 'undefined') {
    try {
      const stored = window.localStorage.getItem(STORAGE_KEY)
      if (stored === 'ru' || stored === 'en') return stored
    } catch {
      // Local storage may be unavailable in hardened environments; fall back to OS language.
    }
  }

  if (typeof navigator !== 'undefined') {
    const languages = navigator.languages?.length ? navigator.languages : [navigator.language]
    if (languages.some((language) => language.toLowerCase().startsWith('ru'))) return 'ru'
  }

  return 'en'
}

export function saveLanguage(language: AppLanguage): void {
  if (typeof window === 'undefined') return
  try {
    window.localStorage.setItem(STORAGE_KEY, language)
  } catch {
    // Language selection remains valid for the current session even if persistence is blocked.
  }
}

export function localeFor(language: AppLanguage): string {
  return language === 'ru' ? 'ru-RU' : 'en-US'
}
