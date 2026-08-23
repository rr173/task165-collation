package service

import (
	"context"
	"testing"
)

func TestBug02_SnapshotRetainsPassageHash(t *testing.T) {
	svc, _ := newTestService(t)
	ctx := context.Background()
	project, _ := svc.CreateProject(ctx, "冻结哈希", "")
	base, _ := svc.CreateWitness(ctx, project.ID, "B", "底本", "", true)
	if _, err := svc.ImportPassages(ctx, base.ID, "不可变段落。"); err != nil { t.Fatal(err) }
	snapshot, err := svc.BuildSnapshot(ctx, project.ID)
	if err != nil { t.Fatal(err) }
	view, err := svc.GetSnapshot(ctx, snapshot.ID)
	if err != nil { t.Fatal(err) }
	for _, hash := range view.PassageHashes {
		if hash == "" { t.Fatal("frozen snapshot returned an empty passage hash") }
		return
	}
	t.Fatal("frozen snapshot did not retain a passage hash")
}
