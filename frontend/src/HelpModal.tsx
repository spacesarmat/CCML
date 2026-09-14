import { useEffect, useMemo, useRef, useState } from 'react'
import type { AppLanguage } from './i18n'

type HelpRow = {
  keys?: string[]
  action: string
  note?: string
}

type HelpSection = {
  id: string
  title: string
  summary: string
  rows?: HelpRow[]
  bullets?: string[]
}

type HelpCopy = {
  title: string
  subtitle: string
  search: string
  close: string
  noResults: string
  contents: string
  sections: HelpSection[]
}

type Props = {
  language: AppLanguage
  open: boolean
  version: string
  onClose: () => void
}

const HELP: Record<AppLanguage, HelpCopy> = {
  ru: {
    title: 'Справка CCML',
    subtitle: 'Медиатека, метаданные, анализ и обработка музыки',
    search: 'Поиск по справке: обложка, LUFS, Ctrl+F, метаданные…',
    close: 'Закрыть',
    noResults: 'По этому запросу ничего не найдено.',
    contents: 'Разделы',
    sections: [
      {
        id: 'start',
        title: 'Быстрый старт',
        summary: 'Основной рабочий цикл CCML.',
        bullets: [
          'Добавьте папку медиатеки слева и запустите сканирование. CCML индексирует аудиофайлы и отображает их в центральной таблице.',
          'Выберите трек в таблице. Справа откроется Инспектор с обложкой, плеером и вкладками Теги, Метаданные, Анализ и Организация.',
          'Для нескольких треков используйте Ctrl+клик, Shift+клик или Ctrl+A. Массовые действия выполняются только над выбранными строками.',
          'Состояние FFmpeg, Essentia и число активных источников метаданных показаны в верхней панели.',
        ],
      },
      {
        id: 'shortcuts',
        title: 'Горячие клавиши',
        summary: 'Навигация по таблице и быстрые действия без мыши.',
        rows: [
          {keys: ['F1'], action: 'Открыть эту справку.'},
          {keys: ['↑', '↓'], action: 'Перейти на предыдущую или следующую строку.'},
          {keys: ['PageUp', 'PageDown'], action: 'Перейти примерно на 10 строк.'},
          {keys: ['Home', 'End'], action: 'Первая или последняя видимая строка.'},
          {keys: ['Shift', '↑ / ↓'], action: 'Расширить непрерывное выделение.'},
          {keys: ['Ctrl', '↑ / ↓'], action: 'Перемещать активную строку, сохраняя текущее множественное выделение.'},
          {keys: ['Ctrl', 'клик'], action: 'Добавить или убрать отдельный трек из выделения.'},
          {keys: ['Shift', 'клик'], action: 'Выделить диапазон от опорной строки.'},
          {keys: ['Ctrl+A'], action: 'Выбрать все видимые строки текущего фильтра.'},
          {keys: ['Ctrl+F'], action: 'Перейти в поле поиска медиатеки.'},
          {keys: ['Enter'], action: 'Сделать активную строку единственным выбранным треком и открыть вкладку Теги.'},
          {keys: ['Space'], action: 'Play/Pause для одного выбранного трека.'},
          {keys: ['Esc'], action: 'Снять выделение и активную строку.'},
        ],
        bullets: [
          'Горячие клавиши таблицы намеренно не срабатывают, пока курсор находится в поле ввода, select, кнопке или слайдере.',
          'Активная строка имеет отдельную рамку; она может отличаться от набора строк, выбранных для массовой операции.',
        ],
      },
      {
        id: 'library',
        title: 'Медиатека и таблица',
        summary: 'Поиск, фильтрация и выбор треков.',
        bullets: [
          'Поиск медиатеки интерактивный: после ввода текста CCML автоматически обновляет результаты примерно через 280 мс. Enter или кнопка «Поиск» запускают запрос немедленно; «Очистить» сбрасывает строку и возвращает полный список.',
          'Быстрые фильтры статуса отдельно показывают все треки, записи без изменений после обогащения и записи, где обработка метаданных завершилась ошибкой.',
          'Кнопка «Фильтры» добавляет комбинируемые фильтры по Исполнителю, Жанру, Лейблу, Кодеку, Тональности, Году, BPM, LUFS и наличию Обложки. Категории работают по AND, несколько значений внутри одной категории — по OR.',
          'Наличие embedded artwork индексируется в SQLite. После установки Stage 13.2 один обычный повторный скан библиотеки заполнит индекс для старых файлов; далее неизменённые файлы снова пропускаются быстро. Новые файлы индексируются сразу.',
          'Активные фильтры сохраняются между запусками и работают поверх интерактивного поиска. «Сбросить фильтры» возвращает полный результат текущего поискового запроса.',
          'Обычный клик выбирает одну строку. Чекбоксы, Ctrl+клик и Shift+клик используются для массового выделения.',
          'При перемещении клавишами активная строка автоматически прокручивается в видимую область.',
          'Кнопка «Колонки» над таблицей позволяет включать и скрывать поля. Порядок меняется перетаскиванием маркеров; раскладка сохраняется между запусками.',
          'Клик по заголовку сортирует таблицу, повторный клик меняет направление. Shift+клик добавляет до четырёх уровней сортировки; клавиатурная навигация и Shift-выделение следуют видимому отсортированному порядку.',
          'Ширина колонок меняется перетаскиванием правой границы заголовка. Двойной клик по границе подбирает автоширину по видимому содержимому. Ширины сохраняются между запусками; «Сбросить» возвращает стандартные размеры.',
          'Колонка «Обложка» показывает миниатюру embedded front cover. Обложки загружаются лениво только для видимых строк, кешируются на время сессии и масштабируются вместе с интерфейсом. Колонку можно скрыть или переместить через «Колонки».',
          'Кнопка «Пресеты» открывает компактное всплывающее окно рядом с «Колонки». Оба popup автоматически закрываются вскоре после ухода курсора и не могут быть открыты одновременно.',
          'Доступны готовые DJ, Метаданные, Технический и Компактный пресеты. Можно сохранить текущее отображение как свой пресет, затем применить, обновить или удалить его. Пресет хранит колонки, порядок, ширины и сортировку; поиск, фильтры Stage 13 и быстрый статус метаданных не меняются.',
          'Интерфейс использует единый визуальный стиль и компактные полосы прокрутки. Полоса появляется только когда содержимое действительно не помещается в доступную область.',
          'В Настройки → Оформление доступны темы Midnight, Graphite, Ocean и Light. Тема применяется сразу, сохраняется локально и восстанавливается при следующем запуске.',
          'Там же можно выбрать масштаб интерфейса 80%, 90%, 100%, 110% или 125%. Масштаб применяется сразу ко всему рабочему пространству и сохраняется между запусками.',
          'После визуальной полировки кнопки, поля, карточки, модальные окна, Help, Jobs и Inspector используют общую плотность и состояния. При уменьшенном окне и масштабе 80–125% интерфейс адаптирует ширины и отступы без изменения данных.',
          'Тема также синхронизирует системное оформление окна. Light использует светлые поверхности во всей библиотеке, Inspector, боковой панели, задачах и настройках.',
          'Во вкладке Инспектор → Метаданные отчёты источников, карточки кандидатов и «Объединение метаданных» также полностью используют цвета выбранной темы.',
          'Таблица загружает все совпавшие треки постранично по 1000 записей, поэтому библиотека больше не ограничена первыми 500 файлами.',
        ],
      },
      {
        id: 'player',
        title: 'Плеер Inspector',
        summary: 'Прослушивание выбранного трека прямо в CCML.',
        bullets: [
          'Плеер показывает Play/Pause, текущую позицию, общую длительность, перемотку и громкость.',
          'Для совместимых форматов CCML сначала пытается воспроизводить исходный файл напрямую.',
          'Если WebView не принимает исходный поток, CCML автоматически готовит совместимый MP3 preview через FFmpeg. Исходный файл при этом не изменяется.',
          'При смене выбранного трека позиция воспроизведения сбрасывается.',
        ],
      },
      {
        id: 'tags',
        title: 'Теги',
        summary: 'Ручное редактирование встроенных тегов аудиофайла.',
        bullets: [
          'Для одного трека можно изменять поля напрямую. При выборе нескольких треков включается массовый Tag Editor.',
          'У каждого поля массового редактора есть режим «Не менять / Заменить / Очистить». «Не менять» сохраняет исходное значение каждого файла; «Заменить» записывает одно новое значение во всю пачку; «Очистить» удаляет поле во всех выбранных треках.',
          'Если исходные значения различаются, поле помечается «Разные значения». Перед записью используйте «Предпросмотр»; вся массовая операция создаётся как один общий Undo change-set.',
          'Блок «Преобразования» выполняет операции отдельно над текущим значением каждого файла: trim краёв, верхний/нижний регистр, поиск/замена, префикс, суффикс и копирование Artist ↔ Album Artist. Для текущих параметров преобразования Preview обязателен перед Apply.',
          'Изменения тегов записываются в файл; CCML ведёт историю изменений для поддерживаемых операций.',
          'Обложка является частью встроенных тегов и хранится внутри поддерживаемого аудиофайла.',
        ],
      },
      {
        id: 'metadata',
        title: 'Метаданные и обложки',
        summary: 'Поиск данных по внешним каталогам и объединение результатов.',
        bullets: [
          'Вкладка Метаданные запрашивает включённые источники: например MusicBrainz, Deezer, Apple iTunes и другие настроенные провайдеры.',
          'Traxsource сначала запрашивается напрямую. Если сайт отвечает Cloudflare/human verification, CCML может автоматически использовать настроенный JSON fallback API; ключ задаётся в Настройки → Traxsource.',
          'Процент рядом с кандидатом — оценка совпадения. Проверяйте исполнителя, название, длительность, релиз и другие поля перед записью.',
          '«Применить теги» записывает только текстовые/числовые метаданные. Чтобы встроить картинку, используйте отдельное действие «Применить с обложкой», когда оно доступно.',
          'Apple iTunes artwork после соответствующего hotfix разрешён для явного встраивания. Старый кэш поиска может требовать «Обновить источники».',
          'Если обложка уже встроена в MP3, но имеет нестандартный APIC-тип, Inspector использует Front Cover в приоритете, а при его отсутствии — первую валидную embedded image.',
          '«Только отсутствующие» полезно включать при автоматическом обогащении, чтобы не перезаписывать уже заполненные поля.',
        ],
      },
      {
        id: 'analysis',
        title: 'Анализ аудио',
        summary: 'Измерение громкости и музыкальных характеристик.',
        bullets: [
          'Анализ громкости использует FFmpeg loudnorm/EBU R128 и показывает Integrated LUFS и True Peak.',
          'ReplayGain записывает соответствующие теги на основе измерения громкости.',
          'BPM & Key использует Essentia, если инструмент доступен в системе.',
          'Во вкладке Анализ отображаются формат, sample rate, число каналов и сохранённые результаты анализа.',
        ],
      },
      {
        id: 'processing',
        title: 'Обработка аудио',
        summary: 'Создание обработанной копии без скрытого изменения исходника.',
        rows: [
          {action: 'Target LUFS', note: 'Целевая интегральная громкость loudnorm, например −14 LUFS.'},
          {action: 'True Peak', note: 'Максимальный целевой true peak, например −1 dBTP.'},
          {action: 'Pre Gain', note: 'Предварительное усиление/ослабление перед основной обработкой.'},
          {action: 'Clipping Repair', note: 'Дополнительное восстановление клиппинга, если включено.'},
          {action: 'Multiband', note: 'Многополосная компрессия, если включена.'},
          {action: 'Limiter', note: 'Ограничение пиков после основной цепочки.'},
        ],
        bullets: [
          'CCML использует двухпроходный loudnorm при валидных измерениях и переходит на dynamic mode при аномальных измеренных значениях.',
          'После рендера выходной файл проверяется и повторно измеряется loudnorm; итоговая громкость берётся из повторного измерения.',
          'При обработке сохраняются metadata, chapters и embedded artwork для поддерживаемого потока.',
          '«Создать обработанную копию» не требует ручной перезаписи исходного файла.',
        ],
      },
      {
        id: 'jobs',
        title: 'Задачи',
        summary: 'Фоновые длительные операции.',
        bullets: [
          'Кнопка Задачи открывает очередь фоновых операций CCML.',
          'Задачи могут продолжаться независимо от текущей вкладки интерфейса.',
          'Дополнение метаданных обрабатывает несколько треков одновременно. В Настройках можно выбрать 1–16 параллельных треков; значение по умолчанию — 10. В Задачах отображаются фактическое число активных worker’ов и все треки, которые прямо сейчас выполняются. Ограничения отдельных провайдеров продолжают соблюдать их rate limits.',
          'Для «Дополнить выбранные» доступны режимы Авто, Быстрый и Полный. Авто сначала использует быстрые источники и обращается к MusicBrainz, Apple iTunes, TheAudioDB, Discogs и Traxsource только если точного результата ещё нет. Точное совпадение высокой уверенности может завершить поиск раньше.',
          'В деталях выполненных файлов Задачи показывают режим, время поиска, число ответивших/пропущенных источников и факт раннего завершения.',
          'Путь файла в Задачах можно нажать: окно задач закроется, CCML перейдёт в Библиотеку, выделит соответствующий трек и прокрутит таблицу к нему. Это работает и для списка активных worker’ов.',
          'Для поддерживаемых задач доступны пауза, продолжение, отмена и повтор ошибок.',
          'После завершения задачи медиатека автоматически обновляет связанные данные.',
        ],
      },
      {
        id: 'organize',
        title: 'Организация файлов',
        summary: 'Предпросмотр и применение правил имени/пути.',
        bullets: [
          'Укажите корневую папку и шаблон пути. Доступны поля вроде исполнителя, альбома, номера и названия трека.',
          'Regex-поля позволяют дополнительно преобразовать сформированное имя.',
          'Сначала используйте Предпросмотр, затем применяйте перемещение/переименование.',
        ],
      },
      {
        id: 'troubleshooting',
        title: 'Если что-то не работает',
        summary: 'Наиболее частые причины и проверки.',
        rows: [
          {action: 'Нет обложки после поиска', note: 'Убедитесь, что нажата именно «Применить с обложкой», а не «Применить теги». При старом результате нажмите «Обновить источники».'},
          {action: 'Обложка есть в файле, но не в Inspector', note: 'Перевыберите трек. CCML перечитает media cache и embedded artwork.'},
          {action: 'Плеер не запускается', note: 'CCML попробует compatibility preview. Проверьте зелёный статус FFmpeg в верхней панели.'},
          {action: 'Нет BPM/Key', note: 'Проверьте состояние Essentia. Этот инструмент является отдельной зависимостью.'},
          {action: 'Источник метаданных ошибается', note: 'Откройте Настройки, проверьте включение и учётные данные провайдера, затем выполните тест источников.'},
          {action: 'Обложка недоступна по сети или URL', note: 'При автоматическом дополнении ошибки artwork HTTP 404/403/5xx, DNS, timeout, пустого или неподдерживаемого изображения больше не валят весь трек: CCML сохраняет доступные теги без обложки и показывает предупреждение в Задачах.'},
          {action: 'FFmpeg недоступен', note: 'Аудиоанализ, совместимый preview и обработка, требующие FFmpeg, будут недоступны до восстановления toolchain.'},
        ],
      },
    ],
  },
  en: {
    title: 'CCML Help',
    subtitle: 'Library, metadata, analysis and music processing',
    search: 'Search help: artwork, LUFS, Ctrl+F, metadata…',
    close: 'Close',
    noResults: 'Nothing matched this help search.',
    contents: 'Contents',
    sections: [
      {
        id: 'start',
        title: 'Quick start',
        summary: 'The main CCML workflow.',
        bullets: [
          'Add a library folder on the left and run a scan. CCML indexes audio files and lists them in the central table.',
          'Select a track in the table. Inspector on the right shows artwork, the player and Tags, Metadata, Analysis and Organize tabs.',
          'Use Ctrl+click, Shift+click or Ctrl+A for multiple tracks. Batch operations apply only to selected rows.',
          'FFmpeg, Essentia and the number of active metadata providers are shown in the top bar.',
        ],
      },
      {
        id: 'shortcuts',
        title: 'Keyboard shortcuts',
        summary: 'Navigate the table and trigger common actions without the mouse.',
        rows: [
          {keys: ['F1'], action: 'Open this help.'},
          {keys: ['↑', '↓'], action: 'Move to the previous or next row.'},
          {keys: ['PageUp', 'PageDown'], action: 'Jump roughly 10 rows.'},
          {keys: ['Home', 'End'], action: 'Move to the first or last visible row.'},
          {keys: ['Shift', '↑ / ↓'], action: 'Extend the contiguous selection.'},
          {keys: ['Ctrl', '↑ / ↓'], action: 'Move the active row while preserving the current batch selection.'},
          {keys: ['Ctrl', 'click'], action: 'Toggle one row in a multi-selection.'},
          {keys: ['Shift', 'click'], action: 'Select a range from the anchor row.'},
          {keys: ['Ctrl+A'], action: 'Select every visible row in the current filter.'},
          {keys: ['Ctrl+F'], action: 'Focus the library search field.'},
          {keys: ['Enter'], action: 'Make the active row the single selection and open Tags.'},
          {keys: ['Space'], action: 'Play/Pause for one selected track.'},
          {keys: ['Esc'], action: 'Clear selection and active row.'},
        ],
        bullets: [
          'Table shortcuts are intentionally disabled while focus is inside an input, select, button or slider.',
          'The active row has its own outline and may differ from the set of rows selected for a batch operation.',
        ],
      },
      {
        id: 'library',
        title: 'Library and table',
        summary: 'Search, filter and select tracks.',
        bullets: [
          'Library search is interactive: after typing, CCML automatically refreshes results after about 280 ms. Enter or Search runs immediately; Clear resets the query and restores the full list.',
          'Quick status filters can show all tracks, unchanged enrichment results, or metadata jobs that failed.',
          'The Filters control adds combinable Artist, Genre, Label, Codec, Key, Year, BPM, LUFS and Cover-presence filters. Categories combine with AND; multiple values inside one category combine with OR.',
          'Embedded-artwork presence is indexed in SQLite. After installing Stage 13.2, run one normal library scan to populate the index for existing files; unchanged files return to fast incremental skips afterwards. New files are indexed immediately.',
          'Active filters persist between launches and work on top of interactive search. Clear filters restores the complete result of the current search query.',
          'A normal click selects one row. Checkboxes, Ctrl+click and Shift+click are for multi-selection.',
          'Keyboard navigation automatically scrolls the active row into view.',
          'The Columns control above the table shows or hides fields. Reorder them by dragging the handles; the layout persists between launches.',
          'Click a column header to sort; click again to reverse it. Shift+click adds up to four sort levels. Keyboard navigation and Shift-selection follow the visible sorted order.',
          'Resize a column by dragging the right edge of its header. Double-click the edge to auto-fit visible content. Widths persist between launches; Reset restores the default sizes.',
          'The Cover column shows an embedded front-cover thumbnail. Covers load lazily only for visible rows, are cached for the session and scale with the interface. Hide or reorder the column through Columns.',
          'The Presets button opens a compact popup next to Columns. Both popups auto-close shortly after the pointer leaves them and cannot stay open at the same time.',
          'Built-in DJ, Metadata, Technical and Compact presets are available. You can save the current display as a custom preset, then apply, update or delete it. A preset stores columns, order, widths and sorting; interactive search, Stage 13 filters and quick metadata status stay unchanged.',
          'The interface uses a unified visual system and compact scrollbars. A scrollbar appears only when content actually exceeds the available area.',
          'Settings → Appearance offers Midnight, Graphite, Ocean and Light themes. The theme applies immediately, is stored locally and is restored on the next launch.',
          'The same Appearance section offers 80%, 90%, 100%, 110% and 125% interface scaling. Scaling applies immediately to the whole workspace and persists between launches.',
          'The final visual pass gives controls, cards, modals, Help, Jobs and Inspector a consistent density and state styling. Smaller windows and 80–125% scaling adapt widths and spacing without changing library data.',
          'The selected theme also synchronizes native window chrome. Light uses light surfaces consistently across Library, Inspector, sidebar, Jobs and Settings.',
          'Inspector → Metadata provider reports, candidate cards and Metadata Merge now fully use the selected theme as well.',
          'The table loads every matching track in 1000-row pages, so the library is no longer limited to the first 500 files.',
        ],
      },
      {
        id: 'player',
        title: 'Inspector player',
        summary: 'Listen to the selected track directly in CCML.',
        bullets: [
          'The player provides Play/Pause, current and total time, seeking and volume.',
          'For compatible formats CCML first tries to stream the original file directly.',
          'If WebView rejects the source, CCML automatically creates a compatible MP3 preview with FFmpeg. The source file is not modified.',
          'Playback position resets when the selected track changes.',
        ],
      },
      {
        id: 'tags',
        title: 'Tags',
        summary: 'Edit embedded audio tags manually.',
        bullets: [
          'For one track fields can be edited directly. Selecting several tracks switches to the batch Tag Editor.',
          'Every batch field has Keep / Replace / Clear mode. Keep preserves each file’s original value; Replace writes one new value to the whole batch; Clear removes the field from all selected tracks.',
          'Fields whose source values differ are marked Mixed values. Use Preview before writing; the whole batch operation is recorded as one undoable change set.',
          'The Transforms block operates on each file’s current value independently: trim edges, uppercase/lowercase, find/replace, prefix, suffix and Artist ↔ Album Artist copy. Preview is mandatory for the current transform settings before Apply becomes available.',
          'Tag changes are written to the file; CCML keeps change history for supported operations.',
          'Artwork is embedded metadata and is stored inside supported audio files.',
        ],
      },
      {
        id: 'metadata',
        title: 'Metadata and artwork',
        summary: 'Search external catalogs and merge provider results.',
        bullets: [
          'Metadata queries enabled providers such as MusicBrainz, Deezer, Apple iTunes and other configured sources.',
          'Traxsource is tried directly first. If the site responds with Cloudflare/human verification, CCML can automatically use the configured JSON fallback API; set its key under Settings → Traxsource.',
          'The candidate percentage is a match score. Check artist, title, duration, release and other fields before writing.',
          'Apply tags writes textual/numeric metadata only. Use the separate Apply with artwork action when you also want to embed the image.',
          'Apple iTunes artwork is available for explicit embedding after the related hotfix. Old cached search results may require Refresh sources.',
          'For embedded MP3 artwork Inspector prefers Front Cover and falls back to the first valid embedded image when necessary.',
          'Only missing is useful for enrichment when existing populated fields should be preserved.',
        ],
      },
      {
        id: 'analysis',
        title: 'Audio analysis',
        summary: 'Measure loudness and musical characteristics.',
        bullets: [
          'Loudness analysis uses FFmpeg loudnorm/EBU R128 and reports Integrated LUFS and True Peak.',
          'ReplayGain writes corresponding tags from the loudness measurement.',
          'BPM & Key uses Essentia when that tool is available.',
          'Analysis also shows format, sample rate, channels and stored analysis results.',
        ],
      },
      {
        id: 'processing',
        title: 'Audio processing',
        summary: 'Create a processed copy without silently replacing the source.',
        rows: [
          {action: 'Target LUFS', note: 'Target integrated loudness for loudnorm, for example −14 LUFS.'},
          {action: 'True Peak', note: 'Target maximum true peak, for example −1 dBTP.'},
          {action: 'Pre Gain', note: 'Gain adjustment before the main processing chain.'},
          {action: 'Clipping Repair', note: 'Optional clipping repair stage.'},
          {action: 'Multiband', note: 'Optional multiband compression.'},
          {action: 'Limiter', note: 'Peak limiting after the main chain.'},
        ],
        bullets: [
          'CCML uses two-pass loudnorm when measurements are valid and falls back to dynamic mode for anomalous measured values.',
          'After rendering, the output is validated and measured again; the final loudness comes from that second measurement.',
          'Metadata, chapters and embedded artwork are preserved by the supported processing path.',
          'Create processed copy does not require replacing the source audio manually.',
        ],
      },
      {
        id: 'jobs',
        title: 'Jobs',
        summary: 'Long-running background operations.',
        bullets: [
          'Jobs opens the persistent CCML background-work queue.',
          'Supported work can continue independently of the currently visible screen.',
          'Metadata enrichment processes multiple tracks concurrently. Settings lets you choose 1–16 parallel tracks; the default is 10. Jobs shows the actual active-worker count and every track currently executing. Provider-specific rate limits remain enforced.',
          'Enrich selected offers Auto, Fast and Full search modes. Auto starts with quick providers and only escalates to MusicBrainz, Apple iTunes, TheAudioDB, Discogs and Traxsource when an exact result is still missing. An exact high-confidence result can stop the search early.',
          'Completed item details in Jobs show the mode, search time, responded/skipped provider counts and whether early stopping occurred.',
          'File paths in Jobs are clickable: CCML closes Jobs, switches to Library, selects the matching track and scrolls it into view. Active-worker paths work the same way.',
          'Supported jobs provide pause, resume, cancel and retry-failed controls.',
          'Related library data is refreshed after jobs finish.',
        ],
      },
      {
        id: 'organize',
        title: 'File organization',
        summary: 'Preview and apply path/name rules.',
        bullets: [
          'Choose a root folder and path template. Fields can include artist, album, track number and title.',
          'Regex fields can further transform the generated name.',
          'Use Preview before applying a move or rename.',
        ],
      },
      {
        id: 'troubleshooting',
        title: 'Troubleshooting',
        summary: 'Common causes and quick checks.',
        rows: [
          {action: 'Artwork found but not saved', note: 'Use Apply with artwork rather than Apply tags. Refresh sources if the candidate came from an old cache.'},
          {action: 'Artwork is embedded but Inspector is empty', note: 'Re-select the track so CCML refreshes its media cache and embedded artwork.'},
          {action: 'Player does not start', note: 'CCML will try a compatibility preview. Check the FFmpeg status in the top bar.'},
          {action: 'No BPM/Key', note: 'Check Essentia status. It is a separate analysis dependency.'},
          {action: 'Metadata provider fails', note: 'Open Settings, verify provider enablement and credentials, then run the provider test.'},
          {action: 'Artwork URL/network failure', note: 'During automatic enrichment, artwork HTTP 404/403/5xx, DNS, timeout, empty-image or unsupported-image failures no longer fail the entire track. CCML keeps the available tags without artwork and shows a warning in Jobs.'},
          {action: 'FFmpeg unavailable', note: 'FFmpeg-dependent analysis, compatibility preview and processing remain unavailable until the toolchain is restored.'},
        ],
      },
    ],
  },
}

function HelpModal({language, open, version, onClose}: Props) {
  const [query, setQuery] = useState('')
  const searchRef = useRef<HTMLInputElement | null>(null)
  const copy = HELP[language]
  const displayVersion = version.split('·')[0].trim()

  useEffect(() => {
    if (!open) return
    const frame = window.requestAnimationFrame(() => searchRef.current?.focus())
    const onKeyDown = (event: KeyboardEvent) => {
      if (event.key === 'Escape') {
        event.preventDefault()
        onClose()
      }
    }
    window.addEventListener('keydown', onKeyDown)
    return () => {
      window.cancelAnimationFrame(frame)
      window.removeEventListener('keydown', onKeyDown)
    }
  }, [open, onClose])

  useEffect(() => {
    if (!open) setQuery('')
  }, [open])

  const sections = useMemo(() => {
    const needle = query.trim().toLocaleLowerCase(language === 'ru' ? 'ru-RU' : 'en-US')
    if (!needle) return copy.sections
    return copy.sections.filter((section) => {
      const haystack = [
        section.title,
        section.summary,
        ...(section.bullets ?? []),
        ...(section.rows ?? []).flatMap((row) => [...(row.keys ?? []), row.action, row.note ?? '']),
      ].join(' ').toLocaleLowerCase(language === 'ru' ? 'ru-RU' : 'en-US')
      return haystack.includes(needle)
    })
  }, [copy.sections, language, query])

  if (!open) return null

  function goToSection(id: string) {
    document.getElementById(`help-${id}`)?.scrollIntoView({behavior: 'smooth', block: 'start'})
  }

  return (
    <div className="settings-backdrop help-backdrop" role="presentation" onMouseDown={(event) => event.target === event.currentTarget && onClose()}>
      <section className="settings-modal panel help-modal" role="dialog" aria-modal="true" aria-label={copy.title}>
        <div className="settings-header help-header">
          <div>
            <h2>{copy.title}</h2>
            <span>{copy.subtitle} · {displayVersion}</span>
          </div>
          <button onClick={onClose} aria-label={copy.close} title={copy.close}>×</button>
        </div>

        <div className="help-search-row">
          <input
            ref={searchRef}
            value={query}
            onChange={(event) => setQuery(event.target.value)}
            placeholder={copy.search}
            aria-label={copy.search}
          />
          {query && <button type="button" onClick={() => setQuery('')}>×</button>}
        </div>

        <div className="help-layout">
          <nav className="help-nav" aria-label={copy.contents}>
            <strong>{copy.contents}</strong>
            {sections.map((section) => (
              <button type="button" key={section.id} onClick={() => goToSection(section.id)}>{section.title}</button>
            ))}
          </nav>

          <div className="help-content">
            {sections.length === 0 && <div className="help-empty">{copy.noResults}</div>}
            {sections.map((section) => (
              <article className="help-section" id={`help-${section.id}`} key={section.id}>
                <header>
                  <h3>{section.title}</h3>
                  <p>{section.summary}</p>
                </header>

                {section.rows && section.rows.length > 0 && (
                  <div className="help-rows">
                    {section.rows.map((row, index) => (
                      <div className="help-row" key={`${section.id}-${index}`}>
                        <div className="help-row-command">
                          {(row.keys ?? []).map((key, keyIndex) => <kbd key={`${key}-${keyIndex}`}>{key}</kbd>)}
                          {(row.keys?.length ?? 0) === 0 && <strong>{row.action}</strong>}
                        </div>
                        <div>
                          {(row.keys?.length ?? 0) > 0 && <strong>{row.action}</strong>}
                          {row.note && <span>{row.note}</span>}
                        </div>
                      </div>
                    ))}
                  </div>
                )}

                {section.bullets && section.bullets.length > 0 && (
                  <ul>
                    {section.bullets.map((item, index) => <li key={`${section.id}-bullet-${index}`}>{item}</li>)}
                  </ul>
                )}
              </article>
            ))}
          </div>
        </div>

        <div className="settings-footer help-footer">
          <span>F1 · {displayVersion}</span>
          <button className="primary" onClick={onClose}>{copy.close}</button>
        </div>
      </section>
    </div>
  )
}

export default HelpModal
