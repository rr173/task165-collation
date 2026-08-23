package httpapi

import (
	"net/http"

	"task165-collation/internal/collate"
	"task165-collation/internal/model"
)

func (a *API) handleListVariants(w http.ResponseWriter, r *http.Request) {
	logReq(r)
	status := r.URL.Query().Get("status")
	variants, err := a.svc.ListVariants(r.Context(), pathID(r, "id"), status)
	if err != nil {
		fail(w, err)
		return
	}
	ok(w, variants)
}

func (a *API) handleGetVariant(w http.ResponseWriter, r *http.Request) {
	logReq(r)
	v, readings, err := a.svc.GetVariant(r.Context(), pathID(r, "id"))
	if err != nil {
		fail(w, err)
		return
	}
	ok(w, map[string]any{"variant": v, "readings": readings})
}

type claimVariantReq struct {
	Editor       string `json:"editor"`
	LeaseSeconds int64  `json:"lease_seconds"`
	ExpectLease  int64  `json:"expect_lease"`
}

func (a *API) handleClaimVariant(w http.ResponseWriter, r *http.Request) {
	logReq(r)
	var req claimVariantReq
	if err := decode(r, &req); err != nil {
		fail(w, err)
		return
	}
	if req.LeaseSeconds <= 0 {
		req.LeaseSeconds = 300
	}
	res, err := a.svc.ClaimVariant(r.Context(), pathID(r, "id"), req.Editor, req.LeaseSeconds, req.ExpectLease)
	if err != nil {
		fail(w, err)
		return
	}
	if !res.OK {
		fail(w, model.ErrLeaseHeld)
		return
	}
	ok(w, res)
}

type releaseVariantReq struct {
	Editor string `json:"editor"`
}

func (a *API) handleReleaseVariant(w http.ResponseWriter, r *http.Request) {
	logReq(r)
	var req releaseVariantReq
	if err := decode(r, &req); err != nil {
		fail(w, err)
		return
	}
	if err := a.svc.ReleaseVariant(r.Context(), pathID(r, "id"), req.Editor); err != nil {
		fail(w, err)
		return
	}
	ok(w, map[string]bool{"released": true})
}

type proposeReadingReq struct {
	WitnessID string `json:"witness_id"`
	Text      string `json:"text"`
	DiffType  string `json:"diff_type"`
}

func (a *API) handleProposeReading(w http.ResponseWriter, r *http.Request) {
	logReq(r)
	var req proposeReadingReq
	if err := decode(r, &req); err != nil {
		fail(w, err)
		return
	}
	reading, err := a.svc.ProposeReading(r.Context(), pathID(r, "id"), req.WitnessID, req.Text, model.DiffType(req.DiffType))
	if err != nil {
		fail(w, err)
		return
	}
	ok(w, reading)
}

type decideReq struct {
	ReadingID         string `json:"reading_id"`
	Reason            string `json:"reason"`
	DecidedBy         string `json:"decided_by"`
	EvidenceWitnessID string `json:"evidence_witness_id"`
	ExpectedVersion   int64  `json:"expected_version"`
}

func (a *API) handleProposeDecision(w http.ResponseWriter, r *http.Request) {
	logReq(r)
	var req decideReq
	if err := decode(r, &req); err != nil {
		fail(w, err)
		return
	}
	d, err := a.svc.ProposeDecision(r.Context(), collate.DecideRequest{
		VariantID:         pathID(r, "id"),
		ReadingID:         req.ReadingID,
		Reason:            req.Reason,
		DecidedBy:         req.DecidedBy,
		EvidenceWitnessID: req.EvidenceWitnessID,
		ExpectedVersion:   req.ExpectedVersion,
	})
	if err != nil {
		fail(w, err)
		return
	}
	ok(w, d)
}

func (a *API) handleTraceVariant(w http.ResponseWriter, r *http.Request) {
	logReq(r)
	tc, err := a.svc.TraceVariant(r.Context(), pathID(r, "id"))
	if err != nil {
		fail(w, err)
		return
	}
	ok(w, tc)
}
