package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode"
)

type GroundTruth struct {
	CaseID            string              `json:"case_id"`
	Title             string              `json:"title"`
	Truth             Truth               `json:"truth"`
	MetaPlot          []MetaHint          `json:"meta_plot"`
	MemoryQuiz        []Quiz              `json:"memory_quiz"`
	EvaluationTargets map[string][]string `json:"evaluation_targets"`
}

type Truth struct {
	Culprit             string   `json:"culprit"`
	Method              string   `json:"method"`
	KeyEvidence         []string `json:"key_evidence"`
	MethodRequiredTerms []string `json:"method_required_terms"`
	SupportingEvidence  []string `json:"supporting_evidence"`
}

type MetaHint struct {
	HintID     string   `json:"hint_id"`
	Content    string   `json:"content"`
	SourceRefs []string `json:"source_refs"`
	TruthLink  string   `json:"truth_link"`
}

type Quiz struct {
	ID      string   `json:"id"`
	Q       string   `json:"q"`
	A       string   `json:"a"`
	Aliases []string `json:"aliases"`
}

type RuntimeCase struct {
	CaseID   string        `json:"case_id"`
	Title    string        `json:"title"`
	Briefing string        `json:"briefing"`
	Scene    []RuntimeItem `json:"scene"`
	NPCs     []RuntimeNPC  `json:"npcs"`
	Archive  []RuntimeItem `json:"archive"`
}

type RuntimeItem struct {
	ID       string   `json:"id"`
	Title    string   `json:"title"`
	Content  string   `json:"content"`
	Keywords []string `json:"keywords,omitempty"`
}

type RuntimeNPC struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Testimony string `json:"testimony"`
}

type Verdict struct {
	CaseID       string   `json:"case_id"`
	Culprit      string   `json:"culprit"`
	Method       string   `json:"method"`
	EvidenceRefs []string `json:"evidence_refs"`
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
}

type CheckResult struct {
	Name    string         `json:"name"`
	Pass    bool           `json:"pass"`
	Summary string         `json:"summary"`
	Details map[string]any `json:"details,omitempty"`
}

type Report struct {
	CaseID  string        `json:"case_id,omitempty"`
	Results []CheckResult `json:"results"`
}

func main() {
	mode := flag.String("mode", "selfcheck", "selfcheck|case|quiz|access|replay|all")
	caseID := flag.String("case", "", "case id, e.g. CASE-001")
	verdictPath := flag.String("verdict", "", "path to verdict JSON")
	answersPath := flag.String("answers", "", "path to quiz answers JSON")
	accessLog := flag.String("access-log", "./eval/runs/access.jsonl", "case server access log JSONL")
	replayPath := flag.String("replay", "", "optional exported session events JSON")
	gtDir := flag.String("ground-truth-dir", "./eval/ground_truth", "ground truth directory")
	casepackDir := flag.String("casepack-dir", "./casepack", "runtime casepack directory")
	out := flag.String("out", "", "optional report JSON output path")
	flag.Parse()

	report := Report{CaseID: *caseID}
	var err error

	switch *mode {
	case "selfcheck":
		report.Results, err = selfcheck(*gtDir, *casepackDir)
	case "case":
		report.Results, err = evalCase(*gtDir, *casepackDir, *caseID, *verdictPath)
	case "quiz":
		report.Results, err = evalQuiz(*gtDir, *caseID, *answersPath)
	case "access":
		report.Results, err = evalAccess(*caseID, *accessLog)
	case "replay":
		report.Results, err = evalReplay(*replayPath)
	case "all":
		var chunks [][]CheckResult
		var rs []CheckResult
		rs, err = selfcheck(*gtDir, *casepackDir)
		chunks = append(chunks, rs)
		if err == nil && *verdictPath != "" {
			rs, err = evalCase(*gtDir, *casepackDir, *caseID, *verdictPath)
			chunks = append(chunks, rs)
		}
		if err == nil && *answersPath != "" {
			rs, err = evalQuiz(*gtDir, *caseID, *answersPath)
			chunks = append(chunks, rs)
		}
		if err == nil && *accessLog != "" {
			rs, err = evalAccess(*caseID, *accessLog)
			chunks = append(chunks, rs)
		}
		if err == nil && *replayPath != "" {
			rs, err = evalReplay(*replayPath)
			chunks = append(chunks, rs)
		}
		for _, c := range chunks {
			report.Results = append(report.Results, c...)
		}
	default:
		err = fmt.Errorf("unknown mode %q", *mode)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "evaluator error:", err)
		os.Exit(2)
	}

	printText(report)
	if *out != "" {
		data, _ := json.MarshalIndent(report, "", "  ")
		if err := os.MkdirAll(filepath.Dir(*out), 0o755); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(2)
		}
		if err := os.WriteFile(*out, append(data, '\n'), 0o644); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(2)
		}
		fmt.Println("report:", *out)
	}
	for _, r := range report.Results {
		if !r.Pass {
			os.Exit(1)
		}
	}
}

func selfcheck(gtDir, casepackDir string) ([]CheckResult, error) {
	files, err := filepath.Glob(filepath.Join(gtDir, "CASE-*.json"))
	if err != nil {
		return nil, err
	}
	if len(files) == 0 {
		return nil, errors.New("no ground truth files found")
	}
	sort.Strings(files)
	results := []CheckResult{}
	for _, gtPath := range files {
		var gt GroundTruth
		if err := readJSON(gtPath, &gt); err != nil {
			return nil, err
		}
		runtimePath := filepath.Join(casepackDir, gt.CaseID+".json")
		data, err := os.ReadFile(runtimePath)
		if err != nil {
			results = append(results, CheckResult{Name: "runtime_exists:" + gt.CaseID, Pass: false, Summary: err.Error()})
			continue
		}
		var raw map[string]json.RawMessage
		if err := json.Unmarshal(data, &raw); err != nil {
			return nil, err
		}
		leaks := []string{}
		for _, k := range []string{"truth", "meta_plot", "memory_quiz", "evaluation_targets"} {
			if _, ok := raw[k]; ok {
				leaks = append(leaks, k)
			}
		}
		results = append(results, CheckResult{
			Name:    "no_ground_truth_leak:" + gt.CaseID,
			Pass:    len(leaks) == 0,
			Summary: fmt.Sprintf("forbidden fields in runtime: %v", leaks),
		})

		var rc RuntimeCase
		if err := json.Unmarshal(data, &rc); err != nil {
			return nil, err
		}
		ids := runtimeIDs(rc)
		missingRefs := []string{}
		for _, ref := range gt.Truth.KeyEvidence {
			if !ids[ref] {
				missingRefs = append(missingRefs, ref)
			}
		}
		results = append(results, CheckResult{
			Name:    "truth_refs_resolve:" + gt.CaseID,
			Pass:    len(missingRefs) == 0,
			Summary: fmt.Sprintf("missing key evidence refs: %v", missingRefs),
		})

		missingMetaRefs := []string{}
		for _, hint := range gt.MetaPlot {
			if len(hint.SourceRefs) == 0 {
				missingMetaRefs = append(missingMetaRefs, hint.HintID+":<none>")
				continue
			}
			for _, ref := range hint.SourceRefs {
				if !ids[ref] {
					missingMetaRefs = append(missingMetaRefs, hint.HintID+":"+ref)
				}
			}
		}
		results = append(results, CheckResult{
			Name:    "meta_plot_refs_resolve:" + gt.CaseID,
			Pass:    len(missingMetaRefs) == 0,
			Summary: fmt.Sprintf("missing meta source refs: %v", missingMetaRefs),
		})

		runtimeText := strings.ToLower(string(data))
		missingAnswers := []string{}
		for _, q := range gt.MemoryQuiz {
			candidates := append([]string{q.A}, q.Aliases...)
			found := false
			for _, c := range candidates {
				if c != "" && strings.Contains(runtimeText, strings.ToLower(c)) {
					found = true
					break
				}
			}
			if !found {
				missingAnswers = append(missingAnswers, q.ID)
			}
		}
		results = append(results, CheckResult{
			Name:    "quiz_answers_exist_in_runtime:" + gt.CaseID,
			Pass:    len(missingAnswers) == 0,
			Summary: fmt.Sprintf("quiz answers absent from runtime material: %v", missingAnswers),
		})
	}

	var state struct {
		CaseID string `json:"case_id"`
	}
	statePath := filepath.Join(casepackDir, "current.json")
	if err := readJSON(statePath, &state); err != nil {
		return results, err
	}
	_, err = os.Stat(filepath.Join(casepackDir, state.CaseID+".json"))
	results = append(results, CheckResult{
		Name:    "current_case_resolves",
		Pass:    err == nil,
		Summary: fmt.Sprintf("current=%s", state.CaseID),
	})
	return results, nil
}

func evalCase(gtDir, casepackDir, caseID, verdictPath string) ([]CheckResult, error) {
	if caseID == "" || verdictPath == "" {
		return nil, errors.New("-case and -verdict are required")
	}
	gt, rc, err := loadCasePair(gtDir, casepackDir, caseID)
	if err != nil {
		return nil, err
	}
	var v Verdict
	if err := readJSON(verdictPath, &v); err != nil {
		return nil, err
	}
	results := []CheckResult{}
	results = append(results, CheckResult{
		Name:    "case_id",
		Pass:    v.CaseID == caseID,
		Summary: fmt.Sprintf("got=%s want=%s", v.CaseID, caseID),
	})
	results = append(results, CheckResult{
		Name:    "culprit",
		Pass:    normalize(v.Culprit) == normalize(gt.Truth.Culprit),
		Summary: fmt.Sprintf("got=%s want=%s", v.Culprit, gt.Truth.Culprit),
	})

	missingTerms := []string{}
	for _, term := range gt.Truth.MethodRequiredTerms {
		if !strings.Contains(normalize(v.Method), normalize(term)) {
			missingTerms = append(missingTerms, term)
		}
	}
	results = append(results, CheckResult{
		Name:    "method_terms",
		Pass:    len(missingTerms) == 0,
		Summary: fmt.Sprintf("missing required method terms: %v", missingTerms),
	})

	provided := make(map[string]bool)
	for _, ref := range v.EvidenceRefs {
		provided[ref] = true
	}
	missingKey := []string{}
	for _, ref := range gt.Truth.KeyEvidence {
		if !provided[ref] {
			missingKey = append(missingKey, ref)
		}
	}
	results = append(results, CheckResult{
		Name:    "key_evidence_coverage",
		Pass:    len(missingKey) == 0,
		Summary: fmt.Sprintf("missing key evidence: %v", missingKey),
	})

	validIDs := runtimeIDs(rc)
	invalidRefs := []string{}
	for _, ref := range v.EvidenceRefs {
		if !validIDs[ref] {
			invalidRefs = append(invalidRefs, ref)
		}
	}
	results = append(results, CheckResult{
		Name:    "evidence_refs_valid",
		Pass:    len(invalidRefs) == 0,
		Summary: fmt.Sprintf("unknown evidence refs: %v", invalidRefs),
	})

	lineCoverage := map[string]bool{"scene": false, "people": false, "archive": false}
	for _, ref := range v.EvidenceRefs {
		for _, x := range rc.Scene {
			if x.ID == ref {
				lineCoverage["scene"] = true
			}
		}
		for _, x := range rc.NPCs {
			if x.ID == ref {
				lineCoverage["people"] = true
			}
		}
		for _, x := range rc.Archive {
			if x.ID == ref {
				lineCoverage["archive"] = true
			}
		}
	}
	missingLines := []string{}
	for _, line := range []string{"scene", "people", "archive"} {
		if !lineCoverage[line] {
			missingLines = append(missingLines, line)
		}
	}
	results = append(results, CheckResult{
		Name:    "three_line_synthesis",
		Pass:    len(missingLines) == 0,
		Summary: fmt.Sprintf("missing evidence lines: %v", missingLines),
	})
	return results, nil
}

func evalQuiz(gtDir, caseID, answersPath string) ([]CheckResult, error) {
	if caseID == "" || answersPath == "" {
		return nil, errors.New("-case and -answers are required")
	}
	var gt GroundTruth
	if err := readJSON(filepath.Join(gtDir, caseID+".json"), &gt); err != nil {
		return nil, err
	}
	answers := map[string]string{}
	if err := readJSON(answersPath, &answers); err != nil {
		return nil, err
	}
	correct := 0
	wrong := []string{}
	missing := []string{}
	for _, q := range gt.MemoryQuiz {
		got, ok := answers[q.ID]
		if !ok {
			missing = append(missing, q.ID)
			continue
		}
		accepted := append([]string{q.A}, q.Aliases...)
		ok = false
		for _, a := range accepted {
			if normalize(got) == normalize(a) {
				ok = true
				break
			}
		}
		if ok {
			correct++
		} else {
			wrong = append(wrong, fmt.Sprintf("%s got=%q", q.ID, got))
		}
	}
	pass := correct == len(gt.MemoryQuiz)
	return []CheckResult{{
		Name:    "memory_quiz",
		Pass:    pass,
		Summary: fmt.Sprintf("correct=%d/%d wrong=%v missing=%v", correct, len(gt.MemoryQuiz), wrong, missing),
		Details: map[string]any{"correct": correct, "total": len(gt.MemoryQuiz), "wrong": wrong, "missing": missing},
	}}, nil
}

func evalAccess(caseID, path string) ([]CheckResult, error) {
	f, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return []CheckResult{{Name: "access_log", Pass: false, Summary: "access log does not exist"}}, nil
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()

	allowed := map[string]string{"det-scene": "scene", "det-people": "people", "det-archive": "archive"}
	counts := map[string]int{"scene": 0, "people": 0, "archive": 0}
	violations := []string{}
	unverifiable := 0
	mainDirect := 0
	matched := 0
	s := bufio.NewScanner(f)
	for s.Scan() {
		var ev AccessEvent
		if json.Unmarshal(s.Bytes(), &ev) != nil || (caseID != "" && ev.CaseID != caseID) {
			continue
		}
		matched++
		counts[ev.Line]++
		pathLower := strings.ToLower(ev.AgentPath)
		if pathLower == "" {
			unverifiable++
			continue
		}
		if pathLower == "main" || pathLower == "main/" {
			mainDirect++
			continue
		}
		classified := false
		for agent, line := range allowed {
			if strings.Contains(pathLower, agent) {
				classified = true
				if ev.Line != line {
					violations = append(violations, fmt.Sprintf("agent=%s line=%s selector=%s", ev.AgentPath, ev.Line, ev.Selector))
				}
				break
			}
		}
		if !classified && strings.HasPrefix(pathLower, "main/") {
			unverifiable++
		}
	}
	if err := s.Err(); err != nil {
		return nil, err
	}
	return []CheckResult{
		{
			Name:    "subagent_tool_isolation_from_access_log",
			Pass:    len(violations) == 0,
			Summary: fmt.Sprintf("events=%d violations=%d unverifiable_agent_path=%d", matched, len(violations), unverifiable),
			Details: map[string]any{"violations": violations, "line_counts": counts},
		},
		{
			Name:    "main_direct_tool_calls",
			Pass:    true,
			Summary: fmt.Sprintf("main direct calls with explicit agent_path=%d (metric only; lower is better)", mainDirect),
		},
	}, nil
}

func evalReplay(path string) ([]CheckResult, error) {
	if path == "" {
		return nil, errors.New("-replay is required")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var root any
	if err := json.Unmarshal(data, &root); err != nil {
		return nil, err
	}
	type call struct{ AgentPath, ToolName string }
	var calls []call
	walkJSON(root, func(m map[string]any) {
		typeVal, _ := m["type"].(string)
		if typeVal != "tool_use_start" {
			return
		}
		agentPath, _ := m["agentPath"].(string)
		payload, _ := m["payload"].(map[string]any)
		toolName, _ := payload["toolName"].(string)
		if toolName != "" {
			calls = append(calls, call{agentPath, toolName})
		}
	})

	toolLine := map[string]string{"case_scene": "scene", "case_interview": "people", "case_archive": "archive"}
	agentLine := map[string]string{"det-scene": "scene", "det-people": "people", "det-archive": "archive"}
	violations := []string{}
	mainDirect := 0
	caseToolCalls := 0
	for _, c := range calls {
		line, isCaseTool := toolLine[c.ToolName]
		if !isCaseTool {
			continue
		}
		caseToolCalls++
		ap := strings.ToLower(c.AgentPath)
		if ap == "" || ap == "main" {
			mainDirect++
			continue
		}
		for agent, allowed := range agentLine {
			if strings.Contains(ap, agent) && line != allowed {
				violations = append(violations, fmt.Sprintf("agent=%s tool=%s", c.AgentPath, c.ToolName))
			}
		}
	}
	return []CheckResult{
		{Name: "replay_subagent_tool_isolation", Pass: len(violations) == 0, Summary: fmt.Sprintf("case-tool calls=%d violations=%d", caseToolCalls, len(violations)), Details: map[string]any{"violations": violations}},
		{Name: "replay_main_direct_tool_calls", Pass: true, Summary: fmt.Sprintf("main direct case-tool calls=%d (metric only; lower is better)", mainDirect)},
	}, nil
}

func walkJSON(v any, fn func(map[string]any)) {
	switch x := v.(type) {
	case map[string]any:
		fn(x)
		for _, child := range x {
			walkJSON(child, fn)
		}
	case []any:
		for _, child := range x {
			walkJSON(child, fn)
		}
	}
}

func loadCasePair(gtDir, casepackDir, caseID string) (GroundTruth, RuntimeCase, error) {
	var gt GroundTruth
	var rc RuntimeCase
	if err := readJSON(filepath.Join(gtDir, caseID+".json"), &gt); err != nil {
		return gt, rc, err
	}
	if err := readJSON(filepath.Join(casepackDir, caseID+".json"), &rc); err != nil {
		return gt, rc, err
	}
	return gt, rc, nil
}

func runtimeIDs(rc RuntimeCase) map[string]bool {
	ids := map[string]bool{}
	for _, x := range rc.Scene {
		ids[x.ID] = true
	}
	for _, x := range rc.NPCs {
		ids[x.ID] = true
	}
	for _, x := range rc.Archive {
		ids[x.ID] = true
	}
	return ids
}

func readJSON(path string, dst any) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(data, dst); err != nil {
		return fmt.Errorf("decode %s: %w", path, err)
	}
	return nil
}

func normalize(s string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(strings.TrimSpace(s)) {
		if unicode.IsSpace(r) || unicode.IsPunct(r) || unicode.IsSymbol(r) {
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

func printText(r Report) {
	if r.CaseID != "" {
		fmt.Printf("case: %s\n", r.CaseID)
	}
	for _, x := range r.Results {
		mark := "PASS"
		if !x.Pass {
			mark = "FAIL"
		}
		fmt.Printf("[%s] %s - %s\n", mark, x.Name, x.Summary)
	}
}
