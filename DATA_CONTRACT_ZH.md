# CareerPilot Agent 数据契约

日期：2026-07-08

本文件定义四源合一后的系统边界：哪些文件是“用户层”，永远不能被升级脚本自动覆盖；哪些文件是“系统层”，可以随功能迭代更新。

## 1. 用户层：不得自动覆盖

这些文件包含张恒玮的个人资料、真实经历、投递记录和生成产物。后续任何自动更新或同步逻辑都不能覆盖它们。

| 路径 | 用途 |
| --- | --- |
| `../job-hunt-kb/data/candidate/profile.yaml` | 个人资料、求职定位、联系方式 |
| `../job-hunt-kb/data/materials/*.yaml` | 教育、项目、实习、奖项、技能等真实 evidence |
| `../job-hunt-kb/data/role_profiles/*.yaml` | 面向不同岗位方向的选择策略 |
| `../job-hunt-kb/data/ats_keywords/*.yaml` | ATS 关键词库 |
| `../job-hunt-kb/data/policies/*.yaml` | 防乱写、选择规则、教育经历规则 |
| `data/evidence/projects.yaml` | CareerPilot 当前本地 evidence 索引 |
| `data/opportunities/*.json` | 手动导入或扫描得到的岗位机会 |
| `data/pipeline.md` | 待评估 / 已评估岗位 pipeline |
| `data/applications.md` | 投递 tracker，记录状态、得分、报告链接 |
| `data/scan-history.tsv` | 岗位扫描历史与去重记录 |
| `reports/*.md` | 每次岗位评估报告 |
| `output/*` | 生成的简历、cover letter、PDF、面试卡片 |
| `interview-prep/*.md` | 面试故事库、公司专项准备材料 |
| `config/profile.local.yaml` | 本地偏好、目标城市、公司白名单、模型配置 |
| `portals.local.yaml` | 本地岗位源与公司白名单 |

## 2. 系统层：可随代码迭代

这些文件是产品逻辑、模板、API、工具、测试和文档，可以在版本升级中更新。

| 路径 | 用途 |
| --- | --- |
| `cmd/` | 服务入口 |
| `internal/agent/` | Agent Runtime 与事件流 |
| `internal/api/` | HTTP API |
| `internal/domain/` | 领域模型 |
| `internal/tools/` | JD 解析、证据检索、评分、材料生成、申请计划等工具 |
| `internal/storage/` | Run 存储 |
| `internal/memory/` | evidence 检索 |
| `internal/opportunities/` | 岗位机会文件库、去重、状态管理 |
| `web/` | Trace UI / 后续 dashboard |
| `docs/` | 系统文档、路线图、技术设计 |
| `templates/` | 简历、报告、cover letter、面试卡片模板 |
| `examples/` | 示例 JD 与演示数据 |
| `tests/` / `*_test.go` | 测试 |

## 3. 文件优先原则

本项目采用 local-first 和 file-first 原则：

- 人类可读文件是长期事实来源。
- SQLite 或其他数据库只作为运行索引和缓存。
- 所有投递状态必须能从 `data/applications.md`、`data/pipeline.md`、`reports/` 还原。
- 生成材料默认进入 `output/`，不直接覆盖原始简历或知识库。

## 4. 反垃圾投递原则

系统必须遵守：

- 不自动提交申请。
- 不自动发送邮件。
- 不自动批量填写 ATS 表单。
- 不为低匹配岗位强行生成“包装版”最终材料。
- 不虚构项目、实习、指标、学历、模型训练或商业规模。
- 所有材料只能作为草稿，必须人工确认后使用。

## 5. 四源融合映射

| 来源 | 融合方式 |
| --- | --- |
| AI Job Search | 借鉴 setup / rank / apply、drafter-reviewer、PDF/ATS 校验、cover letter 与面试准备 |
| Career-Ops | 借鉴 pipeline、A-G 评估、scanner、tracker、报告、liveness、human-in-the-loop 和系统/用户层边界 |
| job-hunt-kb | 作为张恒玮本地求职知识库和材料生成规则 |
| careerpilot-agent | 作为统一 Agent Runtime、API、Trace UI 和未来 dashboard |
