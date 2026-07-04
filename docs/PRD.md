# CareerPilot Agent 产品文档

## 1. 产品定位

CareerPilot Agent 是一个面向 AI Agent / 大模型应用 / 后端 AI Infra 岗位投递的证据驱动型 Agent 产品。用户输入岗位 JD 后，系统自动完成岗位结构化分析、候选人项目证据检索、岗位匹配评分、简历 bullet 生成、面试问答准备和输出自检。

项目的展示重点不是“生成简历”，而是“实现一个可观测、可评测、可约束的 Agent Runtime，并用求职投递场景证明它能解决真实问题”。

## 2. 目标用户

- 正在投递 AI Agent、LLM Application、AI Platform、后端 AI Infra 岗位的候选人。
- 需要针对不同 JD 快速定位项目证据、生成投递材料和准备面试讲解的人。
- 面试官或技术评审，可以通过 Trace 理解候选人是否真正掌握 Agent 工程化。

## 3. 用户痛点

1. 通用 LLM 容易编造不存在的经历。
2. 简历优化通常无法解释“为什么这么写”。
3. JD 关键词和候选人真实项目证据之间缺少结构化映射。
4. 普通 Prompt Demo 不能体现 Agent 编排、工具调用、可观测和评测能力。
5. 候选人需要一个可以在 GitHub、作品集、面试中同时讲清楚的项目。

## 4. 核心价值

CareerPilot Agent 通过以下方式解决问题：

- 用 Tool Registry 限制 Agent 动作空间，避免自由生成不可控。
- 用本地 evidence 作为材料生成依据，降低幻觉和夸大风险。
- 用 Run / Step / Artifact 记录完整执行过程。
- 用 Evaluator 检查关键词覆盖和无证据主张。
- 用 Go 后端展示可靠服务、并发上下文、接口设计和测试能力。

## 5. MVP 用户流程

1. 用户输入公司名、岗位名、目标方向和 JD 文本。
2. Agent 创建 Run，Planner 生成执行计划。
3. `parse_jd` 工具提取职责、技能和关键词。
4. `search_evidence` 工具从候选人项目证据中检索匹配项目。
5. `score_match` 工具生成匹配评分和短板列表。
6. `generate_materials` 工具生成简历 bullet、自我介绍和面试问答。
7. `evaluate_output` 工具生成自检报告。
8. 用户查看 artifacts 和 step trace。

## 6. 功能范围

### P0

- JD 输入与 Run 创建。
- Agent Runtime 顺序执行状态机。
- 本地 evidence 检索。
- 匹配评分。
- 投递材料生成。
- 自检评估。
- HTTP API。
- 基础测试。

### P1

- SQLite 持久化。
- 异步执行和实时 SSE。
- React Trace UI。
- LLM Provider 抽象接入真实模型。
- Eval cases 和 golden tests。

### P2

- pgvector / Qdrant 语义检索。
- Prompt 版本管理。
- Markdown / PDF 导出。
- 人工确认节点。
- 部署和演示环境。

### 暂不做

- 自动投递。
- 自动发送邮件。
- 浏览器批量操作。
- 大规模爬取岗位。
- 账号、支付或外部平台自动化。

这些功能容易把项目带向灰色自动化，不利于突出 AI Agent 工程能力。

## 7. 产物定义

每次 Run 输出四类 Artifact：

1. JD 结构化分析。
2. 岗位匹配报告。
3. 投递材料草稿。
4. 自检评估报告。

## 8. 面试展示话术

可以这样解释项目：

> 我没有做一个单轮 Prompt 简历生成器，而是实现了一个 Go 版 Agent Runtime。系统会先把 JD 解析成结构化技能需求，再从本地 evidence 中检索真实项目证据，随后生成匹配评分和投递材料。每一步都通过 Tool Registry 执行并记录为 Step，最终由 Evaluator 检查关键词覆盖和无证据主张。这样既能展示 Agent 编排能力，也能避免求职材料生成中的幻觉问题。
