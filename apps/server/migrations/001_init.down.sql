-- v001 롤백: 초기 스키마 제거 (역순)
DROP TABLE IF EXISTS user_models;
DROP TABLE IF EXISTS messages;
DROP TABLE IF EXISTS chat_sessions;
DROP TABLE IF EXISTS prompt_rules;
DROP TABLE IF EXISTS character_memories;
DROP TABLE IF EXISTS characters;
DROP TABLE IF EXISTS users;
DROP EXTENSION IF EXISTS vector;