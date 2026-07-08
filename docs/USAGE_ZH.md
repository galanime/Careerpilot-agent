# CareerPilot Agent 使用说明

## 1. 产品是什么

CareerPilot Agent 是一个本地优先、证据驱动、人工确认的求职运营 Agent。它融合四个来源：

- AI Job Search：个人画像、岗位 rank、定制 CV / cover letter、面试准备、PDF / ATS 校验思想。
- Career-Ops：机会 pipeline、A-G 评估、scanner / tracker、报告、liveness、人机确认和反垃圾投递原则。
- job-hunt-kb：张恒玮本地求职知识库，包含项目、实习、技能、岗位画像、ATS 关键词和模板。
- careerpilot-agent：Go Agent Runtime、工具链、API、SSE Trace 和前端控制台。

系统默认不自动投递、不发送邮件、不批量填表。它只做判断、生成草稿、保存追踪状态，最后由本人确认和提交。

## 2. 启动

```powershell
cd D:\C0dex-AIcoding\careerpilot-agent
$env:CAREERPILOT_LLM_PROVIDER = "fake"
D:\Data\go-sdk\go\bin\go.exe run .\cmd\server
```

默认地址：

```text
http://127.0.0.1:8788
```

可选环境变量：

```powershell
$env:CAREERPILOT_ADDR = ":8788"
$env:CAREERPILOT_DB_PATH = "careerpilot.db"
$env:CAREERPILOT_EVIDENCE_PATH = "data/evidence/projects.yaml"
$env:CAREERPILOT_JOB_HUNT_PROJECTS_PATH = "../job-hunt-kb/data/materials/projects.yaml"
$env:CAREERPILOT_OPPORTUNITY_DIR = "data/opportunities"
$env:CAREERPILOT_PIPELINE_PATH = "data/pipeline.md"
```

## 3. 导入岗位

```powershell
$body = @{
  company_name = "ByteDance"
  job_title = "AI Agent Engineer Intern"
  target_role = "AI Agent Engineer"
  location = "北京"
  url = "https://example.com/jobs/agent-intern"
  jd_text = "负责 AI Agent 应用开发，要求 Python、Go、RAG、Tool Calling、后端工程和评估能力。"
} | ConvertTo-Json

Invoke-RestMethod -Method Post `
  -Uri http://127.0.0.1:8788/api/opportunities `
  -Body $body `
  -ContentType "application/json"
```

效果：

- 写入 `data/opportunities/{id}.json`
- 追加 `data/pipeline.md`
- 默认状态为 `new`
- 重复 URL 或重复指纹会返回 409，避免重复评估

## 4. 查看机会列表

```powershell
Invoke-RestMethod http://127.0.0.1:8788/api/opportunities
```

查看单个机会：

```powershell
Invoke-RestMethod http://127.0.0.1:8788/api/opportunities/<id>
```

## 5. 评估岗位

```powershell
Invoke-RestMethod -Method Post `
  -Uri http://127.0.0.1:8788/api/opportunities/<id>/evaluate
```

评估链路：

1. `parse_jd`：解析职责、技能、关键词。
2. `search_evidence`：检索 `data/evidence/projects.yaml` 与 `job-hunt-kb` 项目证据。
3. `score_match`：生成匹配分和短板。
4. `evaluate_opportunity`：生成机会评分、投递决策和反垃圾投递提醒。
5. `generate_materials`：生成简历 bullet、自我介绍和面试问答。
6. `evaluate_output`：检查关键词覆盖和无证据主张。
7. `build_application_plan`：生成 tracker 字段、cover letter 角度、面试准备卡片和提交 guardrail。

产物类型：

- `jd_analysis`
- `match_report`
- `opportunity_evaluation`
- `application_materials`
- `eval_report`
- `application_plan`

评估完成后，机会 JSON 会回写：

- `score`
- `decision`
- `report_run_id`
- `status`

## 6. 查看 Run 与产物

```powershell
Invoke-RestMethod http://127.0.0.1:8788/api/runs
Invoke-RestMethod http://127.0.0.1:8788/api/runs/<run_id>
Invoke-RestMethod http://127.0.0.1:8788/api/runs/<run_id>/artifacts
```

SSE Trace：

```powershell
curl.exe -N http://127.0.0.1:8788/api/runs/<run_id>/events
```

## 7. 更新投递状态

```powershell
$body = @{
  status = "applied"
  notes = @("2026-07-08 已人工确认后投递")
} | ConvertTo-Json

Invoke-RestMethod -Method Post `
  -Uri http://127.0.0.1:8788/api/opportunities/<id>/status `
  -Body $body `
  -ContentType "application/json"
```

可用状态：

- `new`
- `scored`
- `ready_for_review`
- `applied`
- `interview`
- `rejected`
- `archived`
- `do_not_apply`

## 8. 推荐工作流

1. 手动粘贴 JD 或后续由 scanner 导入岗位。
2. 先看机会评分，低于 `review_first` 不进入投递。
3. 只对 `apply` 或 `review_first` 的岗位生成材料。
4. 人工检查材料，特别是：
   - 是否夸大模型训练能力
   - 是否写了未验证指标
   - 是否误把参与开发写成主导
   - 是否暴露私有项目或客户信息
5. 人工投递后更新状态。
6. 面试前查看 `application_plan` 的面试卡片。

## 9. Scanner 与 Dashboard

当前 Dashboard 已包含这些区域：

- 岗位导入：手动粘贴 JD，加入本地 pipeline。
- 机会列表：查看机会状态、分数、决策，一键评估或更新状态。
- 白名单扫描：提交 scan JSON，扫描公开 ATS / careers URL。
- Liveness：检查岗位 URL 是否仍有效。
- Run Trace：查看 Agent 工具链执行过程。
- Artifacts：查看 JD 分析、匹配报告、机会评分、材料草稿、自检报告和申请计划。

Scanner 支持 provider：

- `greenhouse`：需要 `slug`。
- `lever`：需要 `slug`。
- `ashby`：需要 `slug`。
- `direct`：需要 `careers_url`，用于轻量 liveness 与手动 follow。

Scanner 示例：

```json
{
  "target_role": "AI Agent Engineer",
  "title_allow": ["Agent", "AI", "LLM", "Python", "后端"],
  "title_block": ["Senior", "Staff", "Principal", "博士"],
  "companies": [
    {
      "name": "OpenAI",
      "provider": "greenhouse",
      "slug": "openai",
      "enabled": true
    }
  ]
}
```

注意：

- scanner 只适合白名单、低频、个人求职使用。
- WebSearch 和登录态 ATS 自动化暂不进入核心功能。
- URL 结果会先经过 liveness 检查，再入库。
- 重复 URL 或重复 JD 指纹会跳过，避免同一岗位重复投递。

## 10. 后续功能路线

下一批最值得开发：

- A-G 深度报告：角色摘要、CV 匹配、级别策略、薪酬需求、定制计划、面试计划、岗位真实性。
- ATS / PDF 导出：Markdown → HTML/PDF，检查文本层可解析性。
- Cover letter / 邮件草稿：只生成草稿，不发送。
- 面试故事库：沉淀 STAR+Reflection 案例。
- 更完整 scanner provider：Workday、BambooHR、Teamtailor、RSS、自定义本地 parser。
