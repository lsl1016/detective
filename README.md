# 连载侦探社：长期记忆与多智能体 Benchmark 骨架

这是一个可直接接到 `react-base-service` 的 M1/M2 起步工程。它把“游戏世界”和“上帝视角判分”分开：

- **Case Server** 只加载 `casepack/` 运行态材料，提供当前案卷的 `scene / people / archive` 三个 HTTP 工具。
- **Ground Truth** 单独放在 `eval/ground_truth/`，包含真相、暗线和记忆题，Case Server 从不读取它。
- **Evaluator CLI** 做 ground-truth 泄漏自检、结案判分、memory quiz 精确/别名判分、访问日志/回放的多 Agent 规约检查。
- **bootstrap.sh** 按基座接口注册 caller、主 system prompt、3 个 HTTP 工具、2 个 Skill、3 个子 Agent。

> 当前版本已经扩展到 **CASE-001 ~ CASE-015**。CASE-011~014 是第一组“跨案依赖型案件”：单案仍可独立结案，但高价值判断需要召回指定历史案；CASE-015 是第一章真正的 MASTER 案，并加入可机械判分的 `cross_case_dependencies / cross_case_quiz / master_truth`。

---

## 1. 目录

```text
detective/
├── README.md
├── go.mod
├── Makefile
├── .env.example
├── bootstrap.sh
├── conf/
│   └── custom.detective.yaml       # 合并到基座 custom.yaml 的配置片段
├── cases/                          # 人工维护：完整 YAML / ground truth 源
│   ├── CASE-001.yaml
│   ├── CASE-002.yaml
│   ├── CASE-003.yaml ... CASE-015.yaml
│   ├── CASE-DESIGN-NOTES.md       # 全部案件合理性/时间线 review
│   └── CHAPTER-1-MASTER-DESIGN.md # 第一章暗线与 MASTER 因果边界
├── casepack/                       # 生成物：Case Server 唯一读取目录
│   ├── CASE-001.json ... CASE-015.json  # 均无任何 ground-truth 字段
│   └── current.json
├── caseserver/
│   ├── main.go
│   └── main_test.go
├── agents/
│   ├── det-scene.md
│   ├── det-people.md
│   └── det-archive.md
├── skills/
│   ├── cross-validation/SKILL.md
│   └── timeline-reconstruction/SKILL.md
├── cmd/bootstrap/main.go           # bootstrap.sh 的 Go 实现
├── evaluator/main.go
├── eval/
│   ├── ground_truth/               # 生成物：Evaluator 读取
│   ├── scorecard.md
│   ├── templates/
│   └── runs/                       # 运行日志/回放/答案，默认 git ignore
└── scripts/
    ├── compile_cases.py
    ├── smoke.sh
    └── switch_case.sh
```

### 数据边界

```text
cases/CASE-xxx.yaml
        │
        │ compile_cases.py
        ├──────────────────────────────┐
        ▼                              ▼
casepack/CASE-xxx.json          eval/ground_truth/CASE-xxx.json
仅可玩材料                       truth / meta_plot / memory_quiz
                                  cross_case_* / master_truth
        │                              │
        ▼                              ▼
   Case Server                      Evaluator
        │                              │
        └──────── Agent ────────────────┘
                    X
           Agent 永远不应接触右侧
```

`casepack` 编译时明确剥离：

```text
truth
meta_plot
memory_quiz
cross_case_dependencies
cross_case_quiz
master_truth
evaluation_targets
```

Case Server 启动时还会二次检查；如果运行态 JSON 出现这些字段，直接拒绝启动。

---

## 2. 当前案件集

| Case | 标题 | 主要测试点 |
|---|---|---|
| CASE-001 | 雨夜剧院的空座位 | 基础三线委派 / 时间线冲突 |
| CASE-002 | 停在四层的电梯 | 伪证 vs 机器记录 |
| CASE-003 | 打烊后的第三只茶杯 | 返回现场 / 权限 + 车辆 + 终端交叉 |
| CASE-004 | 被提前十分钟的末班车 | 假不在场 / 多时间源可靠性 |
| CASE-005 | 封存库里的旧胶片 | 物理调包 / 历史数字痕迹 |
| CASE-006 | 没有响过的消防铃 | 系统隔离 / 真实故障被利用 |
| CASE-007 | 凌晨两点的药柜 | 工牌与 PIN 身份拆分 / 嫁祸 |
| CASE-008 | 桥下失踪的蓝伞 | 现场转移 / GPS 与土壤交叉 |
| CASE-009 | 停电前打印的遗嘱 | 文件 provenance / 储能时间线 |
| CASE-010 | 不存在的第七码头箱 | 幽灵记录 / 物理-数字双重核验 |
| CASE-011 | 凌晨四点的空白借阅单 | 时间回填 / 依赖 CASE-003 解析 0417 来源 |
| CASE-012 | 没有原稿的第五幅画 | provenance 分层 / 依赖 CASE-005 防止错误归因 |
| CASE-013 | 停运仓里的第四只温度探头 | 探头身份映射 / 依赖 CASE-008 做跨机构实体确认 |
| CASE-014 | 关机后的第九次登录 | 旧账号 override / 依赖 CASE-009 完成 L.W.C. 身份解析 |
| CASE-015 | 最后一次归档 | 第一章 MASTER / 多跳跨案记忆 + 因果边界 |

CASE-011~014 每案都保持 scene / people / archive 三线证据闭环，且 `truth.culprit` 不依赖历史材料；历史记忆只影响更高层的来源、身份和因果判断。CASE-015 则有意把“完整命名凶手/MASTER”设计成跨案硬依赖。

案件合理性与排除链条见 `cases/CASE-DESIGN-NOTES.md`；第一章 MASTER 的完整因果边界见 `cases/CHAPTER-1-MASTER-DESIGN.md`。关键原则：**陆闻川是栖鸦恢复网络的持续组织者，不等于他指挥了此前所有单案凶杀。**

---

## 3. 本地要求

- Go 1.22+
- Python 3（**只有修改 YAML 后重新编译 casepack 时需要**）
- PyYAML 6.x
- curl（smoke test / 管理 Case Server）

安装开发依赖：

```bash
python3 -m pip install -r requirements-dev.txt
```

重新生成运行态与 ground truth：

```bash
python3 scripts/compile_cases.py
```

普通运行 Case Server / Evaluator 不依赖 Python 包。

---

## 4. 先跑本地自检

```bash
go test ./...
go run ./evaluator -mode selfcheck
bash scripts/smoke.sh
```

预期：全部 PASS。

`smoke.sh` 会实际启动一个临时 Case Server，并检查：

1. `CASE-001` 是当前案；
2. 当前案现场工具可查；
3. `/case/CASE-002` 这种历史案路径不存在；
4. 工具 Body 偷塞 `case_id` 会因为严格 JSON schema 被 400 拒绝；
5. 管理端可以切到 `CASE-002`；
6. 切案后第一案 NPC 立即不可查；
7. Evaluator 的泄漏/引用/题库自检通过。

---

## 5. 启动 Case Server

最小：

```bash
go run ./caseserver
```

默认：

```text
listen:       :18081
casepack:     ./casepack
current:      ./casepack/current.json
access log:   ./eval/runs/access.jsonl
```

建议显式设置管理 token：

```bash
export DETECTIVE_ADMIN_TOKEN=dev-admin
go run ./caseserver
```

检查：

```bash
curl http://127.0.0.1:18081/healthz
curl http://127.0.0.1:18081/case/current
```

### 三个业务工具端点

```bash
curl -X POST http://127.0.0.1:18081/tool/scene \
  -H 'Content-Type: application/json' \
  -d '{"evidence_id":"S1"}'

curl -X POST http://127.0.0.1:18081/tool/interview \
  -H 'Content-Type: application/json' \
  -d '{"npc_name":"许岚"}'

curl -X POST http://127.0.0.1:18081/tool/archive \
  -H 'Content-Type: application/json' \
  -d '{"query":"K-17"}'
```

工具**没有** `case_id` 入参，这是故意的。服务永远从 `current.json` 选择当前案，历史案不存在公开读取路由。

### 切换案件

```bash
export DETECTIVE_ADMIN_TOKEN=dev-admin
bash scripts/switch_case.sh CASE-002 --reset-log
```

或：

```bash
curl -X POST http://127.0.0.1:18081/admin/current \
  -H 'Content-Type: application/json' \
  -H 'X-Admin-Token: dev-admin' \
  -d '{"case_id":"CASE-002","reset_log":true}'
```

新一轮正式实验建议 `reset_log=true`。

---

## 6. 配置 react-base-service

先确保基座自己的 `sql/init.sql` 已包含并执行：

- `tblLlmAgent`
- `tblLlmMemoryItem`
- `tblLlmMemoryRevision`

然后把 `conf/custom.detective.yaml` 合并到基座的 `conf/mount/custom.yaml`。

核心开关：

```yaml
llm:
  react:
    memory:
      enabled: true
      allow_user_scope: true
      reflection:
        enabled: true
    subagent:
      enabled: true
      max_parallel: 3
      max_depth: 2
    workspace:
      enabled: false
```

### 为什么 benchmark 时必须关 workspace

这个仓库本身包含 `cases/*.yaml` 和 `eval/ground_truth/*.json`。如果给被测 Agent `load_runtime_code` 或等价代码工作区读取能力，它可能直接读答案，整个评测失效。

生产化部署时更推荐**物理拆容器**：

```text
case-server container: 只挂载 casepack/
evaluator runner:      挂载 eval/ground_truth/
react-base-service:    两边 ground truth 都不挂载
```

---

## 7. 一键注册基座资源

从项目根目录：

```bash
export BASE_URL=http://127.0.0.1:8080/react-base-service
export CASE_SERVER_URL=http://127.0.0.1:18081
export CALLER_KEY=detective-benchmark
export X_USER_NAME=detective-admin

# 只有这个 caller 还没模型凭证时再填：
# export LLM_API_KEY=xxx

bash bootstrap.sh
```

先看 payload、不真正请求：

```bash
DRY_RUN=1 bash bootstrap.sh
```

它会注册：

```text
Caller
  detective-benchmark

System Prompt
  连载侦探社主侦探

Tools
  case_scene      -> POST /tool/scene
  case_interview  -> POST /tool/interview
  case_archive    -> POST /tool/archive

Agents
  det-scene       tools=[case_scene]
  det-people      tools=[case_interview]
  det-archive     tools=[case_archive]

Skills
  detective-cross-validation
  detective-timeline-reconstruction
```

### bootstrap 的一个刻意选择

脚本目前是**严格注册**，已有同名资源时不会擅自猜测“create 是否等价于 update”。这样接口字段变化会直接暴露，不会被“忽略错误继续跑”掩盖。

如果你的环境已注册过同名资源，先通过基座管理面删除/更新，或后续按你们真实 `list/detail/update` 返回结构再把 bootstrap 改成 upsert。

---

## 8. 子 Agent 隔离

三个 Agent 的业务工具白名单分别只有一条：

```text
det-scene    -> case_scene
det-people   -> case_interview
det-archive  -> case_archive
```

Prompt 还额外要求：

- 不转述非本线事实；
- 不把主 Agent 泄露过来的其他线材料当作可验证事实；
- 不读写长期记忆；
- 只返回本线 `evidence / open_questions`。

### 当前基座接口下仍有一个“严格隔离缺口”

业务工具白名单是硬隔离，但基座文档对**内置工具**说明为“按配置另计”。如果 `memory_*` 自动出现在子 run，当前 Agent 注册接口没有暴露“每个子 Agent 禁用 memory 注入/工具”的独立字段。

因此当前骨架做到的是：

```text
业务工具：硬隔离
子会话：基座负责独立上下文
memory：Prompt 纪律隔离（不是安全边界）
```

要把 E4 做成真正的信息安全级硬判据，建议基座补一个类似：

```text
agent.memoryPolicy = none
# 或
subagent.inheritMemory = false
```

在这个能力出现前，不要把“子 Agent 没调用 memory 工具”误当成“它绝对没被注入任何历史记忆”。

---

## 9. 开始一局

先拿当前简报：

```bash
curl http://127.0.0.1:18081/case/current
```

然后在现有 React Web SDK / Playground 中发起新 session，callerKey 使用：

```text
detective-benchmark
```

建议首轮用户输入包含：

```text
开始调查 CASE-001。请按侦探社规程并行派出现场、走访、档案三名子侦探。
```

并把 `/case/current` 返回的 `case_id/title/briefing` 放入这一轮用户消息或 `llmContext`。

如果直接按 WebSocket 协议发 `run`，结构示意：

```json
{
  "type": "run",
  "payload": {
    "callerKey": "detective-benchmark",
    "type": "chat",
    "userPrompt": "开始调查 CASE-001。请并行派出三名子侦探。",
    "llmContext": {
      "case_id": "CASE-001",
      "title": "雨夜剧院的空座位",
      "briefing": "把 /case/current 返回的 briefing 放这里"
    }
  }
}
```

主 system prompt 已要求结案最后输出：

```json
{
  "case_id": "CASE-001",
  "culprit": "...",
  "method": "...",
  "evidence_refs": ["S1", "P1", "A1"]
}
```

把这个 JSON 保存为：

```text
eval/runs/CASE-001.verdict.json
```

---

## 10. Evaluator CLI

### 10.1 Ground-truth / casepack 自检

```bash
go run ./evaluator -mode selfcheck
```

检查：

- 运行态没有答案字段泄漏；
- `truth.key_evidence` 都能解析到真实材料 id；
- `memory_quiz` 标准答案确实出现在本案运行材料中；
- `cross_case_quiz` 标准答案确实出现在声明的历史 source case 中；
- 对 `historical_only: true` 的跨案题，accepted answer 不得出现在当前运行态案卷；
- `cross_case_dependencies` 的 source/current refs 必须全部存在；
- `current.json` 指向真实 casepack。

### 10.2 结案判分

```bash
go run ./evaluator \
  -mode case \
  -case CASE-001 \
  -verdict eval/runs/CASE-001.verdict.json
```

机械检查：

- case_id 正确；
- culprit 精确匹配；
- method 包含脚本规定的关键术语；
- `truth.key_evidence` 是否全部覆盖；
- 是否引用不存在的证据 id；
- evidence_refs 是否覆盖 scene / people / archive 三条线。

### 10.3 Memory Quiz

复制模板：

```bash
cp eval/templates/answers.json eval/runs/CASE-001.answers.json
```

填入回答后：

```bash
go run ./evaluator \
  -mode quiz \
  -case CASE-001 \
  -answers eval/runs/CASE-001.answers.json
```

跨案依赖题（CASE-011+）：

```bash
go run ./evaluator -mode crossquiz -case CASE-015 \
  -answers eval/templates/CASE-015.answers.json
```

MASTER 判分：

```bash
go run ./evaluator -mode master -case CASE-015 \
  -master-verdict eval/templates/master_verdict.json
```

判分会做：

```text
trim
大小写归一
空白/标点归一
canonical + aliases 精确匹配
```

这部分不需要裁判模型。

> 真正测长期记忆时，不是在 CASE-001 刚结束就问 Q1，而是在切到 CASE-002/003/... 后，从历史案题库抽题。此时 Case Server 已经下架 CASE-001，工具无法重新取证。

### 10.4 Case Server 访问日志

```bash
go run ./evaluator \
  -mode access \
  -case CASE-001 \
  -access-log eval/runs/access.jsonl
```

Case Server 会记录：

```text
time
case_id
line
selector
status
X-Session-Id
X-User-Name
X-Run-Id             # 若基座转发
X-Agent-Path         # 若基座转发
X-Tool-Use-Id        # 若基座转发
```

基座文档明确保证运行时上下文头里至少会透传 `X-Session-Id / X-User-Name`，但没有明确承诺 `agentPath` 会作为 HTTP header 透传，所以**访问日志里的 Agent 归属是 best-effort**。

### 10.5 回放 E4 检查

基座事件本身有 `agentPath`，因此更可靠的 Agent 归属证据来自 `/react/session/events` 导出的回放 JSON。

保存为：

```text
eval/runs/CASE-001.replay.json
```

然后：

```bash
go run ./evaluator \
  -mode replay \
  -replay eval/runs/CASE-001.replay.json
```

Evaluator 会递归找 `tool_use_start` 事件，检查：

```text
det-scene   只能调用 case_scene
det-people  只能调用 case_interview
det-archive 只能调用 case_archive
```

同时统计主 run 亲自调用案件工具的次数。

> 若要做到“访问日志 + replay 100% 自动交叉核对”，下一步建议让基座 HTTP 工具执行器额外透传 `X-Agent-Path / X-Run-Id / X-Tool-Use-Id`。Case Server 已经预留这些字段，无需再改协议。

### 10.6 一次跑完

```bash
go run ./evaluator -mode all -case CASE-001 \
  -verdict eval/runs/CASE-001.verdict.json \
  -answers eval/runs/CASE-001.answers.json \
  -access-log eval/runs/access.jsonl \
  -replay eval/runs/CASE-001.replay.json \
  -out eval/runs/CASE-001.report.json
```

---

## 11. Access Log 示例

每个案件工具请求会追加一行 JSONL：

```json
{
  "time": "2026-09-14T14:31:02.123Z",
  "case_id": "CASE-001",
  "line": "scene",
  "selector": "S1",
  "status": 200,
  "session_id": "session_xxx",
  "user_name": "keke",
  "agent_path": "main/det-scene"
}
```

失败调用也记录，因此“试图越线后被 404”不会从审计里消失。

---

## 12. 写新案件

复制一份：

```bash
cp cases/CASE-002.yaml cases/CASE-003.yaml
```

需要保留的核心结构：

```yaml
case_id: CASE-003
title: ...
briefing: ...
scene: ...
npcs: ...
archive: ...
truth:
  culprit: ...
  method: ...
  key_evidence: [S1, P2, A1]
  method_required_terms: [...]
meta_plot:
  - hint_id: M3-1
    content: ...
    source_refs: [S3]
    truth_link: MASTER
memory_quiz:
  - id: Q3-1
    q: ...
    a: ...
    aliases: [...]

# CASE-011+ 可选：
cross_case_dependencies:
  - id: D11-003
    source_case: CASE-003
    source_refs: [S4]
    current_refs: [S6]
    historical_fact: ...
    high_value_judgment: ...
    required_for_single_case: false
    required_for_master: true
cross_case_quiz:
  - id: XQ11-1
    q: ...
    a: ...
    aliases: [...]
    source_case: CASE-003
    source_refs: [S4]
    current_refs: [S6]
    historical_only: true

# MASTER 案可选：
master_truth:
  mastermind: ...
  network_name: ...
  thesis: ...
  required_dependency_ids: [...]
  required_terms: [...]

evaluation_targets: ...
```

然后：

```bash
python3 scripts/compile_cases.py
go run ./evaluator -mode selfcheck
go test ./...
```

编译器会阻止：

- 缺必需字段；
- scene/NPC/archive id 冲突；
- `keywords` / `aliases` / evidence refs 等本应为字符串的列表被 YAML 自动解析成数字；
- `truth.key_evidence` / `supporting_evidence` 指向不存在材料；
- `truth.key_evidence` 没有同时覆盖 scene / people / archive 三条线；
- memory quiz 缺 id/q/a；
- meta-plot 没有 `source_refs`，或暗线引用了不存在的材料 id。

Evaluator 还会检查 quiz 答案是否真的存在于运行态案卷中。

---

## 13. M1 建议验收顺序

### Round 0：不接 LLM

```text
compile_cases
  ↓
Case Server
  ↓
smoke
  ↓
Evaluator selfcheck
```

先证明评测道具自己没有泄漏和历史案旁路。

### Round 1：只测 delegate

临时先不关注跨案记忆分数：

```text
CASE-001
  ↓
3 × delegate_agent
  ↓
泳道并行
  ↓
三线汇总
  ↓
verdict evaluator
  ↓
replay E4
```

### Round 2：开始测记忆

结案后切：

```bash
bash scripts/switch_case.sh CASE-002 --reset-log
```

新 session / 同 caller_user，开始 CASE-002；结案后再问 CASE-001 的 Q1-*。

此时：

```text
CASE-001 已经无法通过 Case Server 重取
```

所以命中只能来自长期记忆或模型偶然先验猜测；通过多题、随机案件事实可把偶然猜中概率压低。

---

## 14. 与 memory / reflection 的衔接

本骨架没有硬编码 `memory_write` 的具体入参，因为这属于基座内置工具，应该以运行时实际 schema 为准；主 system prompt 只规定“写什么”和“为什么写”。

建议结案写入语义：

```text
案件结论：detached
  tags = case, CASE-xxx
  内容 = culprit / method / key evidence ids

跨案已确认暗线摘要：resident
  只保留高频、稳定、已验证事实

玩家偏好：caller_user

玩家强制锁定事实：locked
```

之后 E3 直接查基座的 memory item/revision 表。

Reflection 的第一版建议先人工审计：

```text
compact_end
  ↓
source=reflection revision
  ↓
对照被压缩对话
  ↓
漏 / 脏 / 冗余 / 成功捞回
```

---

## 15. 当前骨架没有假装解决的事情

1. **没有自动算 E2 历史事实矛盾**：M1 仍按 scorecard 人工读回放；M3 再做裁判模型/规则抽取。
2. **没有自动查 `tblLlmMemory*`**：不同环境数据库连接方式未知，避免把内部 DB 假设硬写进 benchmark；E3 先沿用你已有 SQL。
3. **没有伪造 `/react/session/events` 请求 Body**：接入文档给了接口路径和事件结构，但没有给 list/events 的完整请求 schema；因此 CLI 接受“已经导出的 replay JSON”。
4. **子 Agent memory 隔离目前不是硬边界**：见 §8。
5. **bootstrap 不是无脑 upsert**：避免把资源重复/接口变更吞掉。

这些都保留成明确扩展点，而不是在骨架里假装已经自动化。

---

## 16. 下一阶段可直接加什么

这个目录可以原地继续做到 M2/M3：

```text
CASE-016+ 第二章暗线
跨案 quiz sampler
Retention Curve (+1/+3/+5/+10 案)
Memory Revision / Correction Rate
E3 DB auditor
replay + access-log 精确关联
并行 overlap / speedup 统计
Graph Memory 人物关系题
judge model 半自动 E2
```

建议先用 CASE-001/002 做基座接入冒烟，再用 CASE-003~010 做真正的长上下文、长期记忆和跨案暗线测试。后续新增 MASTER 终章前，应先固定陆闻川/栖鸦线的真实历史因果，不要让“重复出现的标识”直接等同于“幕后凶手”。
