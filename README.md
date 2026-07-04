# CareerPilot Agent

CareerPilot Agent 是一个用于投递 AI Agent / 大模型应用 / 后端 AI Infra 岗位的作品集项目。它用 Go 实现一个轻量 Agent Runtime，围绕“岗位 JD 分析 → 候选人证据检索 → 匹配评分 → 简历与面试材料生成 → 自检评估”的闭环流程，展示 Agent 编排、工具调用、可观测 Trace、证据约束生成和工程化测试能力。

## 项目定位

这个项目不是简单的 ChatGPT 包装，而是一个可解释的 Agent 系统：

- Planner 生成固定执行计划。
- Tool Registry 约束 Agent 可调用的工具。
- Memory Store 从本地 evidence 中检索候选人真实项目证据。
- Evaluator 检查输出关键词覆盖和无证据风险。
- API 暴露 Run、Step、Artifact 和 SSE Trace。

适合在简历中包装为：

> 基于 Go 实现证据驱动型 AI Agent Runtime，支持 JD 解析、项目证据检索、岗位匹配评分、投递材料生成和自检评估，并通过 HTTP API/SSE 输出完整执行 Trace。

## 当前 MVP 能力

- Go 标准库 HTTP API。
- SQLite Run Store，持久化 runs、steps、artifacts。
- 本地 YAML 风格 evidence 检索。
- 工具链：`parse_jd`、`search_evidence`、`score_match`、`generate_materials`、`evaluate_output`。
- LLM Provider：Fake、Anthropic/Claude、OpenAI-compatible、Ollama。
- 示例 JD 和候选人项目 evidence。
- 后端单元测试。
- React Trace UI：Run 创建表单、最近 Run 列表、Trace Timeline、Artifact Tabs、Eval Dashboard。
- 后台异步 Run 执行，`POST /api/runs` 立即返回 `202 Accepted`。
- 实时 SSE Trace，推送 run、step、artifact 事件。

## 快速启动

环境要求：

- Go 1.24+（本机已安装便携版到 `D:\Data\go-sdk\go`）
- 可选：Node.js 20+，用于后续前端 Trace UI

```powershell
cd careerpilot-agent
go test ./...
go run ./cmd/server
```

前端 Trace UI：

```powershell
cd careerpilot-agent\web
npm.cmd install
npm.cmd run dev
```

前端默认监听：

```text
http://127.0.0.1:5174
```

可选配置：

```powershell
$env:CAREERPILOT_DB_PATH = "careerpilot.db"
$env:CAREERPILOT_LLM_PROVIDER = "fake"       # fake | anthropic | openai | ollama
$env:CAREERPILOT_LLM_MODEL = "claude-opus-4-7"
$env:CAREERPILOT_LLM_API_KEY = "..."         # anthropic/openai 需要
$env:CAREERPILOT_LLM_BASE_URL = "..."        # openai-compatible/ollama 可用
```

Provider 示例：

```powershell
# Claude / Anthropic
$env:CAREERPILOT_LLM_PROVIDER = "anthropic"
$env:ANTHROPIC_API_KEY = "sk-ant-..."
$env:CAREERPILOT_LLM_MODEL = "claude-opus-4-7"

# OpenAI-compatible，例如 DeepSeek/Qwen/自建网关
$env:CAREERPILOT_LLM_PROVIDER = "openai"
$env:CAREERPILOT_LLM_API_KEY = "..."
$env:CAREERPILOT_LLM_BASE_URL = "https://api.openai.com/v1"
$env:CAREERPILOT_LLM_MODEL = "gpt-4o-mini"

# Ollama 本地模型
$env:CAREERPILOT_LLM_PROVIDER = "ollama"
$env:CAREERPILOT_LLM_BASE_URL = "http://127.0.0.1:11434"
$env:CAREERPILOT_LLM_MODEL = "llama3.1"
```

服务默认监听：

```text
http://127.0.0.1:8788
```

健康检查：

```powershell
Invoke-RestMethod http://127.0.0.1:8788/healthz
```

创建一次 Agent Run（返回 `202 Accepted`，后台继续执行）：

```powershell
$body = @{
  company_name = "ByteDance"
  job_title = "AI Agent Engineer Intern"
  target_role = "AI Agent Engineer"
  jd_text = "负责 AI Agent 平台开发，要求 Go、RAG、Tool Calling、Evaluation、后端工程和工作流编排能力。"
} | ConvertTo-Json
Invoke-RestMethod -Method Post -Uri http://127.0.0.1:8788/api/runs -Body $body -ContentType "application/json"
```

## API

### 创建 Run

```http
POST /api/runs
```

返回 `202 Accepted` 和初始 `created` 状态的 Run，后台 goroutine 会继续执行 Agent。

### 列出 Run

```http
GET /api/runs
```

### 获取 Run 详情

```http
GET /api/runs/{run_id}
```

### 获取产物

```http
GET /api/runs/{run_id}/artifacts
```

### 获取事件流

```http
GET /api/runs/{run_id}/events
```

SSE 会先回放当前内存中的历史事件，再继续推送实时事件。当前事件类型：

- `run_created`
- `run_started`
- `step_started`
- `step_finished`
- `step_failed`
- `artifact_created`
- `run_finished`
- `run_failed`

命令行观察：

```powershell
curl.exe -N http://127.0.0.1:8788/api/runs/<run_id>/events
```

`run_finished` 或 `run_failed` 是终止事件，服务端会结束该 SSE 响应。

## 前端 Trace UI

前端位于 `web/`，提供一次投递材料生成 Run 的可视化控制台：

- 输入公司、岗位、目标方向和 JD 文本并创建 Run。
- 查看最近 Run 列表并回放已完成结果。
- 通过 SSE 实时展示 Planner、工具调用和 Artifact 创建事件。
- 展示 JD 结构化分析、岗位匹配报告、投递材料草稿和自检评估报告。
- 用 Eval Dashboard 汇总匹配分、关键词覆盖率和无证据风险。

## 目录结构

```text
careerpilot-agent/
  cmd/server/             # HTTP 服务入口
  internal/api/           # API 路由和 handler
  internal/agent/         # Agent Runtime 和状态执行
  internal/domain/        # Run、Step、Artifact、Report 等领域模型
  internal/memory/        # 本地 evidence store 与检索
  internal/llm/           # Fake、Claude、OpenAI-compatible、Ollama Provider
  internal/storage/       # SQLite 持久化和内存测试实现
  internal/tools/         # Tool Registry 和工具实现
  data/evidence/          # 候选人项目证据
  examples/jds/           # 示例岗位 JD
  docs/                   # PRD 和技术设计
  web/                    # React Trace UI
```

## 后续路线

1. 增加 eval cases 和 golden trace tests。
2. 增加 SQLite FTS / pgvector 语义检索。
3. 增加结构化输出 schema，减少 LLM JSON 解析失败。
4. 增加 Markdown / PDF 导出。
5. 增加 Docker Compose 和部署说明。
