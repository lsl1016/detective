package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

type Config struct {
	Addr            string
	CasepackDir     string
	CurrentCaseFile string
	AccessLog       string
	AdminToken      string
}

type SceneItem struct {
	ID      string `json:"id"`
	Title   string `json:"title"`
	Content string `json:"content"`
}

type NPC struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Testimony string `json:"testimony"`
}

type ArchiveItem struct {
	ID       string   `json:"id"`
	Title    string   `json:"title"`
	Keywords []string `json:"keywords,omitempty"`
	Content  string   `json:"content"`
}

type RuntimeCase struct {
	CaseID   string        `json:"case_id"`
	Title    string        `json:"title"`
	Briefing string        `json:"briefing"`
	Scene    []SceneItem   `json:"scene"`
	NPCs     []NPC         `json:"npcs"`
	Archive  []ArchiveItem `json:"archive"`
}

type CurrentState struct {
	CaseID string `json:"case_id"`
}

type AccessEvent struct {
	Time      string `json:"time"`
	CaseID    string `json:"case_id"`
	Line      string `json:"line"`
	Selector  string `json:"selector"`
	Status    int    `json:"status"`
	SessionID string `json:"session_id,omitempty"`
	UserName  string `json:"user_name,omitempty"`
	RunID     string `json:"run_id,omitempty"`
	AgentPath string `json:"agent_path,omitempty"`
	ToolUseID string `json:"tool_use_id,omitempty"`
	Remote    string `json:"remote,omitempty"`
}

type Server struct {
	cfg      Config
	mu       sync.RWMutex
	current  string
	logMu    sync.Mutex
	caseMu   sync.RWMutex
	caseMemo map[string]RuntimeCase
}

func env(name, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(name)); v != "" {
		return v
	}
	return fallback
}

func main() {
	cfg := Config{
		Addr:            env("DETECTIVE_ADDR", ":18081"),
		CasepackDir:     env("DETECTIVE_CASEPACK_DIR", "./casepack"),
		CurrentCaseFile: env("DETECTIVE_CURRENT_CASE_FILE", "./casepack/current.json"),
		AccessLog:       env("DETECTIVE_ACCESS_LOG", "./eval/runs/access.jsonl"),
		AdminToken:      os.Getenv("DETECTIVE_ADMIN_TOKEN"),
	}

	s, err := NewServer(cfg)
	if err != nil {
		log.Fatalf("init case server: %v", err)
	}
	log.Printf("case server listening on %s, current=%s", cfg.Addr, s.currentCaseID())
	log.Printf("runtime casepack=%s; access log=%s", cfg.CasepackDir, cfg.AccessLog)
	if cfg.AdminToken == "" {
		log.Printf("WARNING: DETECTIVE_ADMIN_TOKEN is empty; admin endpoints are open (local dev only)")
	}
	if err := http.ListenAndServe(cfg.Addr, s.routes()); err != nil {
		log.Fatal(err)
	}
}

func NewServer(cfg Config) (*Server, error) {
	if cfg.CasepackDir == "" || cfg.CurrentCaseFile == "" || cfg.AccessLog == "" {
		return nil, errors.New("casepack/current/access-log paths are required")
	}
	if err := os.MkdirAll(filepath.Dir(cfg.AccessLog), 0o755); err != nil {
		return nil, fmt.Errorf("mkdir access log dir: %w", err)
	}
	state, err := readCurrentState(cfg.CurrentCaseFile)
	if err != nil {
		return nil, err
	}
	s := &Server{cfg: cfg, current: state.CaseID, caseMemo: make(map[string]RuntimeCase)}
	if _, err := s.loadRuntimeCase(state.CaseID); err != nil {
		return nil, fmt.Errorf("current case %s unavailable: %w", state.CaseID, err)
	}
	return s, nil
}

func (s *Server) routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", s.handleHealth)
	mux.HandleFunc("/case/current", s.handleCurrent)
	mux.HandleFunc("/tool/scene", s.handleScene)
	mux.HandleFunc("/tool/interview", s.handleInterview)
	mux.HandleFunc("/tool/archive", s.handleArchive)
	mux.HandleFunc("/admin/current", s.handleAdminCurrent)
	mux.HandleFunc("/admin/logs", s.handleAdminLogs)
	return mux
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "current_case": s.currentCaseID()})
}

func (s *Server) handleCurrent(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w)
		return
	}
	c, err := s.currentCase()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	// Deliberately returns only runtime material. There is no endpoint for arbitrary historical case IDs.
	writeJSON(w, http.StatusOK, map[string]any{
		"case_id":  c.CaseID,
		"title":    c.Title,
		"briefing": c.Briefing,
	})
}

type sceneRequest struct {
	EvidenceID string `json:"evidence_id"`
}

func (s *Server) handleScene(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w)
		return
	}
	var req sceneRequest
	if err := decodeStrictJSON(r, &req); err != nil || strings.TrimSpace(req.EvidenceID) == "" {
		s.logAccess(r, "scene", req.EvidenceID, http.StatusBadRequest)
		writeError(w, http.StatusBadRequest, "body must be {\"evidence_id\":\"S1\"}; historical case_id is not accepted")
		return
	}
	c, err := s.currentCase()
	if err != nil {
		s.logAccess(r, "scene", req.EvidenceID, http.StatusInternalServerError)
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	for _, item := range c.Scene {
		if item.ID == req.EvidenceID {
			s.logAccess(r, "scene", req.EvidenceID, http.StatusOK)
			writeJSON(w, http.StatusOK, map[string]any{"case_id": c.CaseID, "line": "scene", "evidence": item})
			return
		}
	}
	s.logAccess(r, "scene", req.EvidenceID, http.StatusNotFound)
	writeError(w, http.StatusNotFound, "evidence not found in current case")
}

type interviewRequest struct {
	NPCName string `json:"npc_name"`
}

func (s *Server) handleInterview(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w)
		return
	}
	var req interviewRequest
	if err := decodeStrictJSON(r, &req); err != nil || strings.TrimSpace(req.NPCName) == "" {
		s.logAccess(r, "people", req.NPCName, http.StatusBadRequest)
		writeError(w, http.StatusBadRequest, "body must be {\"npc_name\":\"姓名\"}; historical case_id is not accepted")
		return
	}
	c, err := s.currentCase()
	if err != nil {
		s.logAccess(r, "people", req.NPCName, http.StatusInternalServerError)
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	for _, npc := range c.NPCs {
		if npc.Name == req.NPCName || npc.ID == req.NPCName {
			s.logAccess(r, "people", req.NPCName, http.StatusOK)
			writeJSON(w, http.StatusOK, map[string]any{"case_id": c.CaseID, "line": "people", "npc": npc})
			return
		}
	}
	s.logAccess(r, "people", req.NPCName, http.StatusNotFound)
	writeError(w, http.StatusNotFound, "npc not found in current case")
}

type archiveRequest struct {
	Query string `json:"query"`
}

func (s *Server) handleArchive(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w)
		return
	}
	var req archiveRequest
	if err := decodeStrictJSON(r, &req); err != nil || strings.TrimSpace(req.Query) == "" {
		s.logAccess(r, "archive", req.Query, http.StatusBadRequest)
		writeError(w, http.StatusBadRequest, "body must be {\"query\":\"keyword\"}; historical case_id is not accepted")
		return
	}
	c, err := s.currentCase()
	if err != nil {
		s.logAccess(r, "archive", req.Query, http.StatusInternalServerError)
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	q := strings.ToLower(strings.TrimSpace(req.Query))
	matches := make([]ArchiveItem, 0)
	for _, item := range c.Archive {
		haystacks := []string{item.ID, item.Title, item.Content}
		haystacks = append(haystacks, item.Keywords...)
		for _, h := range haystacks {
			if strings.Contains(strings.ToLower(h), q) {
				matches = append(matches, item)
				break
			}
		}
	}
	status := http.StatusOK
	if len(matches) == 0 {
		status = http.StatusNotFound
	}
	s.logAccess(r, "archive", req.Query, status)
	if len(matches) == 0 {
		writeError(w, http.StatusNotFound, "no archive entry matched query in current case")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"case_id": c.CaseID, "line": "archive", "query": req.Query, "count": len(matches), "records": matches})
}

type switchRequest struct {
	CaseID   string `json:"case_id"`
	ResetLog bool   `json:"reset_log,omitempty"`
}

func (s *Server) handleAdminCurrent(w http.ResponseWriter, r *http.Request) {
	if !s.authorizedAdmin(r) {
		writeError(w, http.StatusUnauthorized, "invalid X-Admin-Token")
		return
	}
	if r.Method == http.MethodGet {
		writeJSON(w, http.StatusOK, CurrentState{CaseID: s.currentCaseID()})
		return
	}
	if r.Method != http.MethodPost {
		methodNotAllowed(w)
		return
	}
	var req switchRequest
	if err := decodeStrictJSON(r, &req); err != nil || strings.TrimSpace(req.CaseID) == "" {
		writeError(w, http.StatusBadRequest, "body must contain case_id")
		return
	}
	if _, err := s.loadRuntimeCase(req.CaseID); err != nil {
		writeError(w, http.StatusNotFound, "runtime casepack not found")
		return
	}
	if err := writeCurrentState(s.cfg.CurrentCaseFile, req.CaseID); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	s.mu.Lock()
	s.current = req.CaseID
	s.mu.Unlock()
	if req.ResetLog {
		s.logMu.Lock()
		err := os.WriteFile(s.cfg.AccessLog, nil, 0o644)
		s.logMu.Unlock()
		if err != nil {
			writeError(w, http.StatusInternalServerError, fmt.Sprintf("switched case but failed to reset log: %v", err))
			return
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "case_id": req.CaseID, "access_log_reset": req.ResetLog})
}

func (s *Server) handleAdminLogs(w http.ResponseWriter, r *http.Request) {
	if !s.authorizedAdmin(r) {
		writeError(w, http.StatusUnauthorized, "invalid X-Admin-Token")
		return
	}
	if r.Method != http.MethodGet {
		methodNotAllowed(w)
		return
	}
	limit := 100
	if raw := r.URL.Query().Get("limit"); raw != "" {
		if n, err := strconv.Atoi(raw); err == nil && n > 0 && n <= 2000 {
			limit = n
		}
	}
	f, err := os.Open(s.cfg.AccessLog)
	if errors.Is(err, os.ErrNotExist) {
		writeJSON(w, http.StatusOK, map[string]any{"events": []AccessEvent{}})
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer f.Close()
	var all []AccessEvent
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		var ev AccessEvent
		if json.Unmarshal(scanner.Bytes(), &ev) == nil {
			all = append(all, ev)
		}
	}
	if len(all) > limit {
		all = all[len(all)-limit:]
	}
	writeJSON(w, http.StatusOK, map[string]any{"events": all})
}

func (s *Server) currentCaseID() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.current
}

func (s *Server) currentCase() (RuntimeCase, error) {
	return s.loadRuntimeCase(s.currentCaseID())
}

func (s *Server) loadRuntimeCase(caseID string) (RuntimeCase, error) {
	if caseID == "" || strings.Contains(caseID, "/") || strings.Contains(caseID, "\\") || strings.Contains(caseID, "..") {
		return RuntimeCase{}, errors.New("invalid case id")
	}
	s.caseMu.RLock()
	if c, ok := s.caseMemo[caseID]; ok {
		s.caseMu.RUnlock()
		return c, nil
	}
	s.caseMu.RUnlock()

	path := filepath.Join(s.cfg.CasepackDir, caseID+".json")
	data, err := os.ReadFile(path)
	if err != nil {
		return RuntimeCase{}, err
	}
	// Ground truth must never be present in a runtime casepack.
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return RuntimeCase{}, err
	}
	for _, forbidden := range []string{"truth", "meta_plot", "memory_quiz", "evaluation_targets"} {
		if _, ok := raw[forbidden]; ok {
			return RuntimeCase{}, fmt.Errorf("runtime casepack leaks forbidden field %q", forbidden)
		}
	}
	var c RuntimeCase
	if err := json.Unmarshal(data, &c); err != nil {
		return RuntimeCase{}, err
	}
	if c.CaseID != caseID {
		return RuntimeCase{}, fmt.Errorf("casepack id mismatch: filename=%s payload=%s", caseID, c.CaseID)
	}
	s.caseMu.Lock()
	s.caseMemo[caseID] = c
	s.caseMu.Unlock()
	return c, nil
}

func (s *Server) authorizedAdmin(r *http.Request) bool {
	if s.cfg.AdminToken == "" {
		return true
	}
	return r.Header.Get("X-Admin-Token") == s.cfg.AdminToken
}

func (s *Server) logAccess(r *http.Request, line, selector string, status int) {
	ev := AccessEvent{
		Time:      time.Now().UTC().Format(time.RFC3339Nano),
		CaseID:    s.currentCaseID(),
		Line:      line,
		Selector:  selector,
		Status:    status,
		SessionID: r.Header.Get("X-Session-Id"),
		UserName:  r.Header.Get("X-User-Name"),
		RunID:     firstNonEmpty(r.Header.Get("X-Run-Id"), r.Header.Get("X-React-Run-Id")),
		AgentPath: firstNonEmpty(r.Header.Get("X-Agent-Path"), r.Header.Get("X-React-Agent-Path")),
		ToolUseID: firstNonEmpty(r.Header.Get("X-Tool-Use-Id"), r.Header.Get("X-React-Tool-Use-Id")),
		Remote:    r.RemoteAddr,
	}
	lineBytes, _ := json.Marshal(ev)
	s.logMu.Lock()
	defer s.logMu.Unlock()
	f, err := os.OpenFile(s.cfg.AccessLog, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		log.Printf("write access log: %v", err)
		return
	}
	defer f.Close()
	if _, err := f.Write(append(lineBytes, '\n')); err != nil {
		log.Printf("write access log: %v", err)
	}
}

func decodeStrictJSON(r *http.Request, dst any) error {
	defer r.Body.Close()
	dec := json.NewDecoder(http.MaxBytesReader(nil, r.Body, 1<<20))
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		return err
	}
	var extra any
	if err := dec.Decode(&extra); err == nil {
		return errors.New("multiple JSON values")
	}
	return nil
}

func readCurrentState(path string) (CurrentState, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return CurrentState{}, fmt.Errorf("read current state: %w", err)
	}
	var state CurrentState
	if err := json.Unmarshal(data, &state); err != nil {
		return CurrentState{}, fmt.Errorf("decode current state: %w", err)
	}
	if strings.TrimSpace(state.CaseID) == "" {
		return CurrentState{}, errors.New("current state has empty case_id")
	}
	return state, nil
}

func writeCurrentState(path, caseID string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, _ := json.MarshalIndent(CurrentState{CaseID: caseID}, "", "  ")
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, append(data, '\n'), 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]any{"error": msg})
}

func methodNotAllowed(w http.ResponseWriter) {
	writeError(w, http.StatusMethodNotAllowed, "method not allowed")
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}
