package httpapi

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"task165-collation/internal/service"
	"task165-collation/internal/store"
)

func newTestAPI(t *testing.T) (*API, *service.Service) {
	t.Helper()
	dir := t.TempDir()
	s, err := store.Open(filepath.Join(dir, "api.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	svc := service.New(s)
	api := New(svc)
	return api, svc
}

func TestWorkspacePageAndAssets(t *testing.T) {
	api, _ := newTestAPI(t)
	for _, tc := range []struct {
		path     string
		contains string
	}{
		{"/", "古籍异文校勘工作台"},
		{"/workspace.js", "refreshProjects"},
		{"/workspace.css", "workspace"},
	} {
		rec := doJSON(t, api, http.MethodGet, tc.path, nil)
		if rec.Code != http.StatusOK {
			t.Fatalf("GET %s: %d %s", tc.path, rec.Code, rec.Body.String())
		}
		if !strings.Contains(rec.Body.String(), tc.contains) {
			t.Fatalf("GET %s did not contain %q", tc.path, tc.contains)
		}
	}
}

func doJSON(t *testing.T, api http.Handler, method, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var buf bytes.Buffer
	if body != nil {
		_ = json.NewEncoder(&buf).Encode(body)
	}
	req := httptest.NewRequest(method, path, &buf)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	api.ServeHTTP(rec, req)
	return rec
}

func TestStatsEndpoint(t *testing.T) {
	api, _ := newTestAPI(t)
	rec := doJSON(t, api, "GET", "/api/stats", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("stats: %d %s", rec.Code, rec.Body.String())
	}
}

func TestProjectAndWitnessFlow(t *testing.T) {
	api, svc := newTestAPI(t)

	// Create project.
	rec := doJSON(t, api, "POST", "/api/projects", map[string]any{"name": "论语校勘", "description": "示例"})
	if rec.Code != http.StatusOK {
		t.Fatalf("create project: %d %s", rec.Code, rec.Body.String())
	}
	var env struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
		t.Fatal(err)
	}
	pid := env.Data.ID
	if pid == "" {
		t.Fatal("empty project id")
	}

	// Add base witness.
	rec = doJSON(t, api, "POST", "/api/projects/"+pid+"/witnesses", map[string]any{
		"code": "B", "title": "底本", "bibliographic_info": "宋刻本", "is_base": true,
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("create witness: %d %s", rec.Code, rec.Body.String())
	}
	var wEnv struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &wEnv); err != nil {
		t.Fatal(err)
	}

	// Import passages.
	rec = doJSON(t, api, "POST", "/api/witnesses/"+wEnv.Data.ID+"/passages", map[string]any{
		"text": "第一段。\n\n第二段。",
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("import passages: %d %s", rec.Code, rec.Body.String())
	}

	// List passages.
	rec = doJSON(t, api, "GET", "/api/witnesses/"+wEnv.Data.ID+"/passages", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("list passages: %d", rec.Code)
	}
	var pEnv struct {
		Data []map[string]any `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &pEnv); err != nil {
		t.Fatal(err)
	}
	if len(pEnv.Data) != 2 {
		t.Fatalf("expected 2 passages, got %d", len(pEnv.Data))
	}

	// Project stats.
	rec = doJSON(t, api, "GET", "/api/projects/"+pid+"/stats", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("project stats: %d", rec.Code)
	}

	_ = svc
}

func TestTransitionEndpoint(t *testing.T) {
	api, _ := newTestAPI(t)
	rec := doJSON(t, api, "POST", "/api/projects", map[string]any{"name": "P"})
	var env struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
		t.Fatal(err)
	}
	// Transition without base must fail (422).
	rec = doJSON(t, api, "POST", "/api/projects/"+env.Data.ID+"/transition", map[string]any{"status": "aligning"})
	if rec.Code != http.StatusUnprocessableEntity && rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected transition failure, got %d %s", rec.Code, rec.Body.String())
	}
}

// TestAnchorDeprecateReleasesViaAPI exercises the reported bug end-to-end
// through the id-only /api/anchors/{id}/deprecate route: after withdrawing a
// candidate anchor, a replacement anchor can be proposed for the same passage.
func TestAnchorDeprecateReleasesViaAPI(t *testing.T) {
	api, _ := newTestAPI(t)

	// Create project + base witness + variant witness.
	rec := doJSON(t, api, "POST", "/api/projects", map[string]any{"name": "锚点释放"})
	var projEnv struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &projEnv); err != nil {
		t.Fatal(err)
	}
	pid := projEnv.Data.ID

	for _, w := range []struct {
		code, title string
		isBase      bool
	}{
		{"B", "底本", true},
		{"W", "见证本", false},
	} {
		rec = doJSON(t, api, "POST", "/api/projects/"+pid+"/witnesses", map[string]any{
			"code": w.code, "title": w.title, "is_base": w.isBase,
		})
		if rec.Code != http.StatusOK {
			t.Fatalf("create witness %s: %d %s", w.code, rec.Code, rec.Body.String())
		}
	}

	// Pull witness ids to import passages.
	rec = doJSON(t, api, "GET", "/api/projects/"+pid+"/witnesses", nil)
	var witEnv struct {
		Data []map[string]any `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &witEnv); err != nil {
		t.Fatal(err)
	}
	var baseID, witID string
	for _, w := range witEnv.Data {
		if b, _ := w["is_base"].(bool); b {
			baseID, _ = w["id"].(string)
		} else {
			witID, _ = w["id"].(string)
		}
	}

	for _, wid := range []string{baseID, witID} {
		rec = doJSON(t, api, "POST", "/api/witnesses/"+wid+"/passages", map[string]any{"text": "学而时习之。"})
		if rec.Code != http.StatusOK {
			t.Fatalf("import passages %s: %d %s", wid, rec.Code, rec.Body.String())
		}
	}

	// Base passage id for anchoring.
	rec = doJSON(t, api, "GET", "/api/witnesses/"+baseID+"/passages", nil)
	var basePassEnv struct {
		Data []map[string]any `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &basePassEnv); err != nil {
		t.Fatal(err)
	}
	basePassageID, _ := basePassEnv.Data[0]["id"].(string)
	rec = doJSON(t, api, "GET", "/api/witnesses/"+witID+"/passages", nil)
	var witPassEnv struct {
		Data []map[string]any `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &witPassEnv); err != nil {
		t.Fatal(err)
	}
	witPassageID, _ := witPassEnv.Data[0]["id"].(string)

	// Propose a candidate anchor.
	rec = doJSON(t, api, "POST", "/api/projects/"+pid+"/anchors", map[string]any{
		"base_passage_id": basePassageID, "witness_id": witID, "witness_passage_id": witPassageID,
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("propose anchor: %d %s", rec.Code, rec.Body.String())
	}
	var anchorEnv struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &anchorEnv); err != nil {
		t.Fatal(err)
	}
	anchorID := anchorEnv.Data.ID

	// Same passage must be occupied before deprecate.
	rec = doJSON(t, api, "POST", "/api/projects/"+pid+"/anchors", map[string]any{
		"base_passage_id": basePassageID, "witness_id": witID, "witness_passage_id": witPassageID,
	})
	if rec.Code != http.StatusConflict {
		t.Fatalf("expected 409 occupied before deprecate, got %d %s", rec.Code, rec.Body.String())
	}

	// Deprecate via the id-only route.
	rec = doJSON(t, api, "POST", "/api/anchors/"+anchorID+"/deprecate", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("deprecate: %d %s", rec.Code, rec.Body.String())
	}

	// A replacement anchor for the same passage must now succeed.
	rec = doJSON(t, api, "POST", "/api/projects/"+pid+"/anchors", map[string]any{
		"base_passage_id": basePassageID, "witness_id": witID, "witness_passage_id": witPassageID,
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("expected replacement anchor after deprecate, got %d %s", rec.Code, rec.Body.String())
	}
}
