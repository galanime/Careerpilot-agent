package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"careerpilot-agent/internal/domain"
	"careerpilot-agent/internal/storage"
	"careerpilot-agent/internal/tools"
)

type Runtime struct {
	store    storage.RunStore
	registry *tools.Registry
	events   *EventHub
	now      func() time.Time
}

func NewRuntime(store storage.RunStore, registry *tools.Registry) *Runtime {
	return &Runtime{store: store, registry: registry, events: NewEventHub(), now: time.Now}
}

func (r *Runtime) StartRun(ctx context.Context, input domain.RunInput) (domain.Run, error) {
	return r.ExecuteRun(ctx, input)
}

func (r *Runtime) CreateRun(ctx context.Context, input domain.RunInput) (domain.Run, error) {
	now := r.now()
	run := domain.Run{
		ID:        newID("run"),
		Input:     input,
		Status:    domain.RunStatusCreated,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := r.store.SaveRun(run); err != nil {
		return run, err
	}
	r.publish(Event{Type: EventRunCreated, RunID: run.ID, RunStatus: run.Status})
	return run, nil
}

func (r *Runtime) StartRunAsync(ctx context.Context, input domain.RunInput) (domain.Run, error) {
	run, err := r.CreateRun(ctx, input)
	if err != nil {
		return run, err
	}
	go func(run domain.Run) {
		_, _ = r.ResumeRun(context.Background(), run.ID)
	}(run)
	return run, nil
}

func (r *Runtime) ResumeRun(ctx context.Context, runID string) (domain.Run, error) {
	run, err := r.store.GetRun(runID)
	if err != nil {
		return run, err
	}
	run.Status = domain.RunStatusRunning
	run.UpdatedAt = r.now()
	if err := r.saveRun(run); err != nil {
		return run, err
	}
	r.publish(Event{Type: EventRunStarted, RunID: run.ID, RunStatus: run.Status})

	run, err = r.execute(ctx, run)
	if err != nil {
		run.Status = domain.RunStatusFailed
		run.UpdatedAt = r.now()
		if saveErr := r.saveRun(run); saveErr != nil {
			err = fmt.Errorf("%w; additionally failed to save failed run: %v", err, saveErr)
		}
		r.publish(Event{Type: EventRunFailed, RunID: run.ID, RunStatus: run.Status, Error: err.Error()})
		return run, err
	}

	run.Status = domain.RunStatusDone
	run.UpdatedAt = r.now()
	if err := r.saveRun(run); err != nil {
		return run, err
	}
	r.publish(Event{Type: EventRunFinished, RunID: run.ID, RunStatus: run.Status})
	return run, nil
}

func (r *Runtime) ExecuteRun(ctx context.Context, input domain.RunInput) (domain.Run, error) {
	run, err := r.CreateRun(ctx, input)
	if err != nil {
		return run, err
	}
	return r.ResumeRun(ctx, run.ID)
}

func (r *Runtime) GetRun(ctx context.Context, runID string) (domain.Run, error) {
	return r.store.GetRun(runID)
}

func (r *Runtime) ListRuns(ctx context.Context) ([]domain.Run, error) {
	return r.store.ListRuns()
}

func (r *Runtime) SubscribeEvents(ctx context.Context, runID string) <-chan Event {
	return r.events.Subscribe(ctx, runID)
}

func (r *Runtime) execute(ctx context.Context, run domain.Run) (domain.Run, error) {
	plan := []string{"parse_jd", "search_evidence", "score_match", "evaluate_opportunity", "generate_materials", "evaluate_output", "build_application_plan"}
	run = r.addStep(run, domain.StepTypePlanning, "planner", map[string]any{"goal": "evaluate opportunity, avoid spam applications, and generate evidence-grounded application materials"}, map[string]any{"tools": plan}, "")
	if err := r.saveRun(run); err != nil {
		return run, err
	}
	r.publishStep(run.ID, run.Steps[len(run.Steps)-1])

	analysisStep, rawAnalysis, err := r.executeTool(ctx, run, "parse_jd", tools.ParseJDInput{JDText: run.Input.JDText})
	run.Steps = append(run.Steps, analysisStep)
	if err := r.afterStep(run, analysisStep); err != nil {
		return run, err
	}
	if err != nil {
		return run, err
	}
	var analysis domain.JDAnalysis
	if err := json.Unmarshal(rawAnalysis, &analysis); err != nil {
		return run, err
	}

	evidenceStep, rawEvidence, err := r.executeTool(ctx, run, "search_evidence", tools.SearchEvidenceInput{Keywords: analysis.Keywords, Limit: 5})
	run.Steps = append(run.Steps, evidenceStep)
	if err := r.afterStep(run, evidenceStep); err != nil {
		return run, err
	}
	if err != nil {
		return run, err
	}
	var evidence domain.EvidenceSearchResult
	if err := json.Unmarshal(rawEvidence, &evidence); err != nil {
		return run, err
	}

	matchStep, rawMatch, err := r.executeTool(ctx, run, "score_match", tools.MatchScoringInput{Analysis: analysis, Evidence: evidence})
	run.Steps = append(run.Steps, matchStep)
	if err := r.afterStep(run, matchStep); err != nil {
		return run, err
	}
	if err != nil {
		return run, err
	}
	var match domain.MatchReport
	if err := json.Unmarshal(rawMatch, &match); err != nil {
		return run, err
	}

	opportunityInput := tools.EvaluateOpportunityInput{CompanyName: run.Input.CompanyName, JobTitle: run.Input.JobTitle, TargetRole: run.Input.TargetRole, Analysis: analysis, Evidence: evidence, Match: match}
	opportunityStep, rawOpportunity, err := r.executeTool(ctx, run, "evaluate_opportunity", opportunityInput)
	run.Steps = append(run.Steps, opportunityStep)
	if err := r.afterStep(run, opportunityStep); err != nil {
		return run, err
	}
	if err != nil {
		return run, err
	}
	var opportunity domain.OpportunityEvaluation
	if err := json.Unmarshal(rawOpportunity, &opportunity); err != nil {
		return run, err
	}

	materialsInput := tools.GenerateMaterialsInput{CompanyName: run.Input.CompanyName, JobTitle: run.Input.JobTitle, Analysis: analysis, Evidence: evidence, Match: match}
	materialsStep, rawMaterials, err := r.executeTool(ctx, run, "generate_materials", materialsInput)
	run.Steps = append(run.Steps, materialsStep)
	if err := r.afterStep(run, materialsStep); err != nil {
		return run, err
	}
	if err != nil {
		return run, err
	}
	var materials domain.GeneratedMaterials
	if err := json.Unmarshal(rawMaterials, &materials); err != nil {
		return run, err
	}

	evalStep, rawEval, err := r.executeTool(ctx, run, "evaluate_output", tools.EvaluateOutputInput{Analysis: analysis, Evidence: evidence, Materials: materials})
	run.Steps = append(run.Steps, evalStep)
	if err := r.afterStep(run, evalStep); err != nil {
		return run, err
	}
	if err != nil {
		return run, err
	}
	var evalReport domain.EvalReport
	if err := json.Unmarshal(rawEval, &evalReport); err != nil {
		return run, err
	}

	planInput := tools.BuildApplicationPlanInput{CompanyName: run.Input.CompanyName, JobTitle: run.Input.JobTitle, Analysis: analysis, Evidence: evidence, Match: match, Opportunity: opportunity, Materials: materials}
	planStep, rawPlan, err := r.executeTool(ctx, run, "build_application_plan", planInput)
	run.Steps = append(run.Steps, planStep)
	if err := r.afterStep(run, planStep); err != nil {
		return run, err
	}
	if err != nil {
		return run, err
	}
	var applicationPlan domain.ApplicationPlan
	if err := json.Unmarshal(rawPlan, &applicationPlan); err != nil {
		return run, err
	}

	artifacts := []domain.Artifact{
		r.newArtifact(run.ID, "jd_analysis", "JD 结构化分析", toPrettyJSON(analysis), nil),
		r.newArtifact(run.ID, "match_report", "岗位匹配报告", toPrettyJSON(match), nil),
		r.newArtifact(run.ID, "opportunity_evaluation", "机会评分与投递决策", toPrettyJSON(opportunity), map[string]any{"decision": opportunity.Decision, "score": opportunity.OverallScore}),
		r.newArtifact(run.ID, "application_materials", "投递材料草稿", renderMaterials(materials), map[string]any{"evidence_count": len(evidence.Items)}),
		r.newArtifact(run.ID, "eval_report", "自检评估报告", toPrettyJSON(evalReport), nil),
		r.newArtifact(run.ID, "application_plan", "申请计划与追踪字段", toPrettyJSON(applicationPlan), map[string]any{"status": applicationPlan.Status}),
	}
	for _, artifact := range artifacts {
		run.Artifacts = append(run.Artifacts, artifact)
		if err := r.saveRun(run); err != nil {
			return run, err
		}
		r.publish(Event{Type: EventArtifactCreated, RunID: run.ID, Artifact: &artifact})
	}
	return run, nil
}

func (r *Runtime) executeTool(ctx context.Context, run domain.Run, name string, input any) (domain.Step, json.RawMessage, error) {
	started := r.now()
	step := domain.Step{
		ID:        newID("step"),
		RunID:     run.ID,
		Type:      domain.StepTypeTool,
		Name:      name,
		Status:    domain.StepStatusRunning,
		Input:     input,
		StartedAt: started,
	}
	r.publish(Event{Type: EventStepStarted, RunID: run.ID, Step: &step})
	raw, err := r.registry.Execute(ctx, name, input)
	step.EndedAt = r.now()
	if err != nil {
		step.Status = domain.StepStatusFailed
		step.Error = err.Error()
		return step, nil, err
	}
	var output any
	if err := json.Unmarshal(raw, &output); err != nil {
		step.Status = domain.StepStatusFailed
		step.Error = err.Error()
		return step, nil, err
	}
	step.Output = output
	step.Status = domain.StepStatusDone
	return step, raw, nil
}

func (r *Runtime) afterStep(run domain.Run, step domain.Step) error {
	if err := r.saveRun(run); err != nil {
		return err
	}
	r.publishStep(run.ID, step)
	return nil
}

func (r *Runtime) publishStep(runID string, step domain.Step) {
	eventType := EventStepFinished
	if step.Status == domain.StepStatusFailed {
		eventType = EventStepFailed
	}
	r.publish(Event{Type: eventType, RunID: runID, Step: &step, Error: step.Error})
}

func (r *Runtime) saveRun(run domain.Run) error {
	run.UpdatedAt = r.now()
	return r.store.SaveRun(run)
}

func (r *Runtime) publish(event Event) {
	if event.ID == "" {
		event.ID = newID("event")
	}
	if event.CreatedAt.IsZero() {
		event.CreatedAt = r.now()
	}
	r.events.Publish(event)
}

func (r *Runtime) addStep(run domain.Run, stepType domain.StepType, name string, input any, output any, errText string) domain.Run {
	status := domain.StepStatusDone
	if errText != "" {
		status = domain.StepStatusFailed
	}
	now := r.now()
	run.Steps = append(run.Steps, domain.Step{
		ID:        newID("step"),
		RunID:     run.ID,
		Type:      stepType,
		Name:      name,
		Status:    status,
		Input:     input,
		Output:    output,
		Error:     errText,
		StartedAt: now,
		EndedAt:   now,
	})
	return run
}

func (r *Runtime) newArtifact(runID string, artifactType string, title string, content string, metadata any) domain.Artifact {
	return domain.Artifact{ID: newID("artifact"), RunID: runID, Type: artifactType, Title: title, Content: content, Metadata: metadata, CreatedAt: r.now()}
}

func renderMaterials(materials domain.GeneratedMaterials) string {
	out := "## 简历 Bullet\n"
	for _, bullet := range materials.ResumeBullets {
		out += "- " + bullet + "\n"
	}
	out += "\n## 自我介绍\n" + materials.Pitch + "\n\n## 面试问答\n"
	for _, qa := range materials.InterviewQA {
		out += "- " + qa + "\n"
	}
	return out
}

func toPrettyJSON(value any) string {
	payload, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return fmt.Sprintf("%v", value)
	}
	return string(payload)
}
