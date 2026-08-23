package service

import (
	"context"
	"testing"
)

func TestBug10_DeprecatedAnchorReleasesPassage(t *testing.T) {
	svc, _ := newTestService(t)
	ctx := context.Background()
	p, _ := svc.CreateProject(ctx, "锚点释放", "")
	base, _ := svc.CreateWitness(ctx, p.ID, "B", "底本", "", true)
	witness, _ := svc.CreateWitness(ctx, p.ID, "W", "见证本", "", false)
	if _, err := svc.ImportPassages(ctx, base.ID, "底本文字。"); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.ImportPassages(ctx, witness.ID, "见证文字。"); err != nil {
		t.Fatal(err)
	}
	basePassages, _ := svc.ListPassages(ctx, base.ID)
	witnessPassages, _ := svc.ListPassages(ctx, witness.ID)
	first, err := svc.ProposeAnchor(ctx, p.ID, basePassages[0].ID, witness.ID, witnessPassages[0].ID, "撤销前的锚点")
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.DeprecateAnchor(ctx, p.ID, first.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.ProposeAnchor(ctx, p.ID, basePassages[0].ID, witness.ID, witnessPassages[0].ID, "替换锚点"); err != nil {
		t.Fatalf("deprecated anchor should release its passage: %v", err)
	}
}
