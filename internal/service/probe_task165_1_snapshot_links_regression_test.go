package service

import (
	"context"
	"testing"
)

func TestBug01_PublishedSnapshotKeepsFrozenLinks(t *testing.T) {
	svc, _ := newTestService(t)
	ctx := context.Background()
	project, err := svc.CreateProject(ctx, "快照追溯", "")
	if err != nil {
		t.Fatal(err)
	}
	base, err := svc.CreateWitness(ctx, project.ID, "B", "底本", "", true)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.ImportPassages(ctx, base.ID, "可追溯正文。"); err != nil {
		t.Fatal(err)
	}
	snapshot, err := svc.BuildSnapshot(ctx, project.ID)
	if err != nil {
		t.Fatal(err)
	}
	view, err := svc.GetSnapshot(ctx, snapshot.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(view.PassageHashes) != 1 {
		t.Fatalf("published snapshot lost its frozen passage evidence: got %d hashes", len(view.PassageHashes))
	}
}
