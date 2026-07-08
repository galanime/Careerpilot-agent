# CareerPilot Agent 四源融合方案

日期：2026-07-08

## 1. 融合对象

本次融合不是把四个仓库代码硬拼在一起，而是抽取各自最有价值的能力，形成一个统一产品：**CareerPilot Agent：张恒玮个人求职 Agent / 负责任求职运营系统**。

四个来源：

- `AI Job Search`：借鉴 `/setup → /scrape → /rank → /apply` 的个人求职工作流、候选人画像、岗位评分、定制 CV、cover letter、面试准备和 drafter-reviewer 思路。
- `Career-Ops`：借鉴机会 pipeline、A-F / 多维评分、批量岗位筛选、ATS PDF、tracker、公司研究、联系人研究、human-in-the-loop 和反垃圾投递原则。
- `job-hunt-kb`：作为你的本地材料知识库，提供候选人资料、项目、实习、技能、岗位画像、ATS 关键词、模板和防乱写 guardrails。
- `careerpilot-agent`：作为统一 Agent Runtime，负责 JD 解析、证据检索、匹配评分、机会决策、材料生成、自检评估、申请计划和 Trace UI。

## 2. 统一产品定位

一句话：

> 输入 JD 或岗位链接，系统先判断是否值得投，再从张恒玮本地真实 evidence 中选择素材，生成定制简历、自我介绍、开放题回答、面试准备和追踪字段；系统不自动投递，只辅助本人做更少但更准的申请。

核心边界：

- 不做垃圾投递。
- 不自动提交申请。
- 不伪造经历、指标、模型训练或商用规模。
- 低匹配岗位输出“不建议投递 / 先补证据”，而不是强行生成漂亮话。
- 所有材料必须由本人确认后使用。

## 3. 当前已落地的融合点

`careerpilot-agent` 的运行链路已从 5 步升级为 7 个工具步骤：

1. `parse_jd`：结构化解析 JD。
2. `search_evidence`：从本地 evidence 检索真实项目证据。
3. `score_match`：做 JD 与 evidence 的匹配评分。
4. `evaluate_opportunity`：新增，按技能匹配、项目证据、方向一致性、时间地点、风险真实性做机会评分，并输出 `apply / review_first / do_not_apply`。
5. `generate_materials`：生成简历 bullet、自我介绍和面试问答。
6. `evaluate_output`：检查关键词覆盖和无证据风险。
7. `build_application_plan`：新增，生成人工复核清单、tracker 字段、cover letter 角度、面试准备卡片和提交 guardrail。

新增产物：

- `opportunity_evaluation`：机会评分与投递决策。
- `application_plan`：申请计划与追踪字段。

新增工程能力：

- `data/opportunities/*.json`：岗位机会文件库。
- `data/pipeline.md`：本地 pipeline 收件箱。
- `POST /api/opportunities`：手动导入岗位。
- `GET /api/opportunities`：查看岗位机会列表。
- `GET /api/opportunities/{id}`：查看岗位详情。
- `POST /api/opportunities/{id}/evaluate`：从岗位机会发起完整 Agent Run。
- `POST /api/opportunities/{id}/status`：更新投递状态。
- `CAREERPILOT_JOB_HUNT_PROJECTS_PATH`：启动时读取 `job-hunt-kb` 项目材料并合并进 evidence 检索。
- `POST /api/scan`：白名单 scanner，支持 Greenhouse、Lever、Ashby、direct careers URL。
- `POST /api/liveness`：岗位链接有效性检查，识别关闭、过期、404 和信号不足。
- Dashboard：岗位导入、机会列表、状态更新、白名单扫描、liveness、Run Trace、Artifact 预览。

## 3.1 功能矩阵

| 功能 | AI Job Search | Career-Ops | job-hunt-kb | CareerPilot 当前状态 |
| --- | --- | --- | --- | --- |
| 个人画像 / setup | 借鉴 | 借鉴 profile | 已有 profile.yaml | 通过 `job-hunt-kb` 读取，后续补 setup API |
| 岗位导入 | URL / text | URL / pipeline | JD 文本 | 已支持手动 JD 入库 |
| 岗位扫描 | 多门户技能 | ATS/API/Playwright | 无 | 已实现基础版，支持 Greenhouse / Lever / Ashby / direct |
| 去重 | seen jobs | scan-history / fingerprint | 无 | 已支持 URL + fingerprint 去重 |
| 排名 / 机会评分 | `/rank` | A-F / 10维评分 | role profile | 已支持 5 维 opportunity score |
| 反垃圾投递 | fit gate | human-in-the-loop | guardrails | 已写入 opportunity decision 与 submission guardrail |
| 证据检索 | profile docs | cv.md / proof points | materials/*.yaml | 已支持 evidence + job-hunt-kb 项目桥接 |
| 简历定制 | LaTeX CV | ATS PDF | Jinja 模板 | 当前生成 Markdown bullet，PDF 规划中 |
| Cover letter | LaTeX | HTML/PDF | 可新增模板 | 当前生成角度，完整草稿规划中 |
| 面试准备 | interview prep | STAR+R story bank | 自我介绍模板 | 当前生成 QA 卡片，故事库规划中 |
| PDF/ATS 校验 | pdftotext | Playwright/PDF | 未做 | 规划中 |
| Tracker | CSV | applications.md | outputs/packages | 已支持 opportunity status 与 pipeline.md，applications.md 规划中 |
| Liveness | URL fetch | liveness gate | 无 | 已实现基础版 |
| 公司研究 | reviewer | deep/contacto | 无 | 规划中 |
| Dashboard | 无 | Go TUI / Web UI | 无 | 已实现 Web 基础版 |
| 多 CLI / Agent 技能 | Claude Code | 多 CLI | CLI | 当前 Go API，技能封装规划中 |

## 4. 建议目录分工

保留当前双项目分工：

- `job-hunt-kb/`：材料库与模板生成器。
- `careerpilot-agent/`：Agent Runtime、API、Trace UI 和机会 pipeline。

后续可以在 `careerpilot-agent` 内新增：

```text
careerpilot-agent/
  data/
    opportunities/          # 岗位原文、URL、公司、状态
    tracker/                # 投递状态、复盘、下一步行动
    portals/                # 公司白名单与岗位源配置
  internal/
    opportunities/          # 机会入库、去重、状态管理
    scraper/                # 白名单岗位源扫描，不做高频爬取
    exporter/               # Markdown / PDF / Word 导出
```

## 5. 后续优先级

P0：已完成首步融合

- 机会评分工具。
- 反垃圾投递策略。
- 申请计划 artifact。
- 测试链路更新。

P1：岗位 pipeline

- 新增 opportunity 数据模型。
- 支持手动粘贴 JD 入库。
- 支持状态：`new / scored / ready_for_review / applied / interview / rejected / archived`。
- 支持按公司、岗位方向、得分、状态筛选。

P2：本地知识库桥接

- 让 `careerpilot-agent` 直接读取 `job-hunt-kb/data/materials/*.yaml`。
- 把 `job-hunt-kb` 的岗位画像、ATS 关键词和模板作为 Runtime 工具输入。
- 生成 `resume_ats_zh.md`、`resume_hr_zh.md`、`project_300char_zh.md`、`self_intro_1min_zh.md`。

P3：扫描与去重

- 只扫白名单公司和公开页面。
- 支持手动导入 URL / JD。
- 对岗位做标题、公司、地点、JD 指纹去重。
- 默认低频、人工触发，不做账号自动化。

P4：材料导出与校验

- Markdown 首版。
- PDF / Word 后续。
- ATS 文本层检查。
- 版面与链接检查。

P5：行业版本

- AI 应用工程实习版。
- Python 后端 / 平台工程版。
- 金融科技 / 银行科技版。
- AI 产品技术协同版。

## 6. 简历项目表述建议

项目名：

**CareerPilot Agent：证据驱动型求职运营 Agent**

简历 bullet：

- 基于 Go 实现求职 Agent Runtime，将 JD 解析、项目证据检索、匹配评分、机会评估、材料生成、自检评估和申请计划拆成可观测工具链，并通过 HTTP API / SSE Trace 输出完整执行过程。
- 融合开源 AI Job Search / Career-Ops 的工作流思想与本地 `job-hunt-kb` 知识库，新增机会评分和 human-in-the-loop guardrail，低匹配岗位输出不建议投递，避免垃圾投递和无证据包装。
- 使用本地 evidence 约束简历 bullet、自我介绍和面试问答生成，Evaluator 检查关键词覆盖与无证据主张，使求职材料生成从“单轮 prompt”升级为可追踪、可复核的 Agent 工作流。
