-- v001: 초기 스키마 (users/characters/memories/sessions/messages/rules/models)
-- 요구: PostgreSQL 16 + pgvector 확장 (Neon 기본 지원)

CREATE EXTENSION IF NOT EXISTS vector;

-- 익명 기기 사용자
CREATE TABLE users (
    id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    device_id      TEXT NOT NULL UNIQUE,
    birth_year     INTEGER,
    adult_verified BOOLEAN NOT NULL DEFAULT FALSE,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- 캐릭터 (페르소나 JSON 포함)
CREATE TABLE characters (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id    UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name       TEXT NOT NULL,
    title      TEXT NOT NULL DEFAULT '',
    avatar_url TEXT,
    category   TEXT NOT NULL DEFAULT '기타',
    persona    JSONB NOT NULL DEFAULT '{}'::jsonb,
    greeting   TEXT NOT NULL DEFAULT '',
    age        INTEGER CHECK (age IS NULL OR age >= 19),
    adult      BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_characters_user ON characters(user_id);

-- 장기 기억 (3계층: short/medium/long + 벡터)
CREATE TABLE character_memories (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    character_id UUID NOT NULL REFERENCES characters(id) ON DELETE CASCADE,
    mem_type     TEXT NOT NULL DEFAULT 'long' CHECK (mem_type IN ('short', 'medium', 'long')),
    content      TEXT NOT NULL,
    embedding    VECTOR(1536),
    importance   REAL NOT NULL DEFAULT 0.5,
    pinned       BOOLEAN NOT NULL DEFAULT FALSE,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_memories_character ON character_memories(character_id);
CREATE INDEX idx_memories_embedding ON character_memories
    USING hnsw (embedding vector_cosine_ops);

-- 대화 규칙 (프롬프트 템플릿) — chat_sessions가 참조하므로 먼저 생성
CREATE TABLE prompt_rules (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id       UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name          TEXT NOT NULL,
    system_prompt TEXT NOT NULL DEFAULT '',
    json_schema   JSONB,
    is_default    BOOLEAN NOT NULL DEFAULT FALSE,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_rules_user ON prompt_rules(user_id);

-- 대화방 (중기 요약 summary 포함)
CREATE TABLE chat_sessions (
    id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    character_id   UUID NOT NULL REFERENCES characters(id) ON DELETE CASCADE,
    user_id        UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    model_id       TEXT NOT NULL DEFAULT '',
    rule_id        UUID REFERENCES prompt_rules(id),
    status         TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'archived')),
    summary        TEXT,
    relation_state JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_sessions_user ON chat_sessions(user_id);
CREATE INDEX idx_sessions_character ON chat_sessions(character_id);

-- 대화 메시지 (토큰/비용 기록)
CREATE TABLE messages (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    session_id UUID NOT NULL REFERENCES chat_sessions(id) ON DELETE CASCADE,
    role       TEXT NOT NULL CHECK (role IN ('user', 'assistant', 'system')),
    content    TEXT NOT NULL,
    model      TEXT NOT NULL DEFAULT '',
    token_in   INTEGER NOT NULL DEFAULT 0,
    token_out  INTEGER NOT NULL DEFAULT 0,
    cost       NUMERIC(12, 6) NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_messages_session ON messages(session_id, created_at);

-- 커스텀 모델 (OpenRouter 외 Base URL 지원)
CREATE TABLE user_models (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id     UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name        TEXT NOT NULL,
    model_id    TEXT NOT NULL,
    base_url    TEXT NOT NULL DEFAULT '',
    api_key_ref TEXT,
    is_free     BOOLEAN NOT NULL DEFAULT FALSE,
    description TEXT NOT NULL DEFAULT '',
    enabled     BOOLEAN NOT NULL DEFAULT TRUE,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_models_user ON user_models(user_id);