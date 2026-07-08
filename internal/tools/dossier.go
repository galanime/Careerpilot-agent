package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"careerpilot-agent/internal/domain"
)

type BuildDossierTool struct{}

type BuildDossierInput struct {
	CompanyName string                       `json:"company_name"`
	JobTitle    string                       `json:"job_title"`
	TargetRole  string                       `json:"target_role"`
	Analysis    domain.JDAnalysis            `json:"analysis"`
	Evidence    domain.EvidenceSearchResult  `json:"evidence"`
	Match       domain.MatchReport           `json:"match"`
	Opportunity domain.OpportunityEvaluation `json:"opportunity"`
	Materials   domain.GeneratedMaterials    `json:"materials"`
	Eval        domain.EvalReport            `json:"eval"`
	Plan        domain.ApplicationPlan       `json:"plan"`
	GeneratedAt time.Time                    `json:"generated_at"`
}

func (t *BuildDossierTool) Name() string {
	return "build_dossier"
}

func (t *BuildDossierTool) Description() string {
	return "Build A-G evaluation report, cover letter draft, email draft, open question answers, and ATS checklist."
}

func (t *BuildDossierTool) Execute(ctx context.Context, input json.RawMessage) (json.RawMessage, error) {
	payload, err := Decode[BuildDossierInput](input)
	if err != nil {
		return nil, err
	}
	if payload.GeneratedAt.IsZero() {
		payload.GeneratedAt = time.Now().UTC()
	}
	dossier := domain.ApplicationDossier{
		ReportMarkdown:     buildReport(payload),
		CoverLetterDraft:   buildCoverLetter(payload),
		EmailDraft:         buildEmail(payload),
		OpenQuestionDrafts: buildOpenQuestions(payload),
		ATSChecklist:       buildATSChecklist(payload),
	}
	return Encode(dossier)
}

func buildReport(payload BuildDossierInput) string {
	var out strings.Builder
	company := fallback(payload.CompanyName, "目标公司")
	role := fallback(payload.JobTitle, "目标岗位")
	date := payload.GeneratedAt.Format("2006-01-02")

	out.WriteString(fmt.Sprintf("# Evaluation: %s — %s\n\n", company, role))
	out.WriteString(fmt.Sprintf("**Date:** %s\n", date))
	out.WriteString(fmt.Sprintf("**Target Role:** %s\n", fallback(payload.TargetRole, "未填写")))
	out.WriteString(fmt.Sprintf("**Score:** %.1f/100\n", payload.Opportunity.OverallScore))
	out.WriteString(fmt.Sprintf("**Decision:** %s\n\n", payload.Opportunity.Decision))
	out.WriteString("---\n\n")
	out.WriteString("## Machine Summary\n\n")
	out.WriteString("```yaml\n")
	out.WriteString(fmt.Sprintf("company: %q\nrole: %q\ndecision: %q\nscore: %.1f\nunsupported_claims: %d\n", company, role, payload.Opportunity.Decision, payload.Opportunity.OverallScore, len(payload.Eval.UnsupportedClaims)))
	out.WriteString("```\n\n")

	out.WriteString("## A) Role Summary\n\n")
	out.WriteString(listOrFallback(payload.Analysis.Responsibilities, "JD 未提供足够职责细节。"))
	out.WriteString("\n## B) Match With Evidence\n\n")
	out.WriteString("**强匹配：**\n")
	out.WriteString(listOrFallback(payload.Match.StrongMatches, "暂无强匹配关键词。"))
	out.WriteString("\n**推荐使用的项目证据：**\n")
	out.WriteString(listOrFallback(payload.Match.RecommendedUse, "未检索到可用项目证据。"))
	out.WriteString("\n## C) Level And Strategy\n\n")
	out.WriteString("以实习 / 初级 AI 应用工程候选人定位，不包装成模型训练或底层算法候选人。主线强调 Python 工程、Agent 工作流、工具调用、证据检索、Trace 和人工审核。\n\n")
	out.WriteString("## D) Comp And Demand\n\n")
	out.WriteString("当前未接入薪酬数据源；本系统不编造薪资。若 JD 提供薪资，应在人工复核时原文记录。\n\n")
	out.WriteString("## E) Customization Plan\n\n")
	out.WriteString(listOrFallback(payload.Plan.NextActions, "先补充可验证 evidence，再定制材料。"))
	out.WriteString("\n## F) Interview Plan\n\n")
	out.WriteString(listOrFallback(payload.Plan.InterviewPrepCards, "暂无面试卡片。"))
	out.WriteString("\n## G) Posting Legitimacy\n\n")
	out.WriteString("基础版仅完成 URL liveness 检查；深度公司真实性、repost、裁员和薪酬研究需后续 deep research 模块补齐。\n\n")
	out.WriteString("## Cover Letter Draft\n\n")
	out.WriteString(buildCoverLetter(payload))
	out.WriteString("\n\n## Application Email Draft\n\n")
	out.WriteString(buildEmail(payload))
	out.WriteString("\n\n## ATS Checklist\n\n")
	out.WriteString(listOrFallback(buildATSChecklist(payload), "暂无检查项。"))
	return out.String()
}

func buildCoverLetter(payload BuildDossierInput) string {
	company := fallback(payload.CompanyName, "贵司")
	role := fallback(payload.JobTitle, "目标岗位")
	var out strings.Builder
	out.WriteString(fmt.Sprintf("尊敬的 %s 招聘团队：\n\n", company))
	out.WriteString(fmt.Sprintf("您好，我希望申请 %s。我的定位是 AI 应用工程 / Agent 应用开发 / Python 自动化落地，优势在于把业务流程拆解成可追踪、可复核的工程链路，而不是做无证据的模型训练包装。\n\n", role))
	if len(payload.Materials.ResumeBullets) > 0 {
		out.WriteString("与该岗位最相关的经历包括：\n")
		for _, bullet := range payload.Materials.ResumeBullets {
			out.WriteString("- " + bullet + "\n")
		}
		out.WriteString("\n")
	}
	out.WriteString("如果进入面试，我可以重点展开本地 Agent Runtime、job-hunt-kb 证据检索、LifeHelper / Short Video Studio / HUIHU Ops Agent 等项目如何支撑工具调用、工作流、Trace、评估和人工审核。\n\n")
	out.WriteString("感谢您的时间，期待进一步交流。\n")
	return out.String()
}

func buildEmail(payload BuildDossierInput) string {
	company := fallback(payload.CompanyName, "贵司")
	role := fallback(payload.JobTitle, "目标岗位")
	return fmt.Sprintf("主题：张恒玮 - %s - AI 应用工程实习申请\n\n您好，\n\n我想申请 %s 的 %s。我的背景是北邮新一代电子信息技术硕士在读，当前主线是 AI 应用工程、Agent 应用开发和 Python 自动化落地。随信附上简历与项目说明，材料均基于本地 evidence 生成并已人工复核。\n\n谢谢，\n张恒玮", role, company, role)
}

func buildOpenQuestions(payload BuildDossierInput) []string {
	return []string{
		"Q: 为什么选择该岗位？\nA: 岗位要求与我的 AI 应用工程、Agent 工作流、工具调用和后端工程经历匹配；我希望把现有项目中的可观测、可评估、人工审核经验迁移到真实业务。",
		"Q: 你的优势是什么？\nA: 我能把不确定业务需求拆成可运行工程链路，并用 evidence、Trace 和自检机制降低 AI 生成幻觉和无证据包装风险。",
		"Q: 你的短板是什么？\nA: 我不把自己包装成模型训练或底层算法候选人；如果岗位涉及训练/CUDA/预训练，我会如实说明当前更偏应用层工程，并用项目证明迁移能力。",
	}
}

func buildATSChecklist(payload BuildDossierInput) []string {
	checks := []string{
		"联系方式必须是可复制文本，不依赖图片或图标。",
		"保留 JD 中真实匹配的关键词，不 stuffing 无证据关键词。",
		"每个核心 bullet 都能对应本地 evidence。",
		"PDF 导出后需检查文本层顺序和关键词可解析性。",
	}
	if payload.Eval.KeywordCoverage < 0.45 {
		checks = append(checks, "关键词覆盖偏低，投递前需要补充项目证据或调整表达。")
	}
	if len(payload.Eval.UnsupportedClaims) > 0 {
		checks = append(checks, "存在无证据主张，投递前必须删除或补证据。")
	}
	return checks
}

func listOrFallback(values []string, fallbackText string) string {
	if len(values) == 0 {
		return "- " + fallbackText + "\n"
	}
	var out strings.Builder
	for _, value := range values {
		out.WriteString("- " + value + "\n")
	}
	return out.String()
}

func fallback(value string, fallbackText string) string {
	if strings.TrimSpace(value) == "" {
		return fallbackText
	}
	return strings.TrimSpace(value)
}
