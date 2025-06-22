-- Создание основной таблицы files
CREATE TABLE files (
    -- Уникальный идентификатор файла
    id               UUID        PRIMARY KEY DEFAULT gen_random_uuid(),

    -- Владелец (пользователь)
    owner_id         UUID        NOT NULL,

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
    indexable_text   TEXT,  -- contentHints.indexableText
    thumbnail_link   TEXT,  -- contentHints.thumbnail
    web_view_link    TEXT,  -- webViewLink
    web_content_link TEXT,  -- webContentLink
    icon_link        TEXT   -- iconLink
);

-- Индексы для оптимизации запросов
CREATE INDEX idx_files_owner_id ON files(owner_id);
CREATE INDEX idx_files_parent_id ON files(parent_id);
CREATE INDEX idx_files_is_trashed ON files(is_trashed);
CREATE INDEX idx_files_starred ON files(starred);
CREATE INDEX idx_files_created_at ON files(created_at);
CREATE INDEX idx_files_updated_at ON files(updated_at);
CREATE INDEX idx_files_name ON files(name);
CREATE INDEX idx_files_mime_type ON files(mime_type);

-- Уникальный индекс для предотвращения дублирования файлов в одной папке
CREATE UNIQUE INDEX idx_files_unique_name_parent ON files(name, parent_id) WHERE parent_id IS NOT NULL;
CREATE UNIQUE INDEX idx_files_unique_name_root ON files(name) WHERE parent_id IS NULL; 