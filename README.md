# CareerPilot Agent

CareerPilot Agent 是一个本地优先、证据驱动、人工确认的求职运营 Agent。它融合两个开源求职自动化项目的产品思想，以及本地 `job-hunt-kb` 求职知识库和现有 Go Agent Runtime，围绕“岗位导入 → 机会评分 → 候选人证据检索 → 匹配评分 → 材料生成 → 自检评估 → 申请计划 → 投递追踪”的闭环流程，展示 Agent 编排、工具调用、可观测 Trace、证据约束生成和负责任自动化能力。

本项目明确不是垃圾投递器：系统不会自动提交申请、不会发送邮件、不会伪造经历；低匹配岗位会输出“不建议投递 / 先补证据”，最终投递动作必须由本人确认。

## 四源融合

- [AI Job Search](https://github.com/MadsLorentzen/ai-job-search)：借鉴 setup / scrape / rank / apply、drafter-reviewer、PDF/ATS 校验、cover letter 和面试准备流程。
- [Career-Ops](https://github.com/santifer/career-ops)：借鉴 opportunity pipeline、A-G 评估、scanner / tracker、human-in-the-loop、liveness、报告和数据契约。
- `../job-hunt-kb`：作为张恒玮本地求职知识库，提供个人资料、项目、实习、技能、岗位画像、ATS 关键词和模板。
- `careerpilot-agent`：作为统一 Go Agent Runtime、HTTP API、SSE Trace 和后续 dashboard。

## 当前能力

- Go 标准库 HTTP API。
- SQLite Run Store，持久化 runs、steps、artifacts。
- 文件型 Opportunity Store，持久化岗位机会、状态、评分与决策。
- `data/pipeline.md` 本地岗位收件箱。
- 白名单岗位 scanner：支持 Greenhouse、Lever、Ashby 和 direct careers URL。
- Liveness gate：对岗位 URL 做有效性检查，识别关闭、404、过期和信号不足。
- A-G 深度评估报告、cover letter 草稿、申请邮件草稿和 ATS/PDF 检查清单。
- 从 opportunity 发起评估时自动写入 `reports/` 与 `data/applications.md`。
- 本地 YAML 风格 evidence 检索。
- `job-hunt-kb` 项目材料桥接：启动时可读取 `../job-hunt-kb/data/materials/projects.yaml` 并合并进 evidence 检索。
- 工具链：`parse_jd`、`search_evidence`、`score_match`、`evaluate_opportunity`、`generate_materials`、`evaluate_output`、`build_application_plan`。
- LLM Provider：Fake、Anthropic/Claude、OpenAI-compatible、Ollama。
- 后端单元测试与 golden trace 测试。
- React Trace UI：Run 创建表单、最近 Run 列表、Trace Timeline、Artifact Tabs、Eval Dashboard。
- 实时 SSE Trace，推送 run、step、artifact 事件。

## 快速启动

环境要求：

- Go 1.24+，本机便携版路径：`D:\Data\go-sdk\go`
- 可选：Node.js 20+，用于前端 Trace UI

```powershell
cd D:\C0dex-AIcoding\careerpilot-agent
D:\Data\go-sdk\go\bin\go.exe test ./...
D:\Data\go-sdk\go\bin\go.exe run ./cmd/server
```

服务默认监听：

```text
http://127.0.0.1:8788
```

可选配置：

```powershell
$env:CAREERPILOT_DB_PATH = "careerpilot.db"
$env:CAREERPILOT_OPPORTUNITY_DIR = "data/opportunities"
$env:CAREERPILOT_PIPELINE_PATH = "data/pipeline.md"
$env:CAREERPILOT_REPORTS_DIR = "reports"
$env:CAREERPILOT_APPLICATIONS_PATH = "data/applications.md"
$env:CAREERPILOT_EVIDENCE_PATH = "data/evidence/projects.yaml"
$env:CAREERPILOT_JOB_HUNT_PROJECTS_PATH = "../job-hunt-kb/data/materials/projects.yaml"
$env:CAREERPILOT_LLM_PROVIDER = "fake"       # fake | anthropic | openai | ollama
$env:CAREERPILOT_LLM_MODEL = "gpt-4o-mini"
$env:CAREERPILOT_LLM_API_KEY = "..."
$env:CAREERPILOT_LLM_BASE_URL = "..."
```

## 常用 API

健康检查：

```powershell
Invoke-RestMethod http://127.0.0.1:8788/healthz
```

导入岗位机会：

```powershell
$body = @{
  company_name = "ByteDance"
  job_title = "AI Agent Engineer Intern"
  target_role = "AI Agent Engineer"
  location = "北京"
  url = "https://example.com/jobs/agent"
  jd_text = "负责 AI Agent 应用开发，要求 Python、Go、RAG、Tool Calling、后端工程和评估能力。"
} | ConvertTo-Json

Invoke-RestMethod -Method Post `
  -Uri http://127.0.0.1:8788/api/opportunities `
  -Body $body `
  -ContentType "application/json"
```

评估岗位：

```powershell
Invoke-RestMethod -Method Post `
  -Uri http://127.0.0.1:8788/api/opportunities/<id>/evaluate
```

从 opportunity 发起评估会额外落盘：

- `reports/*.md`：A-G 深度评估报告。
- `data/applications.md`：本地投递 tracker。

更新投递状态：

```powershell
$body = @{
  status = "applied"
  notes = @("已人工确认后投递")
} | ConvertTo-Json

Invoke-RestMethod -Method Post `
  -Uri http://127.0.0.1:8788/api/opportunities/<id>/status `
  -Body $body `
  -ContentType "application/json"
```

扫描白名单公司岗位：

```powershell
$body = @{
  target_role = "AI Agent Engineer"
  title_allow = @("Agent", "AI", "LLM", "Python", "后端")
  title_block = @("Senior", "Staff", "Principal", "博士")
  companies = @(
    @{
      name = "OpenAI"
      provider = "greenhouse"
      slug = "openai"
      enabled = $true
    }
  )
} | ConvertTo-Json -Depth 5

Invoke-RestMethod -Method Post `
  -Uri http://127.0.0.1:8788/api/scan `
  -Body $body `
  -ContentType "application/json"
```

检查岗位链接是否有效：

```powershell
$body = @{ url = "https://example.com/jobs/agent" } | ConvertTo-Json

Invoke-RestMethod -Method Post `
  -Uri http://127.0.0.1:8788/api/liveness `
  -Body $body `
  -ContentType "application/json"
```

Run 与 Trace：

```powershell
Invoke-RestMethod http://127.0.0.1:8788/api/runs
Invoke-RestMethod http://127.0.0.1:8788/api/runs/<run_id>/artifacts
curl.exe -N http://127.0.0.1:8788/api/runs/<run_id>/events
```

## API Surface

- `GET /healthz`
- `GET /api/runs`
- `POST /api/runs`
- `GET /api/runs/{run_id}`
- `GET /api/runs/{run_id}/artifacts`
- `GET /api/runs/{run_id}/events`
- `GET /api/opportunities`
- `POST /api/opportunities`
- `GET /api/opportunities/{id}`
- `POST /api/opportunities/{id}/evaluate`
- `POST /api/opportunities/{id}/status`
- `POST /api/scan`
- `POST /api/liveness`

## 目录结构

```text
careerpilot-agent/
  cmd/server/             # HTTP 服务入口
  internal/api/           # API 路由和 handler
  internal/agent/         # Agent Runtime 和状态执行
  internal/domain/        # Run、Opportunity、Artifact 等领域模型
  internal/memory/        # 本地 evidence store、job-hunt-kb 桥接
  internal/opportunities/ # 文件型岗位机会库、去重、状态管理
  internal/scanner/       # 白名单 scanner 与 liveness 检查
  internal/llm/           # Fake、Claude、OpenAI-compatible、Ollama Provider
  internal/storage/       # SQLite Run Store 和内存测试实现
  internal/tools/         # Tool Registry 和工具实现
  data/evidence/          # 候选人项目证据
  data/opportunities/     # 岗位机会 JSON
  data/pipeline.md        # 本地岗位收件箱
  data/applications.md    # 本地投递 tracker，gitignored
  reports/                # A-G 评估报告，gitignored
  examples/jds/           # 示例岗位 JD
  docs/                   # PRD、技术设计、融合路线图和使用说明
  web/                    # React Trace UI
```

## 文档

- [四源融合方案](docs/FUSION_ROADMAP_ZH.md)
- [使用说明](docs/USAGE_ZH.md)
- [数据契约](DATA_CONTRACT_ZH.md)
- [PRD](docs/PRD.md)
- [技术设计](docs/TECH_DESIGN.md)

## 后续路线

1. A-G 深度报告：角色摘要、CV 匹配、级别策略、薪酬需求、定制计划、面试计划、岗位真实性。
2. Markdown / HTML / PDF 导出和 ATS 文本层校验。
3. Cover letter、邮件草稿、开放题回答和面试故事库。
4. 更完整 scanner provider：Workday、BambooHR、Teamtailor、RSS、自定义本地 parser。
5. SQLite FTS / pgvector 语义检索。
