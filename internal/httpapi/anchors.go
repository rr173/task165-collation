package httpapi

import (
	"net/http"
	"strconv"

	"task165-collation/internal/model"
)

type proposeAnchorReq struct {
	BasePassageID    string `json:"base_passage_id"`
	WitnessID        string `json:"witness_id"`
	WitnessPassageID string `json:"witness_passage_id"`
	Note             string `json:"note"`
}

func (a *API) handleProposeAnchor(w http.ResponseWriter, r *http.Request) {
	logReq(r)
	projectID := pathID(r, "id")
	var req proposeAnchorReq
	if err := decode(r, &req); err != nil {
		fail(w, err)
		return
	}
	anchor, err := a.svc.ProposeAnchor(r.Context(), projectID, req.BasePassageID, req.WitnessID, req.WitnessPassageID, req.Note)
	if err != nil {
		fail(w, err)
		return
	}
	ok(w, anchor)
}

type confirmAnchorReq struct {
	Version int64 `json:"version"`
}

func (a *API) handleConfirmAnchor(w http.ResponseWriter, r *http.Request) {
	logReq(r)
	var req confirmAnchorReq
	if err := decode(r, &req); err != nil {
		fail(w, err)
		return
	}
	anchor, err := a.svc.ConfirmAnchor(r.Context(), "", pathID(r, "id"), req.Version)
	if err != nil {
		fail(w, err)
		return
	}
	ok(w, anchor)
}

func (a *API) handleDeprecateAnchor(w http.ResponseWriter, r *http.Request) {
	logReq(r)
	if err := a.svc.DeprecateAnchor(r.Context(), "", pathID(r, "id")); err != nil {
		fail(w, err)
		return
	}
	ok(w, map[string]bool{"deprecated": true})
}

func (a *API) handleListAnchors(w http.ResponseWriter, r *http.Request) {
	logReq(r)
	anchors, err := a.svc.ListAnchors(r.Context(), pathID(r, "id"))
	if err != nil {
		fail(w, err)
		return
	}
	ok(w, anchors)
}

func (a *API) handleRunAlignment(w http.ResponseWriter, r *http.Request) {
	logReq(r)
	n, err := a.svc.RunAlignment(r.Context(), pathID(r, "id"))
	if err != nil {
		fail(w, err)
		return
	}
	ok(w, map[string]int{"variants_created": n})
}

func (a *API) handleRecoverAlignment(w http.ResponseWriter, r *http.Request) {
	logReq(r)
	n, err := a.svc.RecoverAlignment(r.Context(), pathID(r, "id"))
	if err != nil {
		fail(w, err)
		return
	}
	ok(w, map[string]int{"affected_intervals": n})
}

var _ = strconv.Itoa
var _ = model.VariantUnhandled
