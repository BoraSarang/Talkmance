package store

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/talkmance/server/internal/errs"
)

// Character 캐릭터 (persona JSON 포함)
type Character struct {
	ID        uuid.UUID      `json:"id"`
	UserID    uuid.UUID      `json:"user_id"`
	Name      string         `json:"name"`
	Title     string         `json:"title"`
	AvatarURL string         `json:"avatar_url,omitempty"`
	Category  string         `json:"category"`
	Persona   map[string]any `json:"persona"`
	Greeting  string         `json:"greeting"`
	Age       *int           `json:"age"`
	Adult     bool           `json:"adult"`
	CreatedAt time.Time      `json:"created_at"`
}

// ChatSession 대화방
type ChatSession struct {
	ID            uuid.UUID      `json:"id"`
	CharacterID   uuid.UUID      `json:"character_id"`
	UserID        uuid.UUID      `json:"user_id"`
	ModelID       string         `json:"model_id"`
	RuleID        *uuid.UUID     `json:"rule_id"`
	Status        string         `json:"status"`
	Summary       string         `json:"summary,omitempty"`
	RelationState map[string]any `json:"relation_state"`
	CreatedAt     time.Time      `json:"created_at"`
	UpdatedAt     time.Time      `json:"updated_at"`
	LastMessage   *string        `json:"last_message,omitempty"`
	LastMessageAt *time.Time     `json:"last_message_at,omitempty"`
}

// Message 대화 메시지
type Message struct {
	ID        uuid.UUID `json:"id"`
	SessionID uuid.UUID `json:"session_id"`
	Role      string    `json:"role"`
	Content   string    `json:"content"`
	Model     string    `json:"model,omitempty"`
	TokenIn   int       `json:"token_in"`
	TokenOut  int       `json:"token_out"`
	Cost      float64   `json:"cost"`
	CreatedAt time.Time `json:"created_at"`
}

// ---- 캐릭터 ----

// ListCharacters 사용자 캐릭터 목록
func (s *Store) ListCharacters(ctx context.Context, userID uuid.UUID) ([]Character, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT id, user_id, name, title, COALESCE(avatar_url,''), category, persona, greeting, age, adult, created_at
		 FROM characters WHERE user_id = $1 ORDER BY created_at DESC`, userID)
	if err != nil {
		return nil, errs.Wrap(errs.ESRVDb1001, err)
	}
	defer rows.Close()

	out := make([]Character, 0)
	for rows.Next() {
		c, err := scanCharacter(rows)
		if err != nil {
			return nil, errs.Wrap(errs.ESRVDb1001, err)
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// GetCharacter 단건 (소유자 검증)
func (s *Store) GetCharacter(ctx context.Context, id, userID uuid.UUID) (*Character, error) {
	row := s.pool.QueryRow(ctx,
		`SELECT id, user_id, name, title, COALESCE(avatar_url,''), category, persona, greeting, age, adult, created_at
		 FROM characters WHERE id = $1 AND user_id = $2`, id, userID)
	c, err := scanCharacter(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, errs.New(errs.EComChar1002, nil)
	}
	if err != nil {
		return nil, errs.Wrap(errs.ESRVDb1001, err)
	}
	return &c, nil
}

// CreateCharacter 캐릭터 생성
func (s *Store) CreateCharacter(ctx context.Context, userID uuid.UUID, c Character) (uuid.UUID, error) {
	if c.Persona == nil {
		c.Persona = map[string]any{}
	}
	var id uuid.UUID
	err := s.pool.QueryRow(ctx,
		`INSERT INTO characters (user_id, name, title, avatar_url, category, persona, greeting, age, adult)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9) RETURNING id`,
		userID, c.Name, c.Title, nullStr(c.AvatarURL), c.Category, c.Persona, c.Greeting, c.Age, c.Adult).Scan(&id)
	if err != nil {
		return uuid.Nil, errs.Wrap(errs.ESRVDb1001, err)
	}
	return id, nil
}

// UpdateCharacter 캐릭터 수정
func (s *Store) UpdateCharacter(ctx context.Context, id, userID uuid.UUID, c Character) error {
	tag, err := s.pool.Exec(ctx,
		`UPDATE characters SET
		   name = COALESCE($3, name),
		   title = COALESCE($4, title),
		   avatar_url = COALESCE($5, avatar_url),
		   category = COALESCE($6, category),
		   persona = COALESCE($7, persona),
		   greeting = COALESCE($8, greeting),
		   age = $9, adult = $10
		 WHERE id = $1 AND user_id = $2`,
		id, userID, nullStr(c.Name), nullStr(c.Title), nullStr(c.AvatarURL), nullStr(c.Category),
		c.Persona, nullStr(c.Greeting), c.Age, c.Adult)
	if err != nil {
		return errs.Wrap(errs.ESRVDb1001, err)
	}
	if tag.RowsAffected() == 0 {
		return errs.New(errs.EComChar1002, nil)
	}
	return nil
}

// UpdateAvatarURL 아바타 URL 갱신 (재생성용)
func (s *Store) UpdateAvatarURL(ctx context.Context, id, userID uuid.UUID, avatarURL string) error {
	tag, err := s.pool.Exec(ctx,
		`UPDATE characters SET avatar_url = $3 WHERE id = $1 AND user_id = $2`,
		id, userID, avatarURL)
	if err != nil {
		return errs.Wrap(errs.ESRVDb1001, err)
	}
	if tag.RowsAffected() == 0 {
		return errs.New(errs.EComChar1002, nil)
	}
	return nil
}

// DeleteCharacter 삭제 (세션은 CASCADE)
func (s *Store) DeleteCharacter(ctx context.Context, id, userID uuid.UUID) error {	tag, err := s.pool.Exec(ctx, `DELETE FROM characters WHERE id = $1 AND user_id = $2`, id, userID)
	if err != nil {
		return errs.Wrap(errs.ESRVDb1001, err)
	}
	if tag.RowsAffected() == 0 {
		return errs.New(errs.EComChar1002, nil)
	}
	return nil
}

// ---- 세션 ----

// ListSessions 사용자 대화방 목록 (캐릭터 정보 포함)
func (s *Store) ListSessions(ctx context.Context, userID uuid.UUID) ([]ChatSession, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT cs.id, cs.character_id, cs.user_id, cs.model_id, cs.rule_id, cs.status, COALESCE(cs.summary,''), cs.relation_state, cs.created_at, cs.updated_at,
		        lm.content, lm.created_at
		 FROM chat_sessions cs
		 LEFT JOIN LATERAL (
		   SELECT content, created_at FROM messages WHERE session_id = cs.id ORDER BY created_at DESC LIMIT 1
		 ) lm ON true
		 WHERE cs.user_id = $1 ORDER BY cs.updated_at DESC`, userID)
	if err != nil {
		return nil, errs.Wrap(errs.ESRVDb1001, err)
	}
	defer rows.Close()

	out := make([]ChatSession, 0)
	for rows.Next() {
		sess, err := scanSessionWithLast(rows)
		if err != nil {
			return nil, errs.Wrap(errs.ESRVDb1001, err)
		}
		out = append(out, sess)
	}
	return out, rows.Err()
}

// GetSession 단건 (소유자 검증)
func (s *Store) GetSession(ctx context.Context, id, userID uuid.UUID) (*ChatSession, error) {
	row := s.pool.QueryRow(ctx,
		`SELECT id, character_id, user_id, model_id, rule_id, status, COALESCE(summary,''), relation_state, created_at, updated_at
		 FROM chat_sessions WHERE id = $1 AND user_id = $2`, id, userID)
	sess, err := scanSession(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, errs.New(errs.EComSess1002, nil)
	}
	if err != nil {
		return nil, errs.Wrap(errs.ESRVDb1001, err)
	}
	return &sess, nil
}

// CreateSession 대화방 생성 (캐릭터 소유 검증)
func (s *Store) CreateSession(ctx context.Context, userID uuid.UUID, sess ChatSession) (uuid.UUID, error) {
	if err := s.ensureCharacterOwner(ctx, sess.CharacterID, userID); err != nil {
		return uuid.Nil, err
	}
	var id uuid.UUID
	err := s.pool.QueryRow(ctx,
		`INSERT INTO chat_sessions (character_id, user_id, model_id, rule_id)
		 VALUES ($1,$2,$3,$4) RETURNING id`,
		sess.CharacterID, userID, sess.ModelID, sess.RuleID).Scan(&id)
	if err != nil {
		return uuid.Nil, errs.Wrap(errs.ESRVDb1001, err)
	}
	return id, nil
}

// UpdateSession 상태/요약/관계 갱신
func (s *Store) UpdateSession(ctx context.Context, id, userID uuid.UUID, status, summary string, relationState map[string]any) error {
	tag, err := s.pool.Exec(ctx,
		`UPDATE chat_sessions SET
		   status = COALESCE($3, status),
		   summary = COALESCE($4, summary),
		   relation_state = COALESCE($5, relation_state),
		   updated_at = now()
		 WHERE id = $1 AND user_id = $2`,
		id, userID, nullStr(status), nullStr(summary), relationState)
	if err != nil {
		return errs.Wrap(errs.ESRVDb1001, err)
	}
	if tag.RowsAffected() == 0 {
		return errs.New(errs.EComSess1002, nil)
	}
	return nil
}

// UpdateSessionModelID 세션 모델 변경
func (s *Store) UpdateSessionModelID(ctx context.Context, id, userID uuid.UUID, modelID string) error {
	tag, err := s.pool.Exec(ctx,
		`UPDATE chat_sessions SET model_id = $3, updated_at = now() WHERE id = $1 AND user_id = $2`,
		id, userID, modelID)
	if err != nil {
		return errs.Wrap(errs.ESRVDb1001, err)
	}
	if tag.RowsAffected() == 0 {
		return errs.New(errs.EComSess1002, nil)
	}
	return nil
}

// DeleteSession 삭제 (메시지 CASCADE)
func (s *Store) DeleteSession(ctx context.Context, id, userID uuid.UUID) error {
	tag, err := s.pool.Exec(ctx, `DELETE FROM chat_sessions WHERE id = $1 AND user_id = $2`, id, userID)
	if err != nil {
		return errs.Wrap(errs.ESRVDb1001, err)
	}
	if tag.RowsAffected() == 0 {
		return errs.New(errs.EComSess1002, nil)
	}
	return nil
}

// ---- 메시지 ----

// ListMessages 세션 메시지 (소유 검증)
func (s *Store) ListMessages(ctx context.Context, sessionID, userID uuid.UUID) ([]Message, error) {
	return s.listMessages(ctx, sessionID, userID, 0)
}

// LastMessages 최근 N개 메시지 (채팅 컨텍스트용)
func (s *Store) LastMessages(ctx context.Context, sessionID, userID uuid.UUID, n int) ([]Message, error) {
	return s.listMessages(ctx, sessionID, userID, n)
}

func (s *Store) listMessages(ctx context.Context, sessionID, userID uuid.UUID, limit int) ([]Message, error) {
	if _, err := s.GetSession(ctx, sessionID, userID); err != nil {
		return nil, err
	}
	q := `SELECT id, session_id, role, content, model, token_in, token_out, cost, created_at
		 FROM messages WHERE session_id = $1 ORDER BY created_at`
	args := []any{sessionID}
	if limit > 0 {
		q = `SELECT * FROM (` + q + ` DESC LIMIT $2) sub ORDER BY created_at`
		args = append(args, limit)
	}
	rows, err := s.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, errs.Wrap(errs.ESRVDb1001, err)
	}
	defer rows.Close()

	out := make([]Message, 0)
	for rows.Next() {
		var m Message
		if err := rows.Scan(&m.ID, &m.SessionID, &m.Role, &m.Content, &m.Model, &m.TokenIn, &m.TokenOut, &m.Cost, &m.CreatedAt); err != nil {
			return nil, errs.Wrap(errs.ESRVDb1001, err)
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

// AppendMessage 메시지 저장 (session 소유 검증)
func (s *Store) AppendMessage(ctx context.Context, sessionID, userID uuid.UUID, m Message) (uuid.UUID, error) {
	if _, err := s.GetSession(ctx, sessionID, userID); err != nil {
		return uuid.Nil, err
	}
	var id uuid.UUID
	err := s.pool.QueryRow(ctx,
		`INSERT INTO messages (session_id, role, content, model, token_in, token_out, cost)
		 VALUES ($1,$2,$3,$4,$5,$6,$7) RETURNING id`,
		sessionID, m.Role, m.Content, m.Model, m.TokenIn, m.TokenOut, m.Cost).Scan(&id)
	if err != nil {
		return uuid.Nil, errs.Wrap(errs.ESRVDb1001, err)
	}
	return id, nil
}

// CountMessages 세션 메시지 수 (소유자 검증)
func (s *Store) CountMessages(ctx context.Context, sessionID, userID uuid.UUID) (int64, error) {
	if _, err := s.GetSession(ctx, sessionID, userID); err != nil {
		return 0, err
	}
	var n int64
	err := s.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM messages WHERE session_id = $1`, sessionID).Scan(&n)
	if err != nil {
		return 0, errs.Wrap(errs.ESRVDb1001, err)
	}
	return n, nil
}

// ---- 내부 헬퍼 ----

func (s *Store) ensureCharacterOwner(ctx context.Context, id, userID uuid.UUID) error {
	var exists bool
	err := s.pool.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM characters WHERE id = $1 AND user_id = $2)`, id, userID).Scan(&exists)
	if err != nil {
		return errs.Wrap(errs.ESRVDb1001, err)
	}
	if !exists {
		return errs.New(errs.EComChar1002, nil)
	}
	return nil
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanCharacter(row rowScanner) (Character, error) {
	var c Character
	err := row.Scan(&c.ID, &c.UserID, &c.Name, &c.Title, &c.AvatarURL, &c.Category, &c.Persona, &c.Greeting, &c.Age, &c.Adult, &c.CreatedAt)
	return c, err
}

func scanSession(row rowScanner) (ChatSession, error) {
	var s ChatSession
	err := row.Scan(&s.ID, &s.CharacterID, &s.UserID, &s.ModelID, &s.RuleID, &s.Status, &s.Summary, &s.RelationState, &s.CreatedAt, &s.UpdatedAt)
	return s, err
}

// scanSessionWithLast ListSessions 전용 (마지막 메시지 포함)
func scanSessionWithLast(row rowScanner) (ChatSession, error) {
	var s ChatSession
	err := row.Scan(&s.ID, &s.CharacterID, &s.UserID, &s.ModelID, &s.RuleID, &s.Status, &s.Summary, &s.RelationState, &s.CreatedAt, &s.UpdatedAt, &s.LastMessage, &s.LastMessageAt)
	return s, err
}
