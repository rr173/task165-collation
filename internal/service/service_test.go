package service

import (
	"context"
	"path/filepath"
	"testing"

	"task165-collation/internal/store"
)

// newTestService opens a service on a temp database.
func newTestService(t *testing.T) (*Service, string) {
	t.Helper()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")
	s, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return New(s), dbPath
}

func TestProjectLifecycle(t *testing.T) {
	svc, _ := newTestService(t)
	ctx := context.Background()

	p, err := svc.CreateProject(ctx, "论语校勘", "测试")
	if err != nil {
		t.Fatalf("create project: %v", err)
	}
	if p.Status != "draft" {
		t.Fatalf("expected draft, got %s", p.Status)
	}

	// Aligning without a base witness must fail.
	if _, err := svc.TransitionProject(ctx, p.ID, p.Version, "aligning"); err == nil {
		t.Fatal("expected error transitioning without base witness")
	}

	base, err := svc.CreateWitness(ctx, p.ID, "B", "底本", "刻本", true)
	if err != nil {
		t.Fatalf("create base: %v", err)
	}
	_ = base

	// Second base must be rejected.
	if _, err := svc.CreateWitness(ctx, p.ID, "B2", "另一底本", "刻本", true); err == nil {
		t.Fatal("expected error for second base")
	}

	p2, err := svc.TransitionProject(ctx, p.ID, p.Version, "aligning")
	if err != nil {
		t.Fatalf("transition to aligning: %v", err)
	}
	if p2.Status != "aligning" {
		t.Fatalf("expected aligning, got %s", p2.Status)
	}
}

func TestImportAndSegmentation(t *testing.T) {
	svc, _ := newTestService(t)
	ctx := context.Background()

	p, _ := svc.CreateProject(ctx, "校勘", "")
	base, _ := svc.CreateWitness(ctx, p.ID, "B", "底本", "", true)

	raw := "第一段文字。\n\n第二段文字。\n\n第三段文字。"
	n, err := svc.ImportPassages(ctx, base.ID, raw)
	if err != nil {
		t.Fatalf("import passages: %v", err)
	}
	if n != 3 {
		t.Fatalf("expected 3 passages, got %d", n)
	}
	passages, err := svc.ListPassages(ctx, base.ID)
	if err != nil {
		t.Fatalf("list passages: %v", err)
	}
	if len(passages) != 3 {
		t.Fatalf("expected 3 persisted passages, got %d", len(passages))
	}
	if passages[0].Ordinal != 1 || passages[1].Ordinal != 2 {
		t.Fatalf("ordinals wrong: %d %d", passages[0].Ordinal, passages[1].Ordinal)
	}
}
