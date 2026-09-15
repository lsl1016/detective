package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func testServer(t *testing.T) (*Server, string) {
	t.Helper()
	dir := t.TempDir()
	casepack := filepath.Join(dir, "casepack")
	if err := os.MkdirAll(casepack, 0o755); err != nil {
		t.Fatal(err)
	}
	cases := []RuntimeCase{
		{CaseID: "CASE-001", Title: "one", Briefing: "b1", Scene: []SceneItem{{ID: "S1", Title: "s", Content: "c"}}, NPCs: []NPC{{ID: "P1", Name: "甲", Testimony: "t"}}, Archive: []ArchiveItem{{ID: "A1", Title: "a", Keywords: []string{"钥匙"}, Content: "K-1"}}},
		{CaseID: "CASE-002", Title: "two", Briefing: "b2", Scene: []SceneItem{{ID: "S9", Title: "s", Content: "c2"}}},
	}
	for _, c := range cases {
		data, _ := json.Marshal(c)
		if err := os.WriteFile(filepath.Join(casepack, c.CaseID+".json"), data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	state := filepath.Join(casepack, "current.json")
	if err := writeCurrentState(state, "CASE-001"); err != nil {
		t.Fatal(err)
	}
	logPath := filepath.Join(dir, "access.jsonl")
	s, err := NewServer(Config{CasepackDir: casepack, CurrentCaseFile: state, AccessLog: logPath, AdminToken: "secret"})
	if err != nil {
		t.Fatal(err)
	}
	return s, logPath
}

func TestOnlyCurrentCaseIsQueryable(t *testing.T) {
	s, _ := testServer(t)
	h := s.routes()

	// There is intentionally no historical-case route.
	r := httptest.NewRequest(http.MethodGet, "/case/CASE-002", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != http.StatusNotFound {
		t.Fatalf("historical route should 404, got %d", w.Code)
	}

	// Tool body cannot smuggle a case_id because strict JSON rejects unknown fields.
	body := bytes.NewBufferString(`{"evidence_id":"S9","case_id":"CASE-002"}`)
	r = httptest.NewRequest(http.MethodPost, "/tool/scene", body)
	w = httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("case_id smuggling should 400, got %d body=%s", w.Code, w.Body.String())
	}

	body = bytes.NewBufferString(`{"evidence_id":"S1"}`)
	r = httptest.NewRequest(http.MethodPost, "/tool/scene", body)
	w = httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("current evidence should be queryable, got %d body=%s", w.Code, w.Body.String())
	}
}

func TestSwitchCase(t *testing.T) {
	s, _ := testServer(t)
	h := s.routes()
	body := bytes.NewBufferString(`{"case_id":"CASE-002","reset_log":true}`)
	r := httptest.NewRequest(http.MethodPost, "/admin/current", body)
	r.Header.Set("X-Admin-Token", "secret")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("switch failed: %d %s", w.Code, w.Body.String())
	}

	body = bytes.NewBufferString(`{"evidence_id":"S1"}`)
	r = httptest.NewRequest(http.MethodPost, "/tool/scene", body)
	w = httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != http.StatusNotFound {
		t.Fatalf("old current evidence must disappear after switch, got %d", w.Code)
	}

	body = bytes.NewBufferString(`{"evidence_id":"S9"}`)
	r = httptest.NewRequest(http.MethodPost, "/tool/scene", body)
	w = httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("new current evidence should be visible, got %d", w.Code)
	}
}

func TestRuntimeGroundTruthLeakRejected(t *testing.T) {
	dir := t.TempDir()
	casepack := filepath.Join(dir, "casepack")
	_ = os.MkdirAll(casepack, 0o755)
	_ = os.WriteFile(filepath.Join(casepack, "current.json"), []byte(`{"case_id":"CASE-001"}`), 0o644)
	_ = os.WriteFile(filepath.Join(casepack, "CASE-001.json"), []byte(`{"case_id":"CASE-001","title":"x","truth":{"culprit":"泄漏"}}`), 0o644)
	_, err := NewServer(Config{CasepackDir: casepack, CurrentCaseFile: filepath.Join(casepack, "current.json"), AccessLog: filepath.Join(dir, "access.jsonl")})
	if err == nil {
		t.Fatal("expected runtime truth leak to be rejected")
	}
}
