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
