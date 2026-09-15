# CASE-011 ~ CASE-015 交付摘要

## 新案件

| Case | 标题 | 当前案独立结案 | 指定历史依赖 | 跨案价值 |
|---|---|---:|---|---|
| CASE-011 | 凌晨四点的空白借阅单 | 是 | CASE-003 | 0417 → 历史服务来源 |
| CASE-012 | 没有原稿的第五幅画 | 是 | CASE-005 | 防止把旧鸦羽/L.W.C. 错归因给当前凶手 |
| CASE-013 | 停运仓里的第四只温度探头 | 是 | CASE-008 | 跨机构重复共现，增强实体连接 |
| CASE-014 | 关机后的第九次登录 | 是 | CASE-009 | L.W.C. → 陆闻川，并确认当前仍活跃 |
| CASE-015 | 最后一次归档 | 否：完整命名需要历史 | 003/005/008/009 + 011~014 | 第一章 MASTER 收束 |

## 第一章 MASTER 真相边界

MASTER 是 **陆闻川**，网络名为 **栖鸦恢复网络**。

成立的事实：陆闻川以 L.W.C. 身份长期控制旧迁移项目遗留的恢复入口，并在当前时间继续通过有偿支持重新启用这些入口；CASE-015 中他本人为了夺取 MASTER-KEY、阻止秦术公开恢复入口总清单而杀害秦术。

不成立的过度推断：没有 ground truth 证明陆闻川指挥了 CASE-003/005/008/009 的本地凶手。那些单案的 culprit、motive、method 继续独立成立。

## 新增可判分结构

- `cross_case_dependencies`：声明历史 source refs、当前 refs 和高价值判断。
- `cross_case_quiz`：历史专属题；`historical_only: true` 时自检保证答案不在当前 casepack。
- `master_truth`：声明 mastermind、network、所需依赖和结论术语。
- Evaluator 新模式：`crossquiz`、`master`。

## 验收命令

```bash
python3 scripts/compile_cases.py
go test ./...
go run ./evaluator -mode selfcheck
bash scripts/smoke.sh

go run ./evaluator -mode crossquiz -case CASE-015 \
  -answers eval/templates/CASE-015.answers.json

go run ./evaluator -mode master -case CASE-015 \
  -master-verdict eval/templates/master_verdict.json
```
