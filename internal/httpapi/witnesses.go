package httpapi

import (
	"net/http"

	"task165-collation/internal/model"
)

type createWitnessReq struct {
	Code              string `json:"code"`
	Title             string `json:"title"`
	BibliographicInfo string `json:"bibliographic_info"`
	IsBase            bool   `json:"is_base"`
}

func (a *API) handleCreateWitness(w http.ResponseWriter, r *http.Request) {
	logReq(r)
	projectID := pathID(r, "id")
	var req createWitnessReq
	if err := decode(r, &req); err != nil {
		fail(w, err)
		return
	}
	wt, err := a.svc.CreateWitness(r.Context(), projectID, req.Code, req.Title, req.BibliographicInfo, req.IsBase)
	if err != nil {
		fail(w, err)
		return
	}
	ok(w, wt)
}

func (a *API) handleListWitnesses(w http.ResponseWriter, r *http.Request) {
	logReq(r)
	list, err := a.svc.ListWitnesses(r.Context(), pathID(r, "id"))
	if err != nil {
		fail(w, err)
		return
	}
	ok(w, list)
}

func (a *API) handleGetWitness(w http.ResponseWriter, r *http.Request) {
	logReq(r)
	id := pathID(r, "id")
	// Witness id is globally unique; find its project via the store.
	wt, err := a.svc.Store().GetWitnessFromAny(id)
	if err != nil {
		fail(w, err)
		return
	}
	ok(w, wt)
}

type importChaptersReq struct {
	Titles []string `json:"titles"`
}

func (a *API) handleImportChapters(w http.ResponseWriter, r *http.Request) {
	logReq(r)
	var req importChaptersReq
	if err := decode(r, &req); err != nil {
		fail(w, err)
		return
	}
	chapters, err := a.svc.ImportChapters(r.Context(), pathID(r, "id"), req.Titles)
	if err != nil {
		fail(w, err)
		return
	}
	ok(w, chapters)
}

func (a *API) handleListChapters(w http.ResponseWriter, r *http.Request) {
	logReq(r)
	chapters, err := a.svc.ListChapters(r.Context(), pathID(r, "id"))
	if err != nil {
		fail(w, err)
		return
	}
	ok(w, chapters)
}

type importPassagesReq struct {
	Text string `json:"text"`
}

func (a *API) handleImportPassages(w http.ResponseWriter, r *http.Request) {
	logReq(r)
	var req importPassagesReq
	if err := decode(r, &req); err != nil {
		fail(w, err)
		return
	}
	n, err := a.svc.ImportPassages(r.Context(), pathID(r, "id"), req.Text)
	if err != nil {
		fail(w, err)
		return
	}
	ok(w, map[string]int{"imported": n})
}

func (a *API) handleListPassages(w http.ResponseWriter, r *http.Request) {
	logReq(r)
	passages, err := a.svc.ListPassages(r.Context(), pathID(r, "id"))
	if err != nil {
		fail(w, err)
		return
	}
	ok(w, passages)
}

var _ = model.ProjectDraft
