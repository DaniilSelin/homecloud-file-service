1. Основная таблица

CREATE TABLE files (
    -- Уникальный идентификатор файла
    id               UUID        PRIMARY KEY DEFAULT gen_random_uuid(),

    -- Владелец (пользователь)
    owner_id         UUID        NOT NULL REFERENCES users(id),

    -- Папка/родитель (может быть NULL для корня)
    parent_id        UUID        REFERENCES files(id),

    -- Имя и расширение
    name             TEXT        NOT NULL,
    file_extension   TEXT,

    -- MIME-тип
    mime_type        TEXT        NOT NULL,

    -- Путь к физическому файлу на хост-машине
    storage_path     TEXT        NOT NULL,

    -- Размер в байтах
    size             BIGINT      NOT NULL,

    -- Контрольные суммы для проверки целостности
    md5_checksum     TEXT,
    sha256_checksum  TEXT,

    -- Флаги и состояния
    is_folder        BOOLEAN     NOT NULL DEFAULT FALSE,
    is_trashed       BOOLEAN     NOT NULL DEFAULT FALSE,
    trashed_at       TIMESTAMP,
    starred          BOOLEAN     NOT NULL DEFAULT FALSE,

    -- Системные метаданные
    created_at       TIMESTAMP   NOT NULL DEFAULT now(),
    updated_at       TIMESTAMP   NOT NULL DEFAULT now(),
    last_viewed_at   TIMESTAMP,
    viewed_by_me     BOOLEAN     NOT NULL DEFAULT FALSE,

    -- Счётчики и версии
    version          BIGINT      NOT NULL DEFAULT 1,
    revision_id      UUID,  -- Ссылка на последнюю ревизию

    -- Google-аналоги (contentHints, exportLinks и т.п.)
    indexable_text   TEXT,  -- contentHints.indexableText :contentReference[oaicite:0]{index=0}
    thumbnail_link   TEXT,  -- contentHints.thumbnail
    web_view_link    TEXT,  -- webViewLink
    web_content_link TEXT,  -- webContentLink
    icon_link        TEXT   -- iconLink
);

2. Таблица ревизий
CREATE TABLE file_revisions (
    id            UUID      PRIMARY KEY DEFAULT gen_random_uuid(),
    file_id       UUID      NOT NULL REFERENCES files(id) ON DELETE CASCADE,
    revision_id   BIGINT    NOT NULL,  -- в том числе для Google-стиля
    md5_checksum  TEXT,
    size          BIGINT,
    created_at    TIMESTAMP NOT NULL DEFAULT now(),
    storage_path  TEXT      NOT NULL,  -- путь к конкретной ревизии
    mime_type     TEXT,
    user_id       UUID      REFERENCES users(id)  -- кто создал ревизию
);

3. Таблица прав доступа

CREATE TABLE file_permissions (
    id           UUID      PRIMARY KEY DEFAULT gen_random_uuid(),
    file_id      UUID      NOT NULL REFERENCES files(id) ON DELETE CASCADE,
    grantee_id   UUID,     -- user|group|domain или NULL для «всех»
    grantee_type TEXT      NOT NULL,  -- USER, GROUP, DOMAIN, ANYONE
    role         TEXT      NOT NULL,  -- OWNER, ORGANIZER, FILE_OWNER, WRITER, COMMENTER, READER
    allow_share  BOOLEAN   NOT NULL DEFAULT FALSE,
    created_at   TIMESTAMP NOT NULL DEFAULT now()
);
