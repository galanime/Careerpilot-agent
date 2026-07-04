package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"careerpilot-agent/internal/domain"
	"careerpilot-agent/internal/llm"
)

type GenerateMaterialsTool struct {
	provider llm.Provider
}

func NewGenerateMaterialsTool(provider llm.Provider) *GenerateMaterialsTool {
	return &GenerateMaterialsTool{provider: provider}
}

type GenerateMaterialsInput struct {
	CompanyName string                      `json:"company_name"`
	JobTitle    string                      `json:"job_title"`
	Analysis    domain.JDAnalysis           `json:"analysis"`
	Evidence    domain.EvidenceSearchResult `json:"evidence"`
	Match       domain.MatchReport          `json:"match"`
}

func (t *GenerateMaterialsTool) Name() string {
	return "generate_materials"
}

func (t *GenerateMaterialsTool) Description() string {
	return "Generate evidence-grounded resume bullets, pitch, and interview talking points."
}

func (t *GenerateMaterialsTool) Execute(ctx context.Context, input json.RawMessage) (json.RawMessage, error) {
	payload, err := Decode[GenerateMaterialsInput](input)
	if err != nil {
		return nil, err
	}

	if t.provider != nil {
		materials, err := t.generateWithLLM(ctx, payload)
		if err == nil {
			return Encode(materials)
		}
	}

	materials := domain.GeneratedMaterials{
		ResumeBullets: buildBullets(payload),
		Pitch:         buildPitch(payload),
		InterviewQA:   buildInterviewQA(payload),
	}
	return Encode(materials)
}

func (t *GenerateMaterialsTool) generateWithLLM(ctx context.Context, payload GenerateMaterialsInput) (domain.GeneratedMaterials, error) {
	requestJSON, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return domain.GeneratedMaterials{}, err
	}
	response, err := t.provider.Generate(ctx, llm.GenerateRequest{
		SystemPrompt: "你是证据驱动的 AI Agent 求职材料生成器。只基于输入 evidence 生成内容，禁止编造经历。必须输出严格 JSON，字段为 resume_bullets(string数组)、pitch(string)、interview_qa(string数组)。",
		Messages: []llm.Message{
			{Role: "user", Content: string(requestJSON)},
		},
		Metadata: map[string]string{"tool": "generate_materials"},
	})
	if err != nil {
		return domain.GeneratedMaterials{}, err
	}

	text := strings.TrimSpace(response.Text)
	text = strings.TrimPrefix(text, "```json")
	text = strings.TrimPrefix(text, "```")
	text = strings.TrimSuffix(text, "```")
	var materials domain.GeneratedMaterials
	if err := json.Unmarshal([]byte(strings.TrimSpace(text)), &materials); err != nil {
		return domain.GeneratedMaterials{}, err
	}
	if len(materials.ResumeBullets) == 0 || strings.TrimSpace(materials.Pitch) == "" {
		return domain.GeneratedMaterials{}, fmt.Errorf("llm response missing required materials fields")
	}
	return materials, nil
}

func buildBullets(payload GenerateMaterialsInput) []string {
	var bullets []string
	for _, item := range payload.Evidence.Items {
		detail := "围绕岗位要求沉淀可验证项目证据"
		if len(item.Details) > 0 {
			detail = item.Details[0]
		}
		tags := strings.Join(item.Tags, "/")
		if tags == "" {
			tags = "AI Agent/后端工程"
		}
		bullets = append(bullets, fmt.Sprintf("基于 %s 项目实践 %s，覆盖 %s 等能力，并可对应 JD 中的 %s 要求。", item.Project, detail, tags, firstOrDefault(payload.Analysis.Keywords, "Agent 工程化")))
		if len(bullets) >= 4 {
			break
		}
	}
	if len(bullets) == 0 {
		bullets = append(bullets, "围绕目标岗位补充真实项目证据后，再生成可投递的简历 bullet，避免无证据包装。")
	}
	return bullets
}

func buildPitch(payload GenerateMaterialsInput) string {
	company := payload.CompanyName
	if company == "" {
		company = "目标公司"
	}
	role := payload.JobTitle
	if role == "" {
		role = "AI Agent 相关岗位"
	}
	return fmt.Sprintf("我希望投递 %s 的 %s。我的优势是把大模型能力落到可观测、可评测的 Agent 工程系统里：既能做工具调用、流程编排和证据检索，也能用 Go/后端工程能力保证服务可靠性。当前匹配评分为 %d/100，建议重点展示 %s。", company, role, payload.Match.Score, strings.Join(payload.Match.RecommendedUse, "；"))
}

func buildInterviewQA(payload GenerateMaterialsInput) []string {
	qa := []string{
		"Q: 你的 Agent 和普通 Prompt 包装有什么区别？ A: 我把任务拆成 Planner、Tool Registry、Memory、Evaluator 和 Trace，每一步都有状态和可审计记录。",
		"Q: 如何避免简历生成幻觉？ A: 所有材料都从本地 evidence 检索结果出发，Evaluator 会检查无证据主张。",
	}
	if len(payload.Match.Gaps) > 0 {
		qa = append(qa, "Q: JD 中的短板怎么处理？ A: 不删除岗位关键词，而是区分强证据和待补强能力，优先用真实项目细节解释迁移能力。")
	}
	return qa
}

func firstOrDefault(values []string, fallback string) string {
	if len(values) == 0 {
		return fallback
	}
	return values[0]
}

type EvaluateOutputTool struct{}

type EvaluateOutputInput struct {
	Analysis  domain.JDAnalysis           `json:"analysis"`
	Evidence  domain.EvidenceSearchResult `json:"evidence"`
	Materials domain.GeneratedMaterials   `json:"materials"`
}

func (t *EvaluateOutputTool) Name() string {
	return "evaluate_output"
}

func (t *EvaluateOutputTool) Description() string {
	return "Check keyword coverage and unsupported claims in generated materials."
}

func (t *EvaluateOutputTool) Execute(ctx context.Context, input json.RawMessage) (json.RawMessage, error) {
	payload, err := Decode[EvaluateOutputInput](input)
	if err != nil {
		return nil, err
	}

	content := payload.Materials.Pitch + " " + strings.Join(payload.Materials.ResumeBullets, " ") + " " + strings.Join(payload.Materials.InterviewQA, " ")
	covered := 0
	for _, keyword := range payload.Analysis.Keywords {
		if containsFold(content, keyword) {
			covered++
		}
	}

	coverage := 1.0
	if len(payload.Analysis.Keywords) > 0 {
		coverage = float64(covered) / float64(len(payload.Analysis.Keywords))
	}

	report := domain.EvalReport{
		Passed:          coverage >= 0.45 && len(payload.Evidence.Items) > 0,
		KeywordCoverage: coverage,
	}
	if len(payload.Evidence.Items) == 0 {
		report.UnsupportedClaims = append(report.UnsupportedClaims, "缺少候选人项目 evidence，不能生成可投递材料")
	}
	if coverage < 0.45 {
		report.ImprovementWarnings = append(report.ImprovementWarnings, "JD 关键词覆盖偏低，建议补充项目证据或调整 bullet 表达")
	}
	if strings.Contains(content, "精通") || strings.Contains(strings.ToLower(content), "expert") {
		report.ImprovementWarnings = append(report.ImprovementWarnings, "材料中出现强能力表述，投递前需要确认是否有足够证据支撑")
	}
	return Encode(report)
}
