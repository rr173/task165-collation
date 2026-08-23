package httpapi

import (
	"net/http"
)

func (a *API) handlePreviewSnapshot(w http.ResponseWriter, r *http.Request) {
	logReq(r)
	body, err := a.svc.PreviewDefinitive(r.Context(), pathID(r, "id"))
	if err != nil {
		fail(w, err)
		return
	}
	ok(w, map[string]string{"body": body})
}

func (a *API) handleBuildSnapshot(w http.ResponseWriter, r *http.Request) {
	logReq(r)
	sn, err := a.svc.BuildSnapshot(r.Context(), pathID(r, "id"))
	if err != nil {
		fail(w, err)
		return
	}
	ok(w, sn)
}

func (a *API) handleListSnapshots(w http.ResponseWriter, r *http.Request) {
	logReq(r)
	list, err := a.svc.ListSnapshots(r.Context(), pathID(r, "id"))
	if err != nil {
		fail(w, err)
		return
	}
	ok(w, list)
}

func (a *API) handleGetSnapshot(w http.ResponseWriter, r *http.Request) {
	logReq(r)
	view, err := a.svc.GetSnapshot(r.Context(), pathID(r, "id"))
	if err != nil {
		fail(w, err)
		return
	}
	ok(w, view)
}

type publishSnapshotReq struct {
	ExpectedVersion int64 `json:"expected_version"`
}

func (a *API) handlePublishSnapshot(w http.ResponseWriter, r *http.Request) {
	logReq(r)
	var req publishSnapshotReq
	if err := decode(r, &req); err != nil {
		fail(w, err)
		return
	}
	sn, err := a.svc.PublishSnapshot(r.Context(), pathID(r, "id"), req.ExpectedVersion)
	if err != nil {
		fail(w, err)
		return
	}
	ok(w, sn)
}

func (a *API) handleRollbackSnapshot(w http.ResponseWriter, r *http.Request) {
	logReq(r)
	sn, err := a.svc.RollbackToSnapshot(r.Context(), pathID(r, "id"))
	if err != nil {
		fail(w, err)
		return
	}
	ok(w, sn)
}
