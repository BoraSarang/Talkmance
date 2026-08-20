-- v003: 일일 사용량 카운터 (무료 티어 50회/일, UTC 날짜 기준)
CREATE TABLE IF NOT EXISTS daily_usage (
    date       TEXT PRIMARY KEY,      -- UTC yyyy-mm-dd
    free_count INT NOT NULL DEFAULT 0 -- OpenRouter 무료 모델 호출 시도 횟수
);