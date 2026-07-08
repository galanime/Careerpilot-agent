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
	CompanyName   string            `json:"company_name"`
	JobTitle      string            `json:"job_title"`
	TargetRole    string            `json:"target_role"`
	JDText        string            `json:"jd_text"`
	OpportunityID string            `json:"opportunity_id,omitempty"`
	Preferences   map[string]string `json:"preferences,omitempty"`
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

type OpportunityDecision string

const (
	OpportunityDecisionApply       OpportunityDecision = "apply"
	OpportunityDecisionReviewFirst OpportunityDecision = "review_first"
	OpportunityDecisionDoNotApply  OpportunityDecision = "do_not_apply"
)

type OpportunityDimension struct {
	Name      string `json:"name"`
	Score     int    `json:"score"`
	Weight    int    `json:"weight"`
	Rationale string `json:"rationale"`
}

type OpportunityEvaluation struct {
	OverallScore        float64                `json:"overall_score"`
	Decision            OpportunityDecision    `json:"decision"`
	Dimensions          []OpportunityDimension `json:"dimensions"`
	DealBreakers        []string               `json:"deal_breakers"`
	RecommendedActions  []string               `json:"recommended_actions"`
	ResponsibleUseNotes []string               `json:"responsible_use_notes"`
}

type GeneratedMaterials struct {
	ResumeBullets []string `json:"resume_bullets"`
	Pitch         string   `json:"pitch"`
	InterviewQA   []string `json:"interview_qa"`
}

type ApplicationPlan struct {
	Status              string   `json:"status"`
	NextActions         []string `json:"next_actions"`
	TrackerFields       []string `json:"tracker_fields"`
	CoverLetterAngles   []string `json:"cover_letter_angles"`
	InterviewPrepCards  []string `json:"interview_prep_cards"`
	SubmissionGuardrail []string `json:"submission_guardrail"`
}

type ApplicationDossier struct {
	ReportMarkdown     string   `json:"report_markdown"`
	CoverLetterDraft   string   `json:"cover_letter_draft"`
	EmailDraft         string   `json:"email_draft"`
	OpenQuestionDrafts []string `json:"open_question_drafts"`
	ATSChecklist       []string `json:"ats_checklist"`
}

type EvalReport struct {
	Passed              bool     `json:"passed"`
	KeywordCoverage     float64  `json:"keyword_coverage"`
	UnsupportedClaims   []string `json:"unsupported_claims"`
	ImprovementWarnings []string `json:"improvement_warnings"`
}

type OpportunityStatus string

const (
	OpportunityStatusNew        OpportunityStatus = "new"
	OpportunityStatusScored     OpportunityStatus = "scored"
	OpportunityStatusReview     OpportunityStatus = "ready_for_review"
	OpportunityStatusApplied    OpportunityStatus = "applied"
	OpportunityStatusInterview  OpportunityStatus = "interview"
	OpportunityStatusRejected   OpportunityStatus = "rejected"
	OpportunityStatusArchived   OpportunityStatus = "archived"
	OpportunityStatusDoNotApply OpportunityStatus = "do_not_apply"
)

type OpportunitySource string

const (
	OpportunitySourceManual OpportunitySource = "manual"
	OpportunitySourceScan   OpportunitySource = "scan"
	OpportunitySourceImport OpportunitySource = "import"
)

type Opportunity struct {
	ID          string            `json:"id"`
	CompanyName string            `json:"company_name"`
	JobTitle    string            `json:"job_title"`
	TargetRole  string            `json:"target_role"`
	Location    string            `json:"location,omitempty"`
	URL         string            `json:"url,omitempty"`
	JDText      string            `json:"jd_text"`
	Source      OpportunitySource `json:"source"`
	Status      OpportunityStatus `json:"status"`
	Fingerprint string            `json:"fingerprint"`
	Score       float64           `json:"score,omitempty"`
	Decision    string            `json:"decision,omitempty"`
	ReportRunID string            `json:"report_run_id,omitempty"`
	Notes       []string          `json:"notes,omitempty"`
	CreatedAt   time.Time         `json:"created_at"`
	UpdatedAt   time.Time         `json:"updated_at"`
}

type OpportunityInput struct {
	CompanyName string            `json:"company_name"`
	JobTitle    string            `json:"job_title"`
	TargetRole  string            `json:"target_role"`
	Location    string            `json:"location,omitempty"`
	URL         string            `json:"url,omitempty"`
	JDText      string            `json:"jd_text"`
	Source      OpportunitySource `json:"source,omitempty"`
	Notes       []string          `json:"notes,omitempty"`
}

type ATSProvider string

const (
	ATSProviderGreenhouse ATSProvider = "greenhouse"
	ATSProviderLever      ATSProvider = "lever"
	ATSProviderAshby      ATSProvider = "ashby"
	ATSProviderDirect     ATSProvider = "direct"
)

type TrackedCompany struct {
	Name       string      `json:"name"`
	Provider   ATSProvider `json:"provider"`
	Slug       string      `json:"slug,omitempty"`
	CareersURL string      `json:"careers_url,omitempty"`
	Enabled    bool        `json:"enabled"`
}

type ScanRequest struct {
	TargetRole string           `json:"target_role,omitempty"`
	TitleAllow []string         `json:"title_allow,omitempty"`
	TitleBlock []string         `json:"title_block,omitempty"`
	Companies  []TrackedCompany `json:"companies"`
}

type LivenessStatus string

const (
	LivenessActive  LivenessStatus = "active"
	LivenessClosed  LivenessStatus = "closed"
	LivenessUnknown LivenessStatus = "unknown"
	LivenessError   LivenessStatus = "error"
)

type LivenessReport struct {
	URL        string         `json:"url"`
	Status     LivenessStatus `json:"status"`
	HTTPStatus int            `json:"http_status,omitempty"`
	FinalURL   string         `json:"final_url,omitempty"`
	Signals    []string       `json:"signals"`
	Error      string         `json:"error,omitempty"`
	CheckedAt  time.Time      `json:"checked_at"`
}

type ScannedJob struct {
	CompanyName string         `json:"company_name"`
	JobTitle    string         `json:"job_title"`
	Location    string         `json:"location,omitempty"`
	URL         string         `json:"url"`
	JDText      string         `json:"jd_text,omitempty"`
	Provider    ATSProvider    `json:"provider"`
	Liveness    LivenessReport `json:"liveness"`
	Added       bool           `json:"added"`
	Skipped     string         `json:"skipped,omitempty"`
	Opportunity *Opportunity   `json:"opportunity,omitempty"`
}

type ScanResult struct {
	Found      int          `json:"found"`
	Added      int          `json:"added"`
	Duplicates int          `json:"duplicates"`
	Filtered   int          `json:"filtered"`
	Closed     int          `json:"closed"`
	Errors     []string     `json:"errors"`
	Jobs       []ScannedJob `json:"jobs"`
	CreatedAt  time.Time    `json:"created_at"`
}
