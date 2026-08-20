package db

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadMigrations(t *testing.T) {
	dir := t.TempDir()
	write := func(name, content string) {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("001_init.up.sql", "CREATE TABLE a (id int);")
	write("001_init.down.sql", "DROP TABLE a;")
	write("002_memories.up.sql", "CREATE TABLE b (id int);")
	write("002_memories.down.sql", "DROP TABLE b;")
	write("README.md", "무시되어야 함")

	migs, err := LoadMigrations(dir)
	if err != nil {
		t.Fatalf("LoadMigrations 실패: %v", err)
	}
	if len(migs) != 2 {
		t.Fatalf("마이그레이션 수=%d, want 2", len(migs))
	}
	if migs[0].Version != 1 || migs[1].Version != 2 {
		t.Errorf("버전 정렬 오류: %+v", migs)
	}
	if migs[0].Name != "init" || migs[1].Name != "memories" {
		t.Errorf("이름 파싱 오류: %+v", migs)
	}
	if migs[0].UpSQL == "" || migs[0].DownSQL == "" {
		t.Error("SQL 로딩 누락")
	}
}

func TestLoadMigrationsMissingPair(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "001_init.up.sql"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadMigrations(dir); err == nil {
		t.Fatal("down 누락 시 에러가 나야 함")
	}
}

func TestLoadMigrationsEmptyDir(t *testing.T) {
	migs, err := LoadMigrations(t.TempDir())
	if err != nil {
		t.Fatalf("빈 디렉토리에서 에러: %v", err)
	}
	if len(migs) != 0 {
		t.Errorf("빈 목록이어야 함: %d", len(migs))
	}
}
