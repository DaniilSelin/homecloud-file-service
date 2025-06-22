-- Создание таблицы ревизий файлов
CREATE TABLE file_revisions (
    id            UUID      PRIMARY KEY DEFAULT gen_random_uuid(),
    file_id       UUID      NOT NULL REFERENCES files(id) ON DELETE CASCADE,
    revision_id   BIGINT    NOT NULL,  -- в том числе для Google-стиля
    md5_checksum  TEXT,
    size          BIGINT,
    created_at    TIMESTAMP NOT NULL DEFAULT now(),
    storage_path  TEXT      NOT NULL,  -- путь к конкретной ревизии
    mime_type     TEXT,
    user_id       UUID      -- кто создал ревизию
);

-- Индексы для оптимизации запросов
CREATE INDEX idx_file_revisions_file_id ON file_revisions(file_id);
CREATE INDEX idx_file_revisions_revision_id ON file_revisions(revision_id);
CREATE INDEX idx_file_revisions_created_at ON file_revisions(created_at);
CREATE INDEX idx_file_revisions_user_id ON file_revisions(user_id);

-- Уникальный индекс для предотвращения дублирования ревизий
CREATE UNIQUE INDEX idx_file_revisions_unique ON file_revisions(file_id, revision_id); 