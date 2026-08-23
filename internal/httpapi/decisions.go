package httpapi

import (
	"net/http"

	"task165-collation/internal/collate"
)

type reviewReq struct {
	Reviewer       string `json:"reviewer"`
	Approve        bool   `json:"approve"`
	Comment        string `json:"comment"`
	ExpectedVersion int64  `json:"expected_version"`
}

func (a *API) handleReviewDecision(w http.ResponseWriter, r *http.Request) {
	logReq(r)
	var req reviewReq
	if err := decode(r, &req); err != nil {
		fail(w, err)
		return
	}
	d, err := a.svc.ReviewDecision(r.Context(), collate.ReviewRequest{
		DecisionID:      pathID(r, "id"),
		Reviewer:        req.Reviewer,
		Approve:         req.Approve,
		Comment:         req.Comment,
		ExpectedVersion: req.ExpectedVersion,
	})
	if err != nil {
		fail(w, err)
		return
	}
	ok(w, d)
}

type withdrawReq struct {
	ExpectedVersion int64 `json:"expected_version"`
}

func (a *API) handleWithdrawDecision(w http.ResponseWriter, r *http.Request) {
	logReq(r)
	var req withdrawReq
	if err := decode(r, &req); err != nil {
		fail(w, err)
		return
	}
	if err := a.svc.WithdrawDecision(r.Context(), pathID(r, "id"), req.ExpectedVersion); err != nil {
		fail(w, err)
		return
	}
	ok(w, map[string]bool{"withdrawn": true})
}
