package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"careerpilot-agent/internal/domain"
)

type EvaluateOpportunityTool struct{}

type EvaluateOpportunityInput struct {
	CompanyName string                      `json:"company_name"`
	JobTitle    string                      `json:"job_title"`
	TargetRole  string                      `json:"target_role"`
	Analysis    domain.JDAnalysis           `json:"analysis"`
	Evidence    domain.EvidenceSearchResult `json:"evidence"`
	Match       domain.MatchReport          `json:"match"`
}

func (t *EvaluateOpportunityTool) Name() string {
	return "evaluate_opportunity"
}

func (t *EvaluateOpportunityTool) Description() string {
	return "Evaluate whether a job is worth applying to, with anti-spam and evidence guardrails."
}

func (t *EvaluateOpportunityTool) Execute(ctx context.Context, input json.RawMessage) (json.RawMessage, error) {
	payload, err := Decode[EvaluateOpportunityInput](input)
	if err != nil {
		return nil, err
	}

	dimensions := []domain.OpportunityDimension{
		scoreSkillMatch(payload),
		scoreEvidenceFit(payload),
		scoreRoleDirection(payload),
		scoreLocationAndTiming(payload),
		scoreRisk(payload),
	}

	weightedTotal := 0
	weightSum := 0
	for _, dimension := range dimensions {
		weightedTotal += dimension.Score * dimension.Weight
		weightSum += dimension.Weight
	}

	overall := 0.0
	if weightSum > 0 {
		overall = float64(weightedTotal) / float64(weightSum)
	}

	evaluation := domain.OpportunityEvaluation{
		OverallScore:        round1(overall),
		Decision:            decisionFor(overall, payload),
		Dimensions:          dimensions,
		DealBreakers:        detectDealBreakers(payload),
		RecommendedActions:  recommendedActions(overall, payload),
		ResponsibleUseNotes: responsibleUseNotes(),
	}
	if len(evaluation.DealBreakers) > 0 && overall < 80 {
		evaluation.Decision = domain.OpportunityDecisionDoNotApply
	}
	return Encode(evaluation)
}

func scoreSkillMatch(payload EvaluateOpportunityInput) domain.OpportunityDimension {
	score := payload.Match.Score
	if score > 100 {
		score = 100
	}
	return domain.OpportunityDimension{
		Name:      "技能匹配",
		Score:     score,
		Weight:    30,
		Rationale: fmt.Sprintf("基于 JD 关键词与本地证据匹配，当前匹配分为 %d/100。", payload.Match.Score),
	}
}

func scoreEvidenceFit(payload EvaluateOpportunityInput) domain.OpportunityDimension {
	score := 35
	if len(payload.Evidence.Items) >= 3 {
		score = 85
	} else if len(payload.Evidence.Items) == 2 {
		score = 75
	} else if len(payload.Evidence.Items) == 1 {
		score = 62
	}
	return domain.OpportunityDimension{
		Name:      "项目证据",
		Score:     score,
		Weight:    25,
		Rationale: fmt.Sprintf("检索到 %d 条可用于简历和面试的 evidence。", len(payload.Evidence.Items)),
	}
}

func scoreRoleDirection(payload EvaluateOpportunityInput) domain.OpportunityDimension {
	text := strings.ToLower(payload.JobTitle + " " + payload.TargetRole + " " + strings.Join(payload.Analysis.Keywords, " "))
	score := 65
	if strings.Contains(text, "agent") || strings.Contains(text, "大模型") || strings.Contains(text, "llm") || strings.Contains(text, "后端") || strings.Contains(text, "python") {
		score = 88
	}
	if strings.Contains(text, "算法") || strings.Contains(text, "训练") || strings.Contains(text, "cuda") {
		score -= 20
	}
	return domain.OpportunityDimension{
		Name:      "方向一致性",
		Score:     clamp(score, 0, 100),
		Weight:    20,
		Rationale: "优先匹配 AI 应用工程、Agent 应用开发、Python 自动化和后端工程方向。",
	}
}

func scoreLocationAndTiming(payload EvaluateOpportunityInput) domain.OpportunityDimension {
	text := strings.ToLower(payload.CompanyName + " " + payload.JobTitle + " " + payload.TargetRole + " " + strings.Join(payload.Analysis.Keywords, " "))
	score := 70
	if strings.Contains(text, "实习") || strings.Contains(text, "intern") {
		score += 12
	}
	if strings.Contains(text, "北京") || strings.Contains(text, "上海") || strings.Contains(text, "苏州") || strings.Contains(text, "remote") || strings.Contains(text, "远程") {
		score += 8
	}
	return domain.OpportunityDimension{
		Name:      "时间与地点",
		Score:     clamp(score, 0, 100),
		Weight:    10,
		Rationale: "默认按张恒玮当前实习优先、北京/上海/苏州优先的策略评分。",
	}
}

func scoreRisk(payload EvaluateOpportunityInput) domain.OpportunityDimension {
	text := strings.ToLower(payload.JobTitle + " " + payload.TargetRole + " " + strings.Join(payload.Analysis.RequiredSkills, " ") + " " + strings.Join(payload.Analysis.Keywords, " "))
	score := 85
	riskyTerms := []string{"预训练", "模型训练", "cuda", "c++", "强化学习", "多模态算法", "博士"}
	for _, term := range riskyTerms {
		if strings.Contains(text, strings.ToLower(term)) {
			score -= 10
		}
	}
	if len(payload.Match.Gaps) > 5 {
		score -= 10
	}
	return domain.OpportunityDimension{
		Name:      "风险与真实性",
		Score:     clamp(score, 0, 100),
		Weight:    15,
		Rationale: "惩罚与真实经历不一致的模型训练、底层算法或过高学历门槛要求。",
	}
}

func detectDealBreakers(payload EvaluateOpportunityInput) []string {
	text := strings.ToLower(payload.JobTitle + " " + payload.TargetRole + " " + strings.Join(payload.Analysis.RequiredSkills, " ") + " " + strings.Join(payload.Analysis.Keywords, " "))
	var blockers []string
	for _, term := range []string{"博士", "phd", "cuda", "大模型预训练", "模型训练经验"} {
		if strings.Contains(text, strings.ToLower(term)) {
			blockers = append(blockers, "JD 明确要求 "+term+"，与当前主线证据存在风险，需要人工确认。")
		}
	}
	if len(payload.Evidence.Items) == 0 {
		blockers = append(blockers, "没有检索到可支撑该岗位的本地项目证据。")
	}
	return blockers
}

func recommendedActions(overall float64, payload EvaluateOpportunityInput) []string {
	switch {
	case overall >= 78:
		return []string{"生成定制简历与面试准备材料", "人工检查证据边界后投递", "把岗位加入高优先级追踪"}
	case overall >= 62:
		return []string{"先人工复核 JD 要求和短板", "只使用有证据支撑的 bullet", "补充 1-2 条项目证据后再投递"}
	default:
		return []string{"不建议投递该岗位", "记录缺口用于后续学习或项目补强", "继续寻找更贴近 AI 应用工程 / Python 自动化的岗位"}
	}
}

func responsibleUseNotes() []string {
	return []string{
		"系统只生成建议和草稿，不自动提交申请。",
		"低匹配岗位不做硬包装，优先输出不建议投递或需补证据。",
		"所有简历、cover letter 和开放题回答必须由本人复核后使用。",
		"不得虚构模型训练、商业规模化、用户量或未验证指标。",
	}
}

func decisionFor(overall float64, payload EvaluateOpportunityInput) domain.OpportunityDecision {
	if overall >= 78 && len(payload.Evidence.Items) > 0 {
		return domain.OpportunityDecisionApply
	}
	if overall >= 62 {
		return domain.OpportunityDecisionReviewFirst
	}
	return domain.OpportunityDecisionDoNotApply
}

func round1(value float64) float64 {
	return float64(int(value*10+0.5)) / 10
}

func clamp(value int, min int, max int) int {
	if value < min {
		return min
	}
	if value > max {
		return max
	}
	return value
}

type BuildApplicationPlanTool struct{}

type BuildApplicationPlanInput struct {
	CompanyName string                       `json:"company_name"`
	JobTitle    string                       `json:"job_title"`
	Analysis    domain.JDAnalysis            `json:"analysis"`
	Evidence    domain.EvidenceSearchResult  `json:"evidence"`
	Match       domain.MatchReport           `json:"match"`
	Opportunity domain.OpportunityEvaluation `json:"opportunity"`
	Materials   domain.GeneratedMaterials    `json:"materials"`
}

func (t *BuildApplicationPlanTool) Name() string {
	return "build_application_plan"
}

func (t *BuildApplicationPlanTool) Description() string {
	return "Build a human-in-the-loop application plan, tracker fields, cover-letter angles, and interview prep cards."
}

func (t *BuildApplicationPlanTool) Execute(ctx context.Context, input json.RawMessage) (json.RawMessage, error) {
	payload, err := Decode[BuildApplicationPlanInput](input)
	if err != nil {
		return nil, err
	}

	status := "needs_review"
	if payload.Opportunity.Decision == domain.OpportunityDecisionApply {
		status = "ready_for_human_review"
	}
	if payload.Opportunity.Decision == domain.OpportunityDecisionDoNotApply {
		status = "do_not_apply"
	}

	company := payload.CompanyName
	if company == "" {
		company = "目标公司"
	}
	role := payload.JobTitle
	if role == "" {
		role = "目标岗位"
	}

	plan := domain.ApplicationPlan{
		Status: status,
		NextActions: []string{
			"把 JD 原文、评分报告和生成材料保存到本地 pipeline。",
			"人工检查岗位是否仍在招、是否满足实习时间和城市要求。",
			"只保留本地 evidence 能支撑的简历 bullet。",
		},
		TrackerFields: []string{
			"company=" + company,
			"role=" + role,
			fmt.Sprintf("score=%.1f", payload.Opportunity.OverallScore),
			"decision=" + string(payload.Opportunity.Decision),
			"status=" + status,
		},
		CoverLetterAngles: []string{
			"用 AI 应用工程和 Python 自动化落地作为主线。",
			"强调从业务流程拆解到可运行原型，而不是模型训练。",
			"把领慧立芯实习作为真实工程落地支撑。",
		},
		InterviewPrepCards: []string{
			"为什么你的 Agent 项目不是普通 prompt demo？",
			"如何证明简历材料没有编造？",
			"如果 JD 要求模型训练，你如何解释自己的应用工程定位？",
		},
		SubmissionGuardrail: []string{
			"系统不得自动点击提交或批量投递。",
			"本人确认后才能投递。",
			"低于 review_first 的岗位默认不生成最终投递包。",
		},
	}

	if payload.Opportunity.Decision == domain.OpportunityDecisionDoNotApply {
		plan.NextActions = append(plan.NextActions, "不要为该岗位生成最终投递包，改为记录短板和学习计划。")
	}
	return Encode(plan)
}
