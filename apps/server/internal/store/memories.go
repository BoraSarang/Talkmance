package store

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/talkmance/server/internal/errs"
)

// Memory 기억 카드 (character_memories)
type Memory struct {
	ID          uuid.UUID `json:"id"`
	CharacterID uuid.UUID `json:"character_id"`
	MemType     string    `json:"mem_type"` // short|medium|long
	Content     string    `json:"content"`
	Importance  float32   `json:"importance"`
	Pinned      bool      `json:"pinned"`
	CreatedAt   time.Time `json:"created_at"`
}

// ListMemories 캐릭터 기억 목록 (소유 검증)
func (s *Store) ListMemories(ctx context.Context, characterID, userID uuid.UUID) ([]Memory, error) {
	if err := s.ensureCharacterOwner(ctx, characterID, userID); err != nil {
		return nil, err
	}
	rows, err := s.pool.Query(ctx,
		`SELECT id, character_id, mem_type, content, importance, pinned, created_at
		 FROM character_memories WHERE character_id = $1
		 ORDER BY pinned DESC, importance DESC, created_at DESC`, characterID)
	if err != nil {
		return nil, errs.Wrap(errs.ESRVDb1001, err)
	}
	defer rows.Close()

	out := make([]Memory, 0)
	for rows.Next() {
		var m Memory
		if err := rows.Scan(&m.ID, &m.CharacterID, &m.MemType, &m.Content, &m.Importance, &m.Pinned, &m.CreatedAt); err != nil {
			return nil, errs.Wrap(errs.ESRVDb1001, err)
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

// CreateMemory 기억 저장 (소유 검증, embedding은 선택)
func (s *Store) CreateMemory(ctx context.Context, characterID, userID uuid.UUID, m Memory) (uuid.UUID, error) {
	if err := s.ensureCharacterOwner(ctx, characterID, userID); err != nil {
		return uuid.Nil, err
	}
	if m.MemType == "" {
		m.MemType = "long"
	}
	var id uuid.UUID
	err := s.pool.QueryRow(ctx,
		`INSERT INTO character_memories (character_id, mem_type, content, importance, pinned)
		 VALUES ($1,$2,$3,$4,$5) RETURNING id`,
		characterID, m.MemType, m.Content, m.Importance, m.Pinned).Scan(&id)
	if err != nil {
		return uuid.Nil, errs.Wrap(errs.ESRVDb1001, err)
	}
	return id, nil
}

// UpdateMemory 기억 수정 (소유 검증 via character join)
func (s *Store) UpdateMemory(ctx context.Context, id, userID uuid.UUID, m Memory) error {
	tag, err := s.pool.Exec(ctx,
		`UPDATE character_memories SET
		   mem_type = $3,
		   content = COALESCE($4, content),
		   importance = $5,
		   pinned = $6
		 WHERE id = $1 AND character_id IN (SELECT id FROM characters WHERE user_id = $2)`,
		id, userID, m.MemType, nullStr(m.Content), m.Importance, m.Pinned)
	if err != nil {
		return errs.Wrap(errs.ESRVDb1001, err)
	}
	if tag.RowsAffected() == 0 {
		return errs.New(errs.EComMem1002, nil)
	}
	return nil
}

// DeleteMemory 기억 삭제 (소유 검증)
func (s *Store) DeleteMemory(ctx context.Context, id, userID uuid.UUID) error {
	tag, err := s.pool.Exec(ctx,
		`DELETE FROM character_memories
		 WHERE id = $1 AND character_id IN (SELECT id FROM characters WHERE user_id = $2)`,
		id, userID)
	if err != nil {
		return errs.Wrap(errs.ESRVDb1001, err)
	}
	if tag.RowsAffected() == 0 {
		return errs.New(errs.EComMem1002, nil)
	}
	return nil
}

// SearchMemoriesByTokens 다중 토큰 ILIKE 검색 (2-gram 확장 포함 — 한국어 형태소 대응, T-21)
// 임베딩 미설정 시 키워드 폴백: 저장 기억과 검색 토큰의 부분 매치(2-gram)로 관련 기억 조회
func (s *Store) SearchMemoriesByTokens(ctx context.Context, characterID, userID uuid.UUID, tokens []string, limit int) ([]Memory, error) {
	if err := s.ensureCharacterOwner(ctx, characterID, userID); err != nil {
		return nil, err
	}
	if limit <= 0 {
		limit = 5
	}
	// 2-gram 확장: "고양이들" → "고양","양이","이들"
	grams := make([]string, 0, len(tokens))
	for _, t := range tokens {
		r := []rune(t)
		grams = append(grams, t)
		if len(r) >= 2 {
			for i := 0; i+2 <= len(r) && len(grams) < 8; i++ {
				grams = append(grams, string(r[i:i+2]))
			}
		}
		if len(grams) >= 8 {
			break
		}
	}

	q := `SELECT id, character_id, mem_type, content, importance, pinned, created_at
		 FROM character_memories WHERE character_id = $1 AND (`
	args := []any{characterID}
	for i, g := range grams {
		if i > 0 {
			q += " OR "
		}
		q += fmt.Sprintf("content ILIKE '%%' || $%d || '%%'", i+2)
		args = append(args, g)
	}
	q += `) ORDER BY pinned DESC, importance DESC LIMIT $` + fmt.Sprintf("%d", len(args)+1)
	args = append(args, limit)

	rows, err := s.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, errs.Wrap(errs.ESRVDb1001, err)
	}
	defer rows.Close()

	out := make([]Memory, 0)
	for rows.Next() {
		var m Memory
		if err := rows.Scan(&m.ID, &m.CharacterID, &m.MemType, &m.Content, &m.Importance, &m.Pinned, &m.CreatedAt); err != nil {
			return nil, errs.Wrap(errs.ESRVDb1001, err)
		}
		out = append(out, m)
	}
	return out, rows.Err()
}
