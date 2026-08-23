package httpapi

import (
	"net/http"

	"task165-collation/internal/model"
)

// handleStats returns global counters across all projects.
func (a *API) handleStats(w http.ResponseWriter, r *http.Request) {
	logReq(r)
	projects, witnesses, passages, anchors, variants, decisions, snapshots, err := a.svc.Store().GlobalCounts()
	if err != nil {
		fail(w, err)
		return
	}
	ok(w, map[string]int{
		"projects":  projects,
		"witnesses": witnesses,
		"passages":  passages,
		"anchors":   anchors,
		"variants":  variants,
		"decisions": decisions,
		"snapshots": snapshots,
	})
}

type createProjectReq struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

func (a *API) handleCreateProject(w http.ResponseWriter, r *http.Request) {
	logReq(r)
	var req createProjectReq
	if err := decode(r, &req); err != nil {
		fail(w, err)
		return
	}
	p, err := a.svc.CreateProject(r.Context(), req.Name, req.Description)
	if err != nil {
		fail(w, err)
		return
	}
	ok(w, p)
}

func (a *API) handleListProjects(w http.ResponseWriter, r *http.Request) {
	logReq(r)
	projects, err := a.svc.ListProjects(r.Context())
	if err != nil {
		fail(w, err)
		return
	}
	ok(w, projects)
}

func (a *API) handleGetProject(w http.ResponseWriter, r *http.Request) {
	logReq(r)
	p, err := a.svc.GetProject(r.Context(), pathID(r, "id"))
	if err != nil {
		fail(w, err)
		return
	}
	ok(w, p)
}

type transitionReq struct {
	Status string `json:"status"`
}

func (a *API) handleTransitionProject(w http.ResponseWriter, r *http.Request) {
	logReq(r)
	id := pathID(r, "id")
	var req transitionReq
	if err := decode(r, &req); err != nil {
		fail(w, err)
		return
	}
	cur, err := a.svc.GetProject(r.Context(), id)
	if err != nil {
		fail(w, err)
		return
	}
	p, err := a.svc.TransitionProject(r.Context(), id, cur.Version, statusFromString(req.Status))
	if err != nil {
		fail(w, err)
		return
	}
	ok(w, p)
}

func (a *API) handleProjectStats(w http.ResponseWriter, r *http.Request) {
	logReq(r)
	st, err := a.svc.ProjectStats(r.Context(), pathID(r, "id"))
	if err != nil {
		fail(w, err)
		return
	}
	ok(w, st)
}

// statusFromString maps a raw status string to a ProjectStatus. Strict
// transition validation happens in the service layer.
func statusFromString(s string) model.ProjectStatus { return model.ProjectStatus(s) }
