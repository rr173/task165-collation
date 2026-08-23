package service

import (
	"context"
	"testing"
)

func TestBug09_SnapshotEvidenceMapIsAvailable(t *testing.T) {
	svc, _ := newTestService(t)
	ctx := context.Background()
	p, _ := svc.CreateProject(ctx, "证据映射", "")
	base, _ := svc.CreateWitness(ctx, p.ID, "B", "底本", "", true)
	if _, err := svc.ImportPassages(ctx, base.ID, "映射正文。"); err != nil {
		t.Fatal(err)
	}
	sn, err := svc.BuildSnapshot(ctx, p.ID)
	if err != nil {
		t.Fatal(err)
	}
	view, err := svc.GetSnapshot(ctx, sn.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(view.PassageHashes) == 0 {
		t.Fatal("snapshot evidence map unavailable")
	}
}
