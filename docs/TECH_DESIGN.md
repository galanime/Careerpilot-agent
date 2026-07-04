# CareerPilot Agent 技术设计

## 1. 设计目标

CareerPilot Agent 的技术目标是用 Go 实现一个轻量、可测试、可替换组件的 Agent Runtime。MVP 不依赖 LangChain、LangGraph 或外部数据库，优先展示底层 Agent 编排能力。

核心目标：

- Agent 执行过程可拆解、可观测、可复现。
- 工具调用通过统一接口注册和执行。
- 材料生成必须基于 evidence，而不是自由编造。
- 后续可以平滑接入真实 LLM Provider、SQLite、向量检索和前端 Trace UI。

## 2. 架构概览

```text
HTTP API
  |
  v
Agent Runtime
  |
  +-- Planner
  +-- Tool Registry
  |     +-- parse_jd
  |     +-- search_evidence
  |     +-- score_match
  |     +-- generate_materials
  |     +-- evaluate_output
  |
  +-- Memory Store
  +-- SQLite Run Store
  +-- LLM Provider
  |     +-- Fake
  |     +-- Anthropic / Claude
  |     +-- OpenAI-compatible
  |     +-- Ollama
  +-- Artifact Builder
```

## 3. 领域模型

### Run

Run 表示一次完整 Agent 执行。包含输入、状态、步骤和产物。

关键字段：

- `id`
- `input`
- `status`
- `steps`
- `artifacts`
- `created_at`
- `updated_at`

### Step

Step 表示 Agent 执行中的一个可观测节点。

关键字段：

- `id`
- `run_id`
- `type`
- `name`
- `status`
- `input`
- `output`
- `error`
- `started_at`
- `ended_at`

### Artifact

Artifact 是用户真正关心的产物，例如岗位分析、匹配报告、投递材料和自检报告。

## 4. Agent 执行计划

MVP 使用确定性计划：

```text
planner
  -> parse_jd
  -> search_evidence
  -> score_match
  -> generate_materials
  -> evaluate_output
```

这样做的原因是：

- 便于测试和演示。
- 降低早期不确定性。
- 面试中更容易讲清楚每一步的输入输出。

后续可以升级为 LLM Planner：让模型根据 JD 和用户偏好选择工具，但仍必须通过 Tool Registry 执行。

## 5. Tool Registry

工具接口：

```go
type Tool interface {
    Name() string
    Description() string
    Execute(ctx context.Context, input json.RawMessage) (json.RawMessage, error)
}
```

Registry 负责：

- 工具注册。
- 工具查找。
- 输入序列化。
- 执行工具并返回原始 JSON。

这个设计的面试价值是：

> LLM 不直接控制系统行为，只能选择预注册工具；业务逻辑和安全边界由后端工具实现。

## 6. Memory Store

MVP 使用本地 `data/evidence/projects.yaml`。每个 evidence item 包含：

- project
- title
- tags
- details

检索方式：

1. 从 JD 中提取 keywords。
2. 拼接 evidence 的 project、title、tags、details。
3. 用大小写不敏感的关键词匹配打分。
4. 返回 Top N evidence。

后续升级路径：

- SQLite FTS。
- PostgreSQL + pgvector。
- Qdrant。
- 混合检索：关键词 + embedding。

## 7. LLM Provider

Provider 接口统一为：

```go
type Provider interface {
    Generate(ctx context.Context, req GenerateRequest) (*GenerateResponse, error)
}
```

当前支持四种实现：

- `fake`：测试和无密钥本地演示。
- `anthropic` / `claude`：通过官方 Anthropic Go SDK 调用 Claude，默认模型 `claude-opus-4-7`，启用 adaptive thinking。
- `openai`：通过 OpenAI-compatible `/chat/completions` 接口调用，可接 DeepSeek、Qwen、自建网关等。
- `ollama`：通过本地 `/api/generate` 调用 Ollama。

`generate_materials` 工具会优先调用 Provider 生成 JSON 材料；如果 Provider 不可用、调用失败或 JSON 解析失败，会回退到模板生成，保证 Demo 稳定可运行。

## 8. SQLite 持久化

`storage.RunStore` 抽象屏蔽内存存储和 SQLite 存储：

```go
type RunStore interface {
    SaveRun(run domain.Run) error
    GetRun(id string) (domain.Run, error)
    ListRuns() ([]domain.Run, error)
}
```

SQLite 表：

- `runs`：保存 Run 输入、状态和时间。
- `steps`：保存每一步工具调用的输入、输出、状态和错误。
- `artifacts`：保存岗位分析、匹配报告、投递材料和自检报告。

默认数据库路径为 `careerpilot.db`，可通过 `CAREERPILOT_DB_PATH` 修改。

## 9. EventHub 与实时 Trace

`agent.EventHub` 是当前 MVP 的进程内事件总线。Runtime 在以下时机发布事件：

- Run 创建、开始、完成、失败。
- Step 开始、完成、失败。
- Artifact 创建。

EventHub 会保存每个 Run 的内存历史，并支持订阅：

```go
func (h *EventHub) Subscribe(ctx context.Context, runID string) <-chan Event
```

SSE 端点订阅该 channel，先回放历史事件，再推送实时事件。当前事件历史是进程内状态，服务重启后仍可通过 SQLite 查询 Run/Step/Artifact，但 SSE 历史不会恢复；后续可以把事件也持久化为 `run_events` 表。

## 10. Evaluator

当前 Evaluator 检查：

- JD 关键词覆盖率。
- 是否缺少 evidence。
- 是否出现“精通 / expert”等高风险夸大表达。

后续可以扩展为：

- 无证据主张检测。
- 项目证据引用完整性。
- 简历 bullet STAR 结构评分。
- 面试回答可追问性评分。
- Golden JD cases 回归测试。

## 11. API 设计

### `POST /api/runs`

创建 Run 后立即返回 `202 Accepted`，后台 goroutine 继续执行 Agent。

### `GET /api/runs`

列出所有 Run。

### `GET /api/runs/{run_id}`

查看 Run 详情。

### `GET /api/runs/{run_id}/artifacts`

查看产物。

### `GET /api/runs/{run_id}/events`

以 SSE 格式回放内存事件历史，并持续推送实时事件：

- `run_created`
- `run_started`
- `step_started`
- `step_finished`
- `step_failed`
- `artifact_created`
- `run_finished`
- `run_failed`

`run_finished` 和 `run_failed` 是终止事件，服务端发送后结束连接。

## 12. 当前限制

- Agent 执行为后台 goroutine，尚未接入任务队列、重试调度或进程外 worker。
- EventHub 历史是内存态，服务重启后 SSE 历史不会恢复。
- Evidence parser 是针对当前 YAML 子集的轻量解析器，不是通用 YAML 解析器。
- `generate_materials` 仍要求模型输出 JSON，后续应升级为结构化输出或更严格的 schema 校验。
- SQLite 当前持久化 runs、steps、artifacts，尚未单独建 `tool_calls` 或 `run_events` 表。
- 前端尚未实现完整 Trace UI。

## 13. 迭代计划

### 迭代 1：前端作品集展示

- Run 创建表单。
- Trace Timeline。
- Artifact Tabs。
- Eval Dashboard。

### 迭代 2：评测增强

- Golden JD cases。
- Golden trace tests。
- 无证据主张检测。
- 简历 bullet STAR 结构评分。

### 迭代 3：求职场景增强

- 多版本简历 bullet。
- 项目讲解稿生成。
- 面试追问树。
- Markdown 导出。
