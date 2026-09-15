package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type Client struct {
	BaseURL  string
	UserName string
	HTTP     *http.Client
	DryRun   bool
}

func env(name, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(name)); v != "" {
		return v
	}
	return fallback
}

func main() {
	baseURL := strings.TrimRight(env("BASE_URL", "http://127.0.0.1:8080/react-base-service"), "/")
	caseServerURL := strings.TrimRight(env("CASE_SERVER_URL", "http://127.0.0.1:18081"), "/")
	callerKey := env("CALLER_KEY", "detective-benchmark")
	userName := env("X_USER_NAME", "detective-admin")
	dryRun := os.Getenv("DRY_RUN") == "1"
	c := &Client{BaseURL: baseURL, UserName: userName, HTTP: &http.Client{Timeout: 20 * time.Second}, DryRun: dryRun}

	steps := []struct {
		name string
		path string
		body any
	}{
		{"caller", "/caller/register", map[string]any{
			"callerKey": callerKey, "name": "连载侦探社 Benchmark", "description": "长期记忆、多智能体委派与回放评测", "platform": "web",
		}},
		{"system-prompt", "/system-prompt/register", map[string]any{
			"callerKey": callerKey, "routeValues": []string{}, "name": "连载侦探社主侦探", "content": mainSystemPrompt,
		}},
	}

	if key := strings.TrimSpace(os.Getenv("LLM_API_KEY")); key != "" {
		steps = append(steps, struct {
			name string
			path string
			body any
		}{"apikey", "/apikey/register", map[string]any{
			"callerKey": callerKey, "routeValues": []string{}, "name": "detective-default", "apiKey": key,
		}})
	}

	steps = append(steps,
		struct {
			name string
			path string
			body any
		}{"tool:case_scene", "/tool/register", toolBody(callerKey, "case_scene", "读取当前案件的一条现场证据。只能查询当前活动案卷，不能指定历史 case_id。", caseServerURL+"/tool/scene", map[string]any{
			"type": "object", "properties": map[string]any{"evidence_id": map[string]any{"type": "string", "minLength": 1, "description": "现场证据ID，如 S1"}}, "required": []string{"evidence_id"}, "additionalProperties": false,
		})},
		struct {
			name string
			path string
			body any
		}{"tool:case_interview", "/tool/register", toolBody(callerKey, "case_interview", "读取当前案件一名 NPC 的完整证词。只能查询当前活动案卷。", caseServerURL+"/tool/interview", map[string]any{
			"type": "object", "properties": map[string]any{"npc_name": map[string]any{"type": "string", "minLength": 1, "description": "NPC 姓名或 P* 编号"}}, "required": []string{"npc_name"}, "additionalProperties": false,
		})},
		struct {
			name string
			path string
			body any
		}{"tool:case_archive", "/tool/register", toolBody(callerKey, "case_archive", "按关键词检索当前案件档案。历史案卷不会返回。", caseServerURL+"/tool/archive", map[string]any{
			"type": "object", "properties": map[string]any{"query": map[string]any{"type": "string", "minLength": 1, "description": "档案关键词，如姓名、编号、物件"}}, "required": []string{"query"}, "additionalProperties": false,
		})},
	)

	for _, step := range steps {
		if err := c.post(step.name, step.path, step.body); err != nil {
			fatal(err)
		}
	}

	for _, rel := range []string{"skills/cross-validation/SKILL.md", "skills/timeline-reconstruction/SKILL.md"} {
		markdown, err := os.ReadFile(rel)
		if err != nil {
			fatal(fmt.Errorf("read %s: %w (run bootstrap from project root)", rel, err))
		}
		if err := c.post("skill:"+filepath.Base(filepath.Dir(rel)), "/skill/import", map[string]any{
			"callerKey": callerKey, "routeValues": []string{}, "markdown": string(markdown),
		}); err != nil {
			fatal(err)
		}
	}

	agents := []map[string]any{
		agentBody(callerKey, "det-scene", "现场侦探", "适用：现场勘验、物证观察、现场痕迹解释。不适用：询问证人、查询档案、历史案件回忆。", mustRead("agents/det-scene.md"), []string{"case_scene"}),
		agentBody(callerKey, "det-people", "走访侦探", "适用：当前案件证人/嫌疑人口供收集与本线矛盾标记。不适用：现场勘验、档案检索、历史案件回忆。", mustRead("agents/det-people.md"), []string{"case_interview"}),
		agentBody(callerKey, "det-archive", "档案侦探", "适用：当前案件档案、记录、编号检索。不适用：现场勘验、询问证人、历史案卷检索。", mustRead("agents/det-archive.md"), []string{"case_archive"}),
	}
	for _, body := range agents {
		if err := c.post("agent:"+body["agentKey"].(string), "/agent/create", body); err != nil {
			fatal(err)
		}
	}

	fmt.Println("bootstrap complete")
	fmt.Printf("callerKey=%s\nbase=%s\ncaseServer=%s\n", callerKey, baseURL, caseServerURL)
	fmt.Println("NOTE: if a resource already exists, delete/update it in the base management API before re-running; this script stays strict so schema errors are not hidden.")
}

func toolBody(callerKey, name, description, url string, inputSchema map[string]any) map[string]any {
	return map[string]any{
		"name": name, "description": description, "toolType": "http", "callerKey": callerKey, "routeValues": []string{},
		"config": map[string]any{
			"url": url, "method": "post", "headers": map[string]string{"X-Detective-Tool": name}, "inputSchema": inputSchema,
			"outputSchema": map[string]any{"type": "object", "description": "Case Server JSON response; contains current case_id, line, and requested material."},
			"description":  description,
		},
	}
}

func agentBody(callerKey, key, name, description, systemPrompt string, tools []string) map[string]any {
	return map[string]any{
		"agentKey": key, "name": name, "description": description, "callerKey": callerKey, "routeValues": []string{},
		"systemPrompt": systemPrompt, "tools": tools, "skills": []string{}, "maxSteps": 6, "permissionMode": "inherit", "status": 1,
	}
}

func (c *Client) post(name, path string, body any) error {
	data, err := json.Marshal(body)
	if err != nil {
		return err
	}
	if c.DryRun {
		fmt.Printf("[DRY] %-28s POST %s%s\n", name, c.BaseURL, path)
		fmt.Println(string(data))
		return nil
	}
	req, err := http.NewRequest(http.MethodPost, c.BaseURL+path, bytes.NewReader(data))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-User-Name", c.UserName)
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return fmt.Errorf("%s: %w", name, err)
	}
	defer resp.Body.Close()
	respData, _ := io.ReadAll(io.LimitReader(resp.Body, 32<<10))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("%s: HTTP %d: %s", name, resp.StatusCode, strings.TrimSpace(string(respData)))
	}
	fmt.Printf("[OK]  %-28s HTTP %d\n", name, resp.StatusCode)
	return nil
}

func mustRead(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		fatal(fmt.Errorf("read %s: %w (run bootstrap from project root)", path, err))
	}
	return string(data)
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "bootstrap error:", err)
	os.Exit(1)
}

const mainSystemPrompt = `你是“连载侦探社”的主侦探。你的目标不是表演，而是基于当前案卷的可核验证据完成调查，并让运行轨迹可以被评测。

调查纪律：
1. 新案开始时，优先把调查拆成 scene / people / archive 三条线；在同一轮尽可能分别调用 delegate_agent 委派 det-scene、det-people、det-archive，使其并行执行。
2. 子侦探只应接收完成本线所需的任务描述，不要在派单中泄露另一条线刚得到的证词或证据。
3. 主侦探优先汇总子侦探报告；只有报告缺失或需要定点复核时才亲自调用 case_scene / case_interview / case_archive。
4. 任何结论都必须引用真实返回过的证据 id。不得编造未访问证据。
5. 遇到“证词矛盾/对不上”时使用交叉验证 Skill；遇到“时间线/几点”问题时使用时间线重建 Skill。
6. 历史案卷工具不可查。涉及历史案件事实时只能依赖已存在的长期记忆；不确定就明确说不确定，不要用当前案材料冒充历史事实。
7. 结案后，如果 memory 工具可用：把案件结论（凶手、手法、关键证据 id）写为按需长期记忆，并把真正高频的跨案暗线摘要保持为常驻层。严格区分“玩家猜测”和“已证实事实”；写入 reason 要说明证据来源。具体字段以 memory_write 当前工具 schema 为准。

结案输出契约：
- 先用自然语言给出交叉验证后的结案陈词。
- 最后必须单独输出一个 JSON 代码块，且只包含：
  {"case_id":"CASE-xxx","culprit":"姓名","method":"手法描述","evidence_refs":["S1","P1","A1"]}
- evidence_refs 至少覆盖 scene、people、archive 三条线；只填你在本次案件中实际获得过的 id。
`
