package db

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// 마이그레이션 파일 형식: NNN_이름.up.sql / NNN_이름.down.sql
type Migration struct {
	Version int
	Name    string
	UpSQL   string
	DownSQL string
}

// LoadMigrations migrations 디렉토리에서 up/down 파일 쌍 로딩 (버전 오름차순)
func LoadMigrations(dir string) ([]Migration, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("db: 마이그레이션 디렉토리 읽기 실패(%s): %w", dir, err)
	}

	byVersion := map[int]*Migration{}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		// NNN_이름.up.sql / NNN_이름.down.sql 파싱
		idx := strings.LastIndex(name, ".")
		if idx < 0 {
			continue
		}
		base, ext := name[:idx], name[idx+1:]
		if ext != "sql" {
			continue
		}
		var kind string
		switch {
		case strings.HasSuffix(base, ".up"):
			kind = "up"
			base = strings.TrimSuffix(base, ".up")
		case strings.HasSuffix(base, ".down"):
			kind = "down"
			base = strings.TrimSuffix(base, ".down")
		default:
			continue
		}

		verStr, rest, ok := strings.Cut(base, "_")
		if !ok {
			continue
		}
		ver, perr := strconv.Atoi(verStr)
		if perr != nil {
			return nil, fmt.Errorf("db: 버전 파싱 실패 %q: %w", name, perr)
		}

		data, rerr := os.ReadFile(filepath.Join(dir, name))
		if rerr != nil {
			return nil, fmt.Errorf("db: 파일 읽기 실패 %q: %w", name, rerr)
		}

		m, exists := byVersion[ver]
		if !exists {
			m = &Migration{Version: ver, Name: rest}
			byVersion[ver] = m
		}
		if m.Name != rest {
			return nil, fmt.Errorf("db: 버전 %d 이름 불일치 (%q vs %q)", ver, m.Name, rest)
		}
		if kind == "up" {
			m.UpSQL = string(data)
		} else {
			m.DownSQL = string(data)
		}
	}

	migs := make([]Migration, 0, len(byVersion))
	for _, m := range byVersion {
		if m.UpSQL == "" || m.DownSQL == "" {
			return nil, fmt.Errorf("db: 버전 %d(%s)의 up/down 중 하나가 누락됨", m.Version, m.Name)
		}
		migs = append(migs, *m)
	}
	sort.Slice(migs, func(i, j int) bool { return migs[i].Version < migs[j].Version })
	return migs, nil
}

const schemaTable = "schema_migrations"

// ensureSchemaTable 마이그레이션 버전 테이블 생성
func ensureSchemaTable(ctx context.Context, conn *pgxpool.Pool) error {
	_, err := conn.Exec(ctx, fmt.Sprintf(
		`CREATE TABLE IF NOT EXISTS %s (
			version    INTEGER PRIMARY KEY,
			applied_at TIMESTAMPTZ NOT NULL DEFAULT now()
		)`, schemaTable))
	return err
}

// appliedVersions 적용된 버전 목록
func appliedVersions(ctx context.Context, conn *pgxpool.Pool) (map[int]bool, error) {
	rows, err := conn.Query(ctx, fmt.Sprintf("SELECT version FROM %s", schemaTable))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	applied := map[int]bool{}
	for rows.Next() {
		var v int
		if err := rows.Scan(&v); err != nil {
			return nil, err
		}
		applied[v] = true
	}
	return applied, rows.Err()
}

// MigrateUp 미적용 마이그레이션을 버전 순서대로 적용 (각각 트랜잭션)
func MigrateUp(ctx context.Context, conn *pgxpool.Pool, migs []Migration) ([]int, error) {
	if err := ensureSchemaTable(ctx, conn); err != nil {
		return nil, fmt.Errorf("db: 스키마 테이블 생성 실패: %w", err)
	}
	applied, err := appliedVersions(ctx, conn)
	if err != nil {
		return nil, fmt.Errorf("db: 적용 버전 조회 실패: %w", err)
	}
	var done []int
	for _, m := range migs {
		if applied[m.Version] {
			continue
		}
		if err := applyOne(ctx, conn, m.UpSQL, m.Version); err != nil {
			return done, fmt.Errorf("db: 마이그레이션 %d(%s) up 실패: %w", m.Version, m.Name, err)
		}
		done = append(done, m.Version)
	}
	return done, nil
}

// MigrateDown 마지막 버전부터 하나씩 롤백
func MigrateDown(ctx context.Context, conn *pgxpool.Pool, migs []Migration) ([]int, error) {
	if err := ensureSchemaTable(ctx, conn); err != nil {
		return nil, fmt.Errorf("db: 스키마 테이블 생성 실패: %w", err)
	}
	applied, err := appliedVersions(ctx, conn)
	if err != nil {
		return nil, fmt.Errorf("db: 적용 버전 조회 실패: %w", err)
	}
	var done []int
	for i := len(migs) - 1; i >= 0; i-- {
		m := migs[i]
		if !applied[m.Version] {
			continue
		}
		if err := applyOne(ctx, conn, m.DownSQL, 0); err != nil {
			return done, fmt.Errorf("db: 마이그레이션 %d(%s) down 실패: %w", m.Version, m.Name, err)
		}
		if _, err := conn.Exec(ctx, fmt.Sprintf("DELETE FROM %s WHERE version = $1", schemaTable), m.Version); err != nil {
			return done, fmt.Errorf("db: 버전 기록 삭제 실패: %w", err)
		}
		done = append(done, m.Version)
	}
	return done, nil
}

// applyOne 단일 마이그레이션을 트랜잭션으로 실행 + 버전 기록
// version=0이면 기록 생략 (down 시 삭제는 호출부에서 처리)
func applyOne(ctx context.Context, conn *pgxpool.Pool, sql string, version int) error {
	tx, err := conn.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err := tx.Exec(ctx, sql); err != nil {
		return err
	}
	if version > 0 {
		if _, err := tx.Exec(ctx, fmt.Sprintf("INSERT INTO %s (version) VALUES ($1)", schemaTable), version); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}
