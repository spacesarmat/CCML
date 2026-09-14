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
          'Stage 17.2 виртуализирует большие таблицы: при количестве более 240 строк в DOM находятся только видимые строки и небольшой запас вокруг них. Полный список, сортировка, фильтры, выделение и клавиатурная навигация при этом продолжают работать со всей библиотекой.',
          'Проверка выбранности также использует Set вместо многократного линейного поиска. Это особенно уменьшает задержки при Ctrl+A, массовом выборе и библиотеках в несколько тысяч файлов.',
          'Раздел «Дубликаты» группирует библиотеку по трём уровням уверенности: одинаковый ISRC, точные Artist/Title с близкой длительностью и возможные версии, где из Title временно исключаются пометки Remix/Edit/Intro/Clean и другие version qualifiers. Возможные версии не считаются подтверждёнными дублями.',
          'Внутри группы можно сравнить файл, кодек, битрейт, sample rate, размер, длительность и ISRC. Stage 16.1 ничего не удаляет и не перемещает — экран только диагностический.',
          'Stage 16.2 добавляет эвристическую оценку качества 0–100: до 80 баллов за аудио-параметры (lossless/lossy, bitrate, sample rate, channels) и до 20 за полноту тегов/ISRC/embedded cover. Файл с уникально лучшим score помечается «Лучшее качество» и поднимается вверх группы.',
          'При полном равенстве score CCML не выбирает искусственного победителя. В группе «Возможный дубль» лидер качества означает только технически предпочтительный файл и не доказывает, что Remix/Edit/Intro являются взаимозаменяемыми версиями.',
          'Stage 16.3 добавляет ручное выделение файлов в группе. «Оставить лучший» только выделяет все остальные строки — никакое действие не запускается автоматически. Backend не позволяет одной операцией убрать все файлы из затронутой группы.',
          '«В карантин» просит выбрать папку вне управляемых папок медиатеки, переносит файлы в подпапку «CCML Duplicates» и затем удаляет их записи из индекса. При ошибке SQLite перенос откатывается.',
          '«Удалить» является безвозвратным действием: перед запуском нужно ввести DELETE. Файлы сначала временно переименовываются, затем записи удаляются одной SQLite-транзакцией; если транзакция не проходит, исходные имена файлов восстанавливаются.',
          'Stage 16.4 добавляет «Проверить аудио». FFmpeg декодирует все файлы текущей группы в mono PCM 4 kHz и сравнивает нормализованные признаки энергии, roughness и zero-crossing с выравниванием до ±6 секунд. Результаты: «Совпадает», «Похоже», «Отличается» или «Ошибка».',
          'Статус «Совпадает» требует очень высокой похожести формы сигнала и близкой полной длительности. Проверка остаётся эвристикой: она помогает принять решение перед карантином/удалением, но сама не запускает действия и не заменяет прослушивание сомнительных Remix/Edit/Intro версий.',
          'Stage 16.4.2 меняет сам способ отображения больших результатов: вместо сотен раскрывающихся карточек слева показывается прокручиваемый список групп, а справа — детали только выбранной группы. Это сохраняет читаемость даже при сотнях групп и освобождает ширину для таблицы сравнения и действий.',
          'Stage 17.1 улучшает поиск метаданных для version-тегов. Если исходный Title с пометками вроде (Extended Mix), (Dirty), (Clean), (Intro), (Radio Edit), (Original Mix), Remix/Rework/Bootleg, Acapella и т. п. не даёт подходящего результата, CCML автоматически повторяет поиск по базовому названию.',
          'Fallback удаляет только распознанные хвостовые version-теги из поискового запроса. При применении метаданных локальная версия названия добавляется обратно, поэтому поиск «Track» не превращает локальный «Track (Extended Mix)» в обычный «Track». Поведение работает и для фонового массового дополнения метаданных.',
          'Stage 17.3 исправляет обложки Spotify: album.images больше не блокируются флагом ArtworkEmbeddable=false. CCML выбирает самую большую валидную HTTP/HTTPS-обложку Spotify и может скачать/встроить её через общий механизм Tag Editor, если включено добавление artwork.',
          'Stage 18.0 закладывает основу DJ Pool Sources. MetadataCandidate теперь переносит BPM, Key и KeyScale, а источники могут помечаться типом DJ Pool без изменения интерфейса существующих провайдеров.',
          'BPM/Key можно выбрать в Metadata Merge и записать вместе с остальными метаданными. Изменение участвует в обычной Tag History/Undo, обновляет SQLite и считывается обратно при последующем сканировании файла.',
          'Stage 18.0 не подключает внешние DJ-pool сайты. Следующие этапы добавляют провайдеры по одному, начиная с MUZVIZOR.',
          'Stage 18.0.1 унифицирует «Дополнить данные» и «Найти метаданные»: enrichment больше не использует отдельные Auto/Fast/Full ветки и раннюю остановку. Для каждого трека вызывается тот же lookupMetadataQuery, поэтому набор источников, кэш, fallback Title, scoring и выбор suggested-кандидата полностью совпадают.',
          'Порог совпадения, «Только отсутствующие поля» и добавление обложки остаются настройками автоматического применения. Они не меняют сам принцип поиска.',
          'Stage 18.1 добавляет MUZVIZOR как опциональный DJ Pool источник. Провайдер использует только публично доступные страницы и переносит Artist, Title, BPM, Camelot Key и Genre; авторизация, подписка и скачивание аудиофайлов не затрагиваются.',
          'MUZVIZOR включается отдельно в Settings. Результаты проходят обычный CCML scoring и Stage 17.1 fallback по очищенному Title, поэтому версия локального названия сохраняется при применении метаданных.',
          'Публичный поиск MUZVIZOR не является документированным developer API. CCML использует консервативный HTML parser и отбрасывает результаты, которые не совпадают по Artist/Title; при изменении сайта источник может временно вернуть 0 результатов вместо подстановки случайного трека.',
          'Hotfix 18.1.1 исправляет редкую потерю обложки при «Дополнить данные»: если выбранная CCML Merge обложка не скачивается, CCML пробует другие доверенные embeddable-artwork URL из того же результата поиска и только затем продолжает без обложки.',
          'Stage 18.2 добавляет RemixPool как опциональный DJ Pool источник. CCML читает только подтверждённую публичную страницу новинок RemixPool и извлекает Artist, Title, BPM, Camelot Key и Genre; авторизация, прослушивание и скачивание аудио не используются.',
          'Провайдер RemixPool консервативно фильтрует результаты по Artist/Title и возвращает 0 результатов, если нужного трека нет среди публично видимых новинок. Stage 17.1 fallback и общий scoring/Merge применяются автоматически.',
          'Hotfix 18.2.1 исправляет поиск MUZVIZOR: CCML использует подтверждённый публичный маршрут /tracks?query= и формирует запрос как Artist - Title. Неподтверждённые варианты /tracks?search, /search?query, /search?q и /tracks?q больше не используются.',
          'Hotfix 18.2.2 учитывает реальную DOM-структуру MUZVIZOR: track__row_main и отдельные колонки title/BPM/key/genre разбираются структурно, а track__stage_* переносится как стадия вечеринки. Если /tracks?query= отдаёт только JS-оболочку без строк, CCML ищет совпадение на публичных server-rendered страницах жанров и кэширует их на 15 минут.',
          'Hotfix 18.2.3 формирует MUZVIZOR query точно как браузерный URL: пробелы кодируются как %20, а не +. Fallback ограничен шестью приоритетными публичными страницами, единичная временная ошибка страницы не останавливает поиск, а версия metadata-cache повышена для принудительного свежего запроса после обновления.',
          'Hotfix 18.2.4 исправляет ложное «Совпадений нет» MUZVIZOR: наличие track__row_main больше не отключает резервный text-parser, если DOM-разбор не получил совпадение. Внутри строки сначала используются track__title/track__artist, затем последние значимые значения title-колонки и фактическая последовательность Title → Artist → TOP 100 → BPM → Key → Genre. Metadata-cache снова инвалидируется.',
          'Stage 16.5 добавляет очередь решений по группам: «Оставить лучший», «Оставить всё», «Карантин выбранных» и «Пропустить». Решения сохраняются локально и автоматически отбрасываются, если состав группы изменился.',
          'Фильтр «Не обработано» позволяет последовательно пройти библиотеку. Кнопка «Проверить очередь» показывает итог: сколько групп обработано, сколько файлов уйдёт в карантин и сколько останется.',
          'Пакетное применение очереди использует только карантин и один выбор папки для всех запланированных файлов. Безвозвратное удаление намеренно остаётся ручным действием внутри одной группы.',
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
          'Для одного трека поля по-прежнему редактируются во вкладке «Теги» Inspector. При выборе 2+ треков рядом с «Пресеты / Колонки» появляются отдельные кнопки «Массовые теги» и «Преобразования».',
          '«Массовые теги» и «Преобразования» открываются из компактной строки рядом с «Пресеты / Колонки»; исторические стили toolbar нормализованы, поэтому блок больше не растягивает рабочую область.',
          'Предпросмотр массовых тегов и преобразований открывается отдельным модальным окном. Для каждого файла показаны изменяемое поле, значение «До» и значение «После», а сверху — количество файлов и изменений.',
          'Применение выполняется прямо из окна предпросмотра и создаёт существующий единый Undo change-set. Popup инструментов автозакрываются после ухода курсора.',
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
          'Stage 17.2 virtualizes large tables: above 240 rows, only visible rows plus a small overscan window are mounted in the DOM. The full result set, sorting, filters, selection and keyboard navigation still operate on the complete library.',
          'Selection membership also uses a Set instead of repeated linear searches. This particularly reduces stalls with Ctrl+A, large selections and libraries containing thousands of files.',
          'Duplicates groups the library at three confidence levels: identical ISRC, exact Artist/Title with close duration, and possible version matches where Remix/Edit/Intro/Clean and similar version qualifiers are temporarily removed from Title. Possible versions are not treated as confirmed duplicates.',
          'Inside a group you can compare file, codec, bitrate, sample rate, size, duration and ISRC. Stage 16.1 does not delete or move anything; this screen is diagnostic only.',
          'Stage 16.2 adds a 0–100 heuristic quality score: up to 80 points for audio properties (lossless/lossy, bitrate, sample rate, channels) and up to 20 for metadata completeness, valid ISRC and embedded artwork. A uniquely highest score is marked Best quality and sorted to the top of its group.',
          'When scores are exactly tied, CCML does not invent a winner. In a Possible duplicate group, the quality leader only identifies the technically preferred file; it does not prove Remix/Edit/Intro versions are interchangeable.',
          'Stage 16.3 adds manual file selection inside each group. Keep best only selects every other row; it never starts a destructive action automatically. The backend refuses any single operation that would remove every file from an affected group.',
          'Quarantine asks for a folder outside managed library roots, moves files into its CCML Duplicates subfolder, then removes their index rows. If SQLite fails, completed moves are rolled back.',
          'Delete is permanent and requires typing DELETE. Files are first renamed to temporary siblings, then index rows are removed in one SQLite transaction; if that transaction fails, original filenames are restored.',
          'Stage 16.4 adds Check audio. FFmpeg decodes every file in the current group to 4 kHz mono PCM and compares normalized energy, roughness and zero-crossing features with alignment up to ±6 seconds. Results are Same, Similar, Different or Error.',
          'Same requires very high decoded-waveform similarity plus close full duration. The check remains heuristic: it supports the quarantine/delete decision, never starts an action itself, and does not replace listening to questionable Remix/Edit/Intro versions.',
          'Stage 16.4.2 changes how large result sets are displayed: instead of hundreds of expandable cards, a scrollable group list appears on the left and only the selected group details appear on the right. This stays readable with hundreds of groups and gives the comparison/actions table more room.',
          'Stage 17.1 improves metadata lookup for version tags. If the original Title containing labels such as (Extended Mix), (Dirty), (Clean), (Intro), (Radio Edit), (Original Mix), Remix/Rework/Bootleg, Acapella and similar qualifiers produces no usable result, CCML automatically retries with the base title.',
          'The fallback only removes recognized trailing version qualifiers from the search query. When metadata is applied, the local version suffix is restored, so searching for “Track” does not turn a local “Track (Extended Mix)” into plain “Track”. This also works for background batch enrichment.',
          'Stage 17.3 fixes Spotify artwork: album.images are no longer blocked by ArtworkEmbeddable=false. CCML selects the largest valid Spotify HTTP/HTTPS image and can download/embed it through the shared Tag Editor artwork path when artwork inclusion is enabled.',
          'Stage 18.0 establishes the DJ Pool Sources foundation. MetadataCandidate now carries BPM, Key and KeyScale, while providers can identify themselves as DJ Pool sources without changing the existing provider interface.',
          'BPM/Key can be selected in Metadata Merge and written with the rest of provider metadata. The change participates in normal Tag History/Undo, updates SQLite and can be read back on a later file scan.',
          'Stage 18.0 does not connect external DJ-pool sites yet. Following stages add providers one at a time, starting with MUZVIZOR.',
          'Stage 18.0.1 unifies Enrich metadata with Find metadata: enrichment no longer uses separate Auto/Fast/Full branches or early stopping. Every track goes through the same lookupMetadataQuery, so providers, cache, Title fallback, scoring and suggested-candidate selection are identical.',
          'Minimum confidence, Fill missing fields only and artwork inclusion remain automatic-apply settings. They no longer change the search algorithm itself.',
          'Stage 18.1 adds MUZVIZOR as an optional DJ Pool source. The provider consumes public pages only and supplies Artist, Title, BPM, Camelot Key and Genre; it does not authenticate, access subscriptions or download audio files.',
          'MUZVIZOR is enabled separately in Settings. Results use normal CCML scoring and the Stage 17.1 cleaned-Title fallback, so the local version suffix is still preserved when metadata is applied.',
          'MUZVIZOR public search is not a documented developer API. CCML uses a conservative HTML parser and rejects Artist/Title mismatches; if the site changes, the provider may temporarily return zero results rather than attach unrelated metadata.',
          'Hotfix 18.1.1 fixes an intermittent Enrich metadata artwork loss: when the CCML Merge artwork URL cannot be downloaded, CCML tries other trusted embeddable artwork URLs from the same lookup before finally continuing without artwork.',
          'Stage 18.2 adds RemixPool as an optional DJ Pool source. CCML reads only the confirmed public RemixPool new-releases page and extracts Artist, Title, BPM, Camelot Key and Genre; it does not authenticate, play, or download audio.',
          'The RemixPool provider filters conservatively by Artist/Title and returns zero results when the requested track is not present in the publicly visible new releases. Stage 17.1 fallback and the shared scoring/Merge pipeline apply automatically.',
          'Hotfix 18.2.1 fixes MUZVIZOR search: CCML uses the confirmed public /tracks?query= route and formats searches as Artist - Title. The unconfirmed /tracks?search, /search?query, /search?q and /tracks?q variants are no longer used.',
          'Hotfix 18.2.2 follows MUZVIZOR’s real DOM structure: track__row_main and the title/BPM/key/genre columns are parsed structurally, while track__stage_* is exposed as the DJ stage. If /tracks?query= returns only a JavaScript shell without rows, CCML falls back to public server-rendered genre pages and caches them for 15 minutes.',
          'Hotfix 18.2.3 formats the MUZVIZOR query exactly like the browser URL: spaces are encoded as %20 rather than +. Fallback is capped at six priority public pages, one temporary page failure no longer stops the lookup, and the metadata-cache version is bumped to force a fresh request after the update.',
          'Hotfix 18.2.4 fixes false MUZVIZOR no-match results: the presence of track__row_main no longer disables the text fallback when DOM parsing yields no match. A row now prefers track__title/track__artist, then the final meaningful title-column values and the observed Title → Artist → TOP 100 → BPM → Key → Genre sequence. The metadata cache is invalidated again.',
          'Stage 16.5 adds a per-group decision queue: Keep best, Keep all, Quarantine selected and Skip. Decisions are persisted locally and automatically discarded if group membership changes.',
          'The Unresolved filter supports a sequential review workflow. Review queue summarizes processed groups, files planned for quarantine and files that will remain.',
          'Batch queue apply uses quarantine only and asks for the destination folder once for all planned files. Permanent deletion intentionally remains a manual single-group action.',
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
          'Single-track fields remain in Inspector → Tags. When 2+ tracks are selected, separate Batch tags and Transforms buttons appear next to Presets / Columns.',
          'Batch tags and Transforms open from a compact row next to Presets / Columns; accumulated historical toolbar styles are normalized so the controls no longer stretch the workspace.',
          'Batch-tag and transform Preview opens in a separate modal. For every file it shows the changed field plus Before and After values, with file/change totals at the top.',
          'Apply is performed directly from the preview modal and uses the existing single Undo change set. Tool popups still auto-close after the pointer leaves.',
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
