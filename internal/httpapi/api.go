// Package httpapi exposes the collation workbench as a JSON API under the
// /api prefix. Every handler returns a consistent envelope {ok, data|error}
// and maps domain errors to HTTP status codes.
package httpapi

import (
	"embed"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strconv"
	"strings"

	"task165-collation/internal/model"
	"task165-collation/internal/service"
)

// workspaceAssets is the small browser client for the collation workspace.
// It is embedded so the long-running Go service remains a single deployable
// artifact while still exposing a real page that consumes the JSON API.
//
//go:embed web/index.html web/workspace.js web/workspace.css
var workspaceAssets embed.FS

// API is the HTTP handler root.
type API struct {
	svc *service.Service
	mux *http.ServeMux
}

// New builds the API with all routes registered.
func New(svc *service.Service) *API {
	a := &API{svc: svc, mux: http.NewServeMux()}
	a.routes()
	return a
}

func (a *API) routes() {
	a.mux.HandleFunc("GET /", a.handleWorkspace)
	a.mux.HandleFunc("GET /workspace.js", a.handleWorkspaceScript)
	a.mux.HandleFunc("GET /workspace.css", a.handleWorkspaceStyle)
	a.mux.HandleFunc("GET /api/stats", a.handleStats)
	a.mux.HandleFunc("POST /api/projects", a.handleCreateProject)
	a.mux.HandleFunc("GET /api/projects", a.handleListProjects)
	a.mux.HandleFunc("GET /api/projects/{id}", a.handleGetProject)
	a.mux.HandleFunc("POST /api/projects/{id}/transition", a.handleTransitionProject)
	a.mux.HandleFunc("GET /api/projects/{id}/stats", a.handleProjectStats)

	a.mux.HandleFunc("POST /api/projects/{id}/witnesses", a.handleCreateWitness)
	a.mux.HandleFunc("GET /api/projects/{id}/witnesses", a.handleListWitnesses)
	a.mux.HandleFunc("GET /api/witnesses/{id}", a.handleGetWitness)
	a.mux.HandleFunc("POST /api/witnesses/{id}/chapters", a.handleImportChapters)
	a.mux.HandleFunc("GET /api/witnesses/{id}/chapters", a.handleListChapters)
	a.mux.HandleFunc("POST /api/witnesses/{id}/passages", a.handleImportPassages)
	a.mux.HandleFunc("GET /api/witnesses/{id}/passages", a.handleListPassages)

	a.mux.HandleFunc("POST /api/projects/{id}/anchors", a.handleProposeAnchor)
	a.mux.HandleFunc("POST /api/anchors/{id}/confirm", a.handleConfirmAnchor)
	a.mux.HandleFunc("POST /api/anchors/{id}/deprecate", a.handleDeprecateAnchor)
	a.mux.HandleFunc("GET /api/projects/{id}/anchors", a.handleListAnchors)
	a.mux.HandleFunc("POST /api/projects/{id}/align", a.handleRunAlignment)
	a.mux.HandleFunc("POST /api/projects/{id}/recover", a.handleRecoverAlignment)

	a.mux.HandleFunc("GET /api/projects/{id}/variants", a.handleListVariants)
	a.mux.HandleFunc("GET /api/variants/{id}", a.handleGetVariant)
	a.mux.HandleFunc("POST /api/variants/{id}/claim", a.handleClaimVariant)
	a.mux.HandleFunc("POST /api/variants/{id}/release", a.handleReleaseVariant)
	a.mux.HandleFunc("POST /api/variants/{id}/readings", a.handleProposeReading)
	a.mux.HandleFunc("POST /api/variants/{id}/decide", a.handleProposeDecision)
	a.mux.HandleFunc("GET /api/variants/{id}/trace", a.handleTraceVariant)

	a.mux.HandleFunc("POST /api/decisions/{id}/review", a.handleReviewDecision)
	a.mux.HandleFunc("POST /api/decisions/{id}/withdraw", a.handleWithdrawDecision)

	a.mux.HandleFunc("GET /api/projects/{id}/snapshot/preview", a.handlePreviewSnapshot)
	a.mux.HandleFunc("POST /api/projects/{id}/snapshots", a.handleBuildSnapshot)
	a.mux.HandleFunc("GET /api/projects/{id}/snapshots", a.handleListSnapshots)
	a.mux.HandleFunc("GET /api/snapshots/{id}", a.handleGetSnapshot)
	a.mux.HandleFunc("POST /api/snapshots/{id}/publish", a.handlePublishSnapshot)
	a.mux.HandleFunc("POST /api/projects/{id}/rollback", a.handleRollbackSnapshot)
}

// handleWorkspace serves the browser-facing editing workspace. API requests
// retain the /api prefix so the page and programmatic clients share one
// service contract.
func (a *API) handleWorkspace(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	serveWorkspaceAsset(w, "web/index.html", "text/html; charset=utf-8")
}

func (a *API) handleWorkspaceScript(w http.ResponseWriter, r *http.Request) {
	serveWorkspaceAsset(w, "web/workspace.js", "application/javascript; charset=utf-8")
}

func (a *API) handleWorkspaceStyle(w http.ResponseWriter, r *http.Request) {
	serveWorkspaceAsset(w, "web/workspace.css", "text/css; charset=utf-8")
}

func serveWorkspaceAsset(w http.ResponseWriter, name, contentType string) {
	b, err := workspaceAssets.ReadFile(name)
	if err != nil {
		http.Error(w, "workspace asset unavailable", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", contentType)
	_, _ = w.Write(b)
}

// ServeHTTP implements http.Handler.
func (a *API) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	a.mux.ServeHTTP(w, r)
}

// --- helpers ---

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func ok(w http.ResponseWriter, data any) { writeJSON(w, http.StatusOK, envelope{OK: true, Data: data}) }

type envelope struct {
	OK    bool   `json:"ok"`
	Data  any    `json:"data,omitempty"`
	Error string `json:"error,omitempty"`
}

func fail(w http.ResponseWriter, err error) {
	code := http.StatusInternalServerError
	switch {
	case errors.Is(err, model.ErrNotFound):
		code = http.StatusNotFound
	case errors.Is(err, model.ErrConflict), errors.Is(err, model.ErrDuplicateHash):
		code = http.StatusConflict
	case errors.Is(err, model.ErrLeaseHeld), errors.Is(err, model.ErrAnchorOccupied):
		code = http.StatusConflict
	case errors.Is(err, model.ErrInvalidState), errors.Is(err, model.ErrInvalidRange),
		errors.Is(err, model.ErrCrossChapter), errors.Is(err, model.ErrWrongProject),
		errors.Is(err, model.ErrCycleMovement), errors.Is(err, model.ErrRevokedReading),
		errors.Is(err, model.ErrPublishedReadOnly), errors.Is(err, model.ErrRoundFrozen):
		code = http.StatusUnprocessableEntity
	}
	writeJSON(w, code, envelope{OK: false, Error: err.Error()})
}

func decode(r *http.Request, v any) error {
	defer r.Body.Close()
	return json.NewDecoder(r.Body).Decode(v)
}

func pathID(r *http.Request, name string) string { return r.PathValue(name) }

func pathInt(r *http.Request, name string) int64 {
	v := r.PathValue(name)
	n, _ := strconv.ParseInt(v, 10, 64)
	return n
}

func logReq(r *http.Request) {
	log.Printf("%s %s", r.Method, r.URL.Path)
}

var _ = strings.TrimSpace
