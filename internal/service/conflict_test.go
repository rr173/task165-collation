package service

import (
	"context"
	"testing"
	"time"

	"task165-collation/internal/collate"
	"task165-collation/internal/model"
)

func TestClaimConflictAndLeaseExpiry(t *testing.T) {
	svc, _ := newTestService(t)
	ctx := context.Background()

	p, _ := svc.CreateProject(ctx, "校勘", "")
	base, _ := svc.CreateWitness(ctx, p.ID, "B", "底本", "", true)
	wit, _ := svc.CreateWitness(ctx, p.ID, "W", "见证本", "", false)
	if _, err := svc.ImportPassages(ctx, base.ID, "子曰：学而时习之。"); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.ImportPassages(ctx, wit.ID, "子曰：学而时习之。"); err != nil {
		t.Fatal(err)
	}
	basePassages, _ := svc.ListPassages(ctx, base.ID)

	v := &model.Variant{
		ID:            NewID(),
		ProjectID:     p.ID,
		BasePassageID: basePassages[0].ID,
		BaseStartChar: 0,
		BaseEndChar:   3,
		DiffType:      model.DiffWord,
		Status:        model.VariantUnhandled,
		Version:       1,
	}
	if err := svc.Store().CreateVariants([]*model.Variant{v}); err != nil {
		t.Fatal(err)
	}

	// First editor claims.
	res1, err := svc.ClaimVariant(ctx, v.ID, "editor-甲", 60, 0)
	if err != nil {
		t.Fatalf("claim by A: %v", err)
	}
	if !res1.OK {
		t.Fatalf("expected claim OK: %+v", res1)
	}
	// Second editor cannot claim while lease is held.
	res2, err := svc.ClaimVariant(ctx, v.ID, "editor-乙", 60, 0)
	if err != nil {
		t.Fatalf("claim by B: %v", err)
	}
	if res2.OK {
		t.Fatal("expected lease held for B")
	}
	// Same editor can renew (lease_version stays).
	res3, err := svc.ClaimVariant(ctx, v.ID, "editor-甲", 60, 1)
	if err != nil {
		t.Fatalf("renew by A: %v", err)
	}
	if !res3.OK {
		t.Fatalf("expected renew OK: %+v", res3)
	}
	// Release lets B claim after version bump.
	if err := svc.ReleaseVariant(ctx, v.ID, "editor-甲"); err != nil {
		t.Fatalf("release: %v", err)
	}
	res4, err := svc.ClaimVariant(ctx, v.ID, "editor-乙", 60, 1)
	if err != nil {
		t.Fatalf("claim after release: %v", err)
	}
	if !res4.OK {
		t.Fatalf("expected claim OK after release: %+v", res4)
	}
}

func TestDecisionConflict(t *testing.T) {
	svc, _ := newTestService(t)
	ctx := context.Background()

	p, _ := svc.CreateProject(ctx, "校勘", "")
	base, _ := svc.CreateWitness(ctx, p.ID, "B", "底本", "", true)
	wit, _ := svc.CreateWitness(ctx, p.ID, "W", "见证本", "", false)
	if _, err := svc.ImportPassages(ctx, base.ID, "子曰：学而时习之。"); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.ImportPassages(ctx, wit.ID, "子曰：学而时习之。"); err != nil {
		t.Fatal(err)
	}
	basePassages, _ := svc.ListPassages(ctx, base.ID)

	v := &model.Variant{
		ID:            NewID(),
		ProjectID:     p.ID,
		BasePassageID: basePassages[0].ID,
		BaseStartChar: 0,
		BaseEndChar:   3,
		DiffType:      model.DiffWord,
		Status:        model.VariantUnhandled,
		Version:       1,
	}
	if err := svc.Store().CreateVariants([]*model.Variant{v}); err != nil {
		t.Fatal(err)
	}
	r1, err := svc.ProposeReading(ctx, v.ID, wit.ID, "悦", model.DiffWord)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.ProposeDecision(ctx, collate.DecideRequest{
		VariantID:       v.ID,
		ReadingID:       r1.ID,
		Reason:          "通假",
		DecidedBy:       "甲",
		ExpectedVersion: 1,
	}); err != nil {
		t.Fatal(err)
	}
	// Stale version must conflict.
	_, err = svc.ProposeDecision(ctx, collate.DecideRequest{
		VariantID:       v.ID,
		ReadingID:       r1.ID,
		Reason:          "迟到",
		DecidedBy:       "乙",
		ExpectedVersion: 1,
	})
	if err == nil {
		t.Fatal("expected conflict for stale decision")
	}
	_, ok := err.(*model.ConflictDetail)
	if !ok {
		t.Fatalf("expected ConflictDetail, got %T: %v", err, err)
	}
}

func TestLeaseExpiryAllowsReclaim(t *testing.T) {
	svc, _ := newTestService(t)
	ctx := context.Background()

	p, _ := svc.CreateProject(ctx, "校勘", "")
	base, _ := svc.CreateWitness(ctx, p.ID, "B", "底本", "", true)
	if _, err := svc.ImportPassages(ctx, base.ID, "子曰：学而时习之。"); err != nil {
		t.Fatal(err)
	}
	basePassages, _ := svc.ListPassages(ctx, base.ID)

	v := &model.Variant{
		ID:            NewID(),
		ProjectID:     p.ID,
		BasePassageID: basePassages[0].ID,
		BaseStartChar: 0,
		BaseEndChar:   2,
		DiffType:      model.DiffWord,
		Status:        model.VariantUnhandled,
		Version:       1,
	}
	if err := svc.Store().CreateVariants([]*model.Variant{v}); err != nil {
		t.Fatal(err)
	}
	res1, _ := svc.ClaimVariant(ctx, v.ID, "甲", 1, 0) // 1s lease
	if !res1.OK {
		t.Fatalf("claim: %+v", res1)
	}
	// Wait for expiry, then a different editor reclaims.
	WaitForLeaseExpiry(ctx, time.Now().Add(1100*time.Millisecond))
	res2, err := svc.ClaimVariant(ctx, v.ID, "乙", 60, 1)
	if err != nil {
		t.Fatalf("reclaim after expiry: %v", err)
	}
	if !res2.OK {
		t.Fatalf("expected reclaim OK: %+v", res2)
	}
}
