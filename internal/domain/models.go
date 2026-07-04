package domain

import "time"

type RunStatus string

const (
	RunStatusCreated RunStatus = "created"
	RunStatusRunning RunStatus = "running"
	RunStatusDone    RunStatus = "done"
	RunStatusFailed  RunStatus = "failed"
)

type StepStatus string

const (
	StepStatusPending StepStatus = "pending"
	StepStatusRunning StepStatus = "running"
	StepStatusDone    StepStatus = "done"
	StepStatusFailed  StepStatus = "failed"
)

type StepType string

const (
	StepTypePlanning StepType = "planning"
	StepTypeTool     StepType = "tool_call"
	StepTypeArtifact StepType = "artifact"
	StepTypeEval     StepType = "evaluation"
)

type RunInput struct {
	CompanyName string            `json:"company_name"`
	JobTitle    string            `json:"job_title"`
	TargetRole  string            `json:"target_role"`
	JDText      string            `json:"jd_text"`
	Preferences map[string]string `json:"preferences,omitempty"`
}

type Run struct {
	ID        string     `json:"id"`
	Input     RunInput   `json:"input"`
	Status    RunStatus  `json:"status"`
	Steps     []Step     `json:"steps"`
	Artifacts []Artifact `json:"artifacts"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
}

type Step struct {
	ID        string     `json:"id"`
	RunID     string     `json:"run_id"`
	Type      StepType   `json:"type"`
	Name      string     `json:"name"`
	Status    StepStatus `json:"status"`
	Input     any        `json:"input,omitempty"`
	Output    any        `json:"output,omitempty"`
	Error     string     `json:"error,omitempty"`
	StartedAt time.Time  `json:"started_at"`
	EndedAt   time.Time  `json:"ended_at"`
}

type Artifact struct {
	ID        string    `json:"id"`
	RunID     string    `json:"run_id"`
	Type      string    `json:"type"`
	Title     string    `json:"title"`
	Content   string    `json:"content"`
	Metadata  any       `json:"metadata,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

type JDAnalysis struct {
	Responsibilities []string `json:"responsibilities"`
	RequiredSkills   []string `json:"required_skills"`
	BonusSkills      []string `json:"bonus_skills"`
	Keywords         []string `json:"keywords"`
}

type EvidenceItem struct {
	Project string   `json:"project"`
	Title   string   `json:"title"`
	Tags    []string `json:"tags"`
	Details []string `json:"details"`
}

type EvidenceSearchResult struct {
	Query   []string       `json:"query"`
	Items   []EvidenceItem `json:"items"`
	Summary string         `json:"summary"`
}

type MatchReport struct {
	Score          int      `json:"score"`
	StrongMatches  []string `json:"strong_matches"`
	MediumMatches  []string `json:"medium_matches"`
	Gaps           []string `json:"gaps"`
	RecommendedUse []string `json:"recommended_use"`
}

type GeneratedMaterials struct {
	ResumeBullets []string `json:"resume_bullets"`
	Pitch         string   `json:"pitch"`
	InterviewQA   []string `json:"interview_qa"`
}

type EvalReport struct {
	Passed              bool     `json:"passed"`
	KeywordCoverage     float64  `json:"keyword_coverage"`
	UnsupportedClaims   []string `json:"unsupported_claims"`
	ImprovementWarnings []string `json:"improvement_warnings"`
}
