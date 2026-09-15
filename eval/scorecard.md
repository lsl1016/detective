# 连载侦探社单案评分表

- Case: `CASE-___`
- 玩家 / caller_user:
- 配置组: `A / B / C / D / E`
- sessionId:
- 日期:

## E1 跨案记忆

- 题目来源案件:
- 命中: `__/__`
- 错题:
- 错题归因：`记忆不存在 / 层级未召回 / 条目错误 / 回答未使用已存在记忆 / 其他`

## E2 连续性矛盾

- 矛盾总数:
- 明细：
  - 历史事实冲突：
  - 已退场人物错误出现：
  - 重复询问已完整取得证词：
  - 其他：

## E3 写入质量

- 漏记:
- 脏记:
- 冗余:
- 错误事实被 update/supersede 而非并存:
- reflection revision 数:
- reflection 实际捞回事实:
- provenance 是否能回指 run/step/evidence:

## E4 多智能体

- 子侦探越权: `0 / ...`
- 三线是否都有子 run:
- 三线时间窗是否重叠:
- Parallel Overlap Ratio:
- 主 run 直接查证次数:
- 最终 evidence_refs 是否覆盖 scene / people / archive:
- 引用 evidence id 是否都能在回放/访问日志中找到:

## 自动评测命令

```bash
go run ./evaluator -mode all -case CASE-001 \
  -verdict eval/runs/CASE-001.verdict.json \
  -answers eval/runs/CASE-001.answers.json \
  -access-log eval/runs/access.jsonl \
  -replay eval/runs/CASE-001.replay.json \
  -out eval/runs/CASE-001.report.json
```

## 备注


## E1-X 跨案依赖题（CASE-011+）

- cross_case_quiz 命中: `__/__`
- declared source case 是否已下架:
- 当前案是否泄漏 accepted answer: `必须为否`
- 错题归因：`历史记忆缺失 / 实体未连接 / 记住但来源混淆 / 过度因果合并`

## MASTER（CASE-015）

- mastermind:
- network:
- required_dependency_ids 覆盖: `__/8`
- 是否正确使用 CASE-003/005/008/009 历史锚点:
- 是否正确使用 CASE-011~014 现代桥接证据:
- Scope Guard：是否错误声称陆闻川指挥此前全部单案凶杀: `0 次为合格`

```bash
go run ./evaluator -mode crossquiz -case CASE-015 \
  -answers eval/templates/CASE-015.answers.json

go run ./evaluator -mode master -case CASE-015 \
  -master-verdict eval/templates/master_verdict.json
```
