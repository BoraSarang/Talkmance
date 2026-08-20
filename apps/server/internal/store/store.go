// Package store — PostgreSQL 액세스 (CRUD 쿼리)
package store

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/talkmance/server/internal/errs"
)

// Store DB 액세스
type Store struct {
	pool *pgxpool.Pool
}

// New store 생성
func New(pool *pgxpool.Pool) *Store {
	return &Store{pool: pool}
}

// CustomModel 커스텀 모델 (user_models)
type CustomModel struct {
	ID          uuid.UUID `json:"id"`
	UserID      uuid.UUID `json:"user_id"`
	Name        string    `json:"name"`
	ModelID     string    `json:"model_id"`
	BaseURL     string    `json:"base_url"`
	APIKeyRef   string    `json:"api_key_ref,omitempty"`
	IsFree      bool      `json:"is_free"`
	Description string    `json:"description"`
	Enabled     bool      `json:"enabled"`
}

// ListCustomModels 사용자 커스텀 모델 목록
func (s *Store) ListCustomModels(ctx context.Context, userID uuid.UUID) ([]CustomModel, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT id, user_id, name, model_id, base_url, api_key_ref, is_free, description, enabled
		 FROM user_models WHERE user_id = $1 ORDER BY created_at`, userID)
	if err != nil {
		return nil, errs.Wrap(errs.ESRVDb1001, err)
	}
	defer rows.Close()

	var out = make([]CustomModel, 0)
	for rows.Next() {
		var m CustomModel
		if err := rows.Scan(&m.ID, &m.UserID, &m.Name, &m.ModelID, &m.BaseURL, &m.APIKeyRef, &m.IsFree, &m.Description, &m.Enabled); err != nil {
			return nil, errs.Wrap(errs.ESRVDb1001, err)
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

// CreateCustomModel 커스텀 모델 추가
func (s *Store) CreateCustomModel(ctx context.Context, userID uuid.UUID, m CustomModel) (uuid.UUID, error) {
	var id uuid.UUID
	err := s.pool.QueryRow(ctx,
		`INSERT INTO user_models (user_id, name, model_id, base_url, api_key_ref, is_free, description, enabled)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8) RETURNING id`,
		userID, m.Name, m.ModelID, m.BaseURL, m.APIKeyRef, m.IsFree, m.Description, m.Enabled).Scan(&id)
	if err != nil {
		return uuid.Nil, errs.Wrap(errs.ESRVDb1001, err)
	}
	return id, nil
}

// GetCustomModel 단건 조회 (소유자 검증 포함)
func (s *Store) GetCustomModel(ctx context.Context, id, userID uuid.UUID) (*CustomModel, error) {
	var m CustomModel
	err := s.pool.QueryRow(ctx,
		`SELECT id, user_id, name, model_id, base_url, api_key_ref, is_free, description, enabled
		 FROM user_models WHERE id = $1 AND user_id = $2`, id, userID).
		Scan(&m.ID, &m.UserID, &m.Name, &m.ModelID, &m.BaseURL, &m.APIKeyRef, &m.IsFree, &m.Description, &m.Enabled)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, errs.New(errs.EComModel1002, nil)
	}
	if err != nil {
		return nil, errs.Wrap(errs.ESRVDb1001, err)
	}
	return &m, nil
}

// UpdateCustomModel 수정 (변경 필드만)
func (s *Store) UpdateCustomModel(ctx context.Context, id, userID uuid.UUID, m CustomModel) error {
	tag, err := s.pool.Exec(ctx,
		`UPDATE user_models SET
		   name = COALESCE($3, name),
		   model_id = COALESCE($4, model_id),
		   base_url = COALESCE($5, base_url),
		   is_free = $6,
		   description = COALESCE($7, description),
		   enabled = $8
		 WHERE id = $1 AND user_id = $2`,
		id, userID, nullStr(m.Name), nullStr(m.ModelID), nullStr(m.BaseURL), m.IsFree, nullStr(m.Description), m.Enabled)
	if err != nil {
		return errs.Wrap(errs.ESRVDb1001, err)
	}
	if tag.RowsAffected() == 0 {
		return errs.New(errs.EComModel1002, nil)
	}
	return nil
}

// DeleteCustomModel 삭제 (소유자 검증 포함)
func (s *Store) DeleteCustomModel(ctx context.Context, id, userID uuid.UUID) error {
	tag, err := s.pool.Exec(ctx, `DELETE FROM user_models WHERE id = $1 AND user_id = $2`, id, userID)
	if err != nil {
		return errs.Wrap(errs.ESRVDb1001, err)
	}
	if tag.RowsAffected() == 0 {
		return errs.New(errs.EComModel1002, nil)
	}
	return nil
}

// nullStr 빈 문자열 → NULL (COALESCE용)
func nullStr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// UserAPIKey 사용자 BYOK 키 (암호화 저장)
type UserAPIKey struct {
	ID    uuid.UUID `json:"id"`
	Label string    `json:"label"`
}

// ListUserKeys 사용자 BYOK 키 목록 (암호문 미노출)
func (s *Store) ListUserKeys(ctx context.Context, userID uuid.UUID) ([]UserAPIKey, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT id, label FROM user_api_keys WHERE user_id = $1 ORDER BY created_at`, userID)
	if err != nil {
		return nil, errs.Wrap(errs.ESRVDb1001, err)
	}
	defer rows.Close()

	keys := make([]UserAPIKey, 0)
	for rows.Next() {
		var k UserAPIKey
		if err := rows.Scan(&k.ID, &k.Label); err != nil {
			return nil, errs.Wrap(errs.ESRVDb1001, err)
		}
		keys = append(keys, k)
	}
	return keys, rows.Err()
}

// CreateUserKey BYOK 키 저장 (암호문)
func (s *Store) CreateUserKey(ctx context.Context, userID uuid.UUID, label, encrypted string) (uuid.UUID, error) {
	var id uuid.UUID
	err := s.pool.QueryRow(ctx,
		`INSERT INTO user_api_keys (user_id, label, key_encrypted) VALUES ($1,$2,$3) RETURNING id`,
		userID, label, encrypted).Scan(&id)
	if err != nil {
		return uuid.Nil, errs.Wrap(errs.ESRVDb1001, err)
	}
	return id, nil
}

// GetUserKey 암호문 조회 (BYOK 모델 호출 시 복호화용)
func (s *Store) GetUserKey(ctx context.Context, id, userID uuid.UUID) (string, error) {
	var encrypted string
	err := s.pool.QueryRow(ctx,
		`SELECT key_encrypted FROM user_api_keys WHERE id = $1 AND user_id = $2`, id, userID).Scan(&encrypted)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", errs.New(errs.EComModel1002, nil)
	}
	if err != nil {
		return "", errs.Wrap(errs.ESRVDb1001, err)
	}
	return encrypted, nil
}

// DeleteUserKey BYOK 키 삭제
func (s *Store) DeleteUserKey(ctx context.Context, id, userID uuid.UUID) error {
	tag, err := s.pool.Exec(ctx, `DELETE FROM user_api_keys WHERE id = $1 AND user_id = $2`, id, userID)
	if err != nil {
		return errs.Wrap(errs.ESRVDb1001, err)
	}
	if tag.RowsAffected() == 0 {
		return errs.New(errs.EComModel1002, nil)
	}
	return nil
}

// GetFreeUsage 특정 날짜(UTC yyyy-mm-dd) 무료 사용량 조회 (없으면 0)
func (s *Store) GetFreeUsage(ctx context.Context, date string) (int, error) {
	var n int
	err := s.pool.QueryRow(ctx,
		`SELECT free_count FROM daily_usage WHERE date = $1`, date).Scan(&n)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, nil
	}
	if err != nil {
		return 0, errs.Wrap(errs.ESRVDb1001, err)
	}
	return n, nil
}

// IncFreeUsage 당일 무료 사용량 +1 (upsert)
func (s *Store) IncFreeUsage(ctx context.Context, date string) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO daily_usage (date, free_count) VALUES ($1, 1)
		ON CONFLICT (date) DO UPDATE SET free_count = daily_usage.free_count + 1`, date)
	if err != nil {
		return errs.Wrap(errs.ESRVDb1001, err)
	}
	return nil
}
