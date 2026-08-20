package store

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/talkmance/server/internal/errs"
)

// PromptRule 대화 규칙 (프롬프트 템플릿)
type PromptRule struct {
	ID           uuid.UUID      `json:"id"`
	UserID       uuid.UUID      `json:"user_id"`
	Name         string         `json:"name"`
	SystemPrompt string         `json:"system_prompt"`
	JSONSchema   map[string]any `json:"json_schema"`
	IsDefault    bool           `json:"is_default"`
	CreatedAt    time.Time      `json:"created_at"`
}

// ListRules 사용자 규칙 목록
func (s *Store) ListRules(ctx context.Context, userID uuid.UUID) ([]PromptRule, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT id, user_id, name, system_prompt, COALESCE(json_schema,'{}'::jsonb), is_default, created_at
		 FROM prompt_rules WHERE user_id = $1 ORDER BY is_default DESC, created_at`, userID)
	if err != nil {
		return nil, errs.Wrap(errs.ESRVDb1001, err)
	}
	defer rows.Close()

	out := make([]PromptRule, 0)
	for rows.Next() {
		var p PromptRule
		if err := rows.Scan(&p.ID, &p.UserID, &p.Name, &p.SystemPrompt, &p.JSONSchema, &p.IsDefault, &p.CreatedAt); err != nil {
			return nil, errs.Wrap(errs.ESRVDb1001, err)
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// CreateRule 규칙 생성 (첫 규칙이면 기본값)
func (s *Store) CreateRule(ctx context.Context, userID uuid.UUID, p PromptRule) (uuid.UUID, bool, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return uuid.Nil, false, errs.Wrap(errs.ESRVDb1001, err)
	}
	defer tx.Rollback(ctx)

	var isDefault bool
	err = tx.QueryRow(ctx,
		`SELECT NOT EXISTS(SELECT 1 FROM prompt_rules WHERE user_id = $1)`, userID).Scan(&isDefault)
	if err != nil {
		return uuid.Nil, false, errs.Wrap(errs.ESRVDb1001, err)
	}

	var id uuid.UUID
	err = tx.QueryRow(ctx,
		`INSERT INTO prompt_rules (user_id, name, system_prompt, json_schema, is_default)
		 VALUES ($1,$2,$3,$4,$5) RETURNING id`,
		userID, p.Name, p.SystemPrompt, p.JSONSchema, isDefault).Scan(&id)
	if err != nil {
		return uuid.Nil, false, errs.Wrap(errs.ESRVDb1001, err)
	}
	if err := tx.Commit(ctx); err != nil {
		return uuid.Nil, false, errs.Wrap(errs.ESRVDb1001, err)
	}
	return id, isDefault, nil
}

// GetRule 단건 (소유자 검증)
func (s *Store) GetRule(ctx context.Context, id, userID uuid.UUID) (*PromptRule, error) {
	var p PromptRule
	err := s.pool.QueryRow(ctx,
		`SELECT id, user_id, name, system_prompt, COALESCE(json_schema,'{}'::jsonb), is_default, created_at
		 FROM prompt_rules WHERE id = $1 AND user_id = $2`, id, userID).
		Scan(&p.ID, &p.UserID, &p.Name, &p.SystemPrompt, &p.JSONSchema, &p.IsDefault, &p.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, errs.New(errs.EComRule1001, nil)
	}
	if err != nil {
		return nil, errs.Wrap(errs.ESRVDb1001, err)
	}
	return &p, nil
}

// UpdateRule 규칙 수정
func (s *Store) UpdateRule(ctx context.Context, id, userID uuid.UUID, p PromptRule) error {
	tag, err := s.pool.Exec(ctx,
		`UPDATE prompt_rules SET
		   name = COALESCE($3, name),
		   system_prompt = COALESCE($4, system_prompt),
		   json_schema = COALESCE($5, json_schema)
		 WHERE id = $1 AND user_id = $2`,
		id, userID, nullStr(p.Name), nullStr(p.SystemPrompt), p.JSONSchema)
	if err != nil {
		return errs.Wrap(errs.ESRVDb1001, err)
	}
	if tag.RowsAffected() == 0 {
		return errs.New(errs.EComRule1001, nil)
	}
	return nil
}

// SetDefaultRule 기본 규칙 변경 (같은 사용자의 다른 규칙은 기본 해제)
func (s *Store) SetDefaultRule(ctx context.Context, id, userID uuid.UUID) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return errs.Wrap(errs.ESRVDb1001, err)
	}
	defer tx.Rollback(ctx)

	tag, err := tx.Exec(ctx,
		`UPDATE prompt_rules SET is_default = (id = $1) WHERE user_id = $2`, id, userID)
	if err != nil {
		return errs.Wrap(errs.ESRVDb1001, err)
	}
	if tag.RowsAffected() == 0 {
		return errs.New(errs.EComRule1001, nil)
	}
	return tx.Commit(ctx)
}

// DeleteRule 규칙 삭제 (기본 규칙은 삭제 불가 — 기본 유지 필요)
func (s *Store) DeleteRule(ctx context.Context, id, userID uuid.UUID) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return errs.Wrap(errs.ESRVDb1001, err)
	}
	defer tx.Rollback(ctx)

	var isDefault bool
	err = tx.QueryRow(ctx,
		`SELECT is_default FROM prompt_rules WHERE id = $1 AND user_id = $2`, id, userID).Scan(&isDefault)
	if errors.Is(err, pgx.ErrNoRows) {
		return errs.New(errs.EComRule1001, nil)
	}
	if err != nil {
		return errs.Wrap(errs.ESRVDb1001, err)
	}
	if isDefault {
		return errs.New(errs.EComRule2002, nil)
	}

	if _, err := tx.Exec(ctx, `DELETE FROM prompt_rules WHERE id = $1 AND user_id = $2`, id, userID); err != nil {
		return errs.Wrap(errs.ESRVDb1001, err)
	}
	return tx.Commit(ctx)
}
