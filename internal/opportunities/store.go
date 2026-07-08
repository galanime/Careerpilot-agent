package opportunities

import (
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"careerpilot-agent/internal/domain"
)

var ErrOpportunityNotFound = errors.New("opportunity not found")
var ErrDuplicateOpportunity = errors.New("duplicate opportunity")

type Store struct {
	dir          string
	pipelinePath string
	now          func() time.Time
}

func NewStore(dir string, pipelinePath string) *Store {
	return &Store{dir: dir, pipelinePath: pipelinePath, now: time.Now}
}

func (s *Store) Add(input domain.OpportunityInput) (domain.Opportunity, error) {
	if strings.TrimSpace(input.JDText) == "" {
		return domain.Opportunity{}, fmt.Errorf("jd_text is required")
	}
	if strings.TrimSpace(input.CompanyName) == "" {
		return domain.Opportunity{}, fmt.Errorf("company_name is required")
	}
	if strings.TrimSpace(input.JobTitle) == "" {
		return domain.Opportunity{}, fmt.Errorf("job_title is required")
	}
	if input.Source == "" {
		input.Source = domain.OpportunitySourceManual
	}

	fingerprint := Fingerprint(input.CompanyName, input.JobTitle, input.URL, input.JDText)
	existing, err := s.List()
	if err != nil {
		return domain.Opportunity{}, err
	}
	for _, opportunity := range existing {
		if opportunity.Fingerprint == fingerprint || (input.URL != "" && opportunity.URL == input.URL) {
			return opportunity, ErrDuplicateOpportunity
		}
	}

	now := s.now().UTC()
	opportunity := domain.Opportunity{
		ID:          newOpportunityID(input.CompanyName, input.JobTitle, now),
		CompanyName: strings.TrimSpace(input.CompanyName),
		JobTitle:    strings.TrimSpace(input.JobTitle),
		TargetRole:  strings.TrimSpace(input.TargetRole),
		Location:    strings.TrimSpace(input.Location),
		URL:         strings.TrimSpace(input.URL),
		JDText:      strings.TrimSpace(input.JDText),
		Source:      input.Source,
		Status:      domain.OpportunityStatusNew,
		Fingerprint: fingerprint,
		Notes:       input.Notes,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if err := s.Save(opportunity); err != nil {
		return opportunity, err
	}
	if err := s.AppendPipeline(opportunity); err != nil {
		return opportunity, err
	}
	return opportunity, nil
}

func (s *Store) Save(opportunity domain.Opportunity) error {
	if err := os.MkdirAll(s.dir, 0o755); err != nil {
		return err
	}
	if opportunity.UpdatedAt.IsZero() {
		opportunity.UpdatedAt = s.now().UTC()
	}
	payload, err := json.MarshalIndent(opportunity, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.pathFor(opportunity.ID), append(payload, '\n'), 0o644)
}

func (s *Store) Get(id string) (domain.Opportunity, error) {
	payload, err := os.ReadFile(s.pathFor(id))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return domain.Opportunity{}, ErrOpportunityNotFound
		}
		return domain.Opportunity{}, err
	}
	var opportunity domain.Opportunity
	if err := json.Unmarshal(payload, &opportunity); err != nil {
		return domain.Opportunity{}, err
	}
	return opportunity, nil
}

func (s *Store) List() ([]domain.Opportunity, error) {
	if err := os.MkdirAll(s.dir, 0o755); err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(s.dir)
	if err != nil {
		return nil, err
	}
	opportunities := []domain.Opportunity{}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		payload, err := os.ReadFile(filepath.Join(s.dir, entry.Name()))
		if err != nil {
			return nil, err
		}
		var opportunity domain.Opportunity
		if err := json.Unmarshal(payload, &opportunity); err != nil {
			return nil, err
		}
		opportunities = append(opportunities, opportunity)
	}
	sort.Slice(opportunities, func(i, j int) bool {
		return opportunities[i].CreatedAt.After(opportunities[j].CreatedAt)
	})
	return opportunities, nil
}

func (s *Store) MarkScored(id string, score float64, decision string, runID string) (domain.Opportunity, error) {
	opportunity, err := s.Get(id)
	if err != nil {
		return opportunity, err
	}
	opportunity.Score = score
	opportunity.Decision = decision
	opportunity.ReportRunID = runID
	opportunity.UpdatedAt = s.now().UTC()
	switch decision {
	case string(domain.OpportunityDecisionApply):
		opportunity.Status = domain.OpportunityStatusReview
	case string(domain.OpportunityDecisionDoNotApply):
		opportunity.Status = domain.OpportunityStatusDoNotApply
	default:
		opportunity.Status = domain.OpportunityStatusScored
	}
	return opportunity, s.Save(opportunity)
}

func (s *Store) UpdateStatus(id string, status domain.OpportunityStatus, notes []string) (domain.Opportunity, error) {
	opportunity, err := s.Get(id)
	if err != nil {
		return opportunity, err
	}
	opportunity.Status = status
	if len(notes) > 0 {
		opportunity.Notes = append(opportunity.Notes, notes...)
	}
	opportunity.UpdatedAt = s.now().UTC()
	return opportunity, s.Save(opportunity)
}

func (s *Store) AppendPipeline(opportunity domain.Opportunity) error {
	if s.pipelinePath == "" {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(s.pipelinePath), 0o755); err != nil {
		return err
	}
	if _, err := os.Stat(s.pipelinePath); errors.Is(err, os.ErrNotExist) {
		header := "# CareerPilot Pipeline\n\n## Pending\n\n"
		if err := os.WriteFile(s.pipelinePath, []byte(header), 0o644); err != nil {
			return err
		}
	}
	line := fmt.Sprintf("- [ ] %s | %s | %s | %s\n", opportunity.ID, opportunity.CompanyName, opportunity.JobTitle, sourceRef(opportunity))
	file, err := os.OpenFile(s.pipelinePath, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer file.Close()
	_, err = file.WriteString(line)
	return err
}

func (s *Store) pathFor(id string) string {
	return filepath.Join(s.dir, id+".json")
}

func sourceRef(opportunity domain.Opportunity) string {
	if opportunity.URL != "" {
		return opportunity.URL
	}
	return "local:opportunities/" + opportunity.ID + ".json"
}

func Fingerprint(company string, title string, url string, jdText string) string {
	normalized := strings.ToLower(strings.Join([]string{
		strings.TrimSpace(company),
		strings.TrimSpace(title),
		strings.TrimSpace(url),
		compactText(jdText),
	}, "\n"))
	sum := sha1.Sum([]byte(normalized))
	return hex.EncodeToString(sum[:])
}

func compactText(value string) string {
	fields := strings.Fields(value)
	if len(fields) > 200 {
		fields = fields[:200]
	}
	return strings.Join(fields, " ")
}

func newOpportunityID(company string, title string, now time.Time) string {
	base := slug(company + "-" + title)
	if base == "" {
		base = "opportunity"
	}
	stamp := now.Format("20060102-150405")
	hash := sha1.Sum([]byte(base + stamp))
	return fmt.Sprintf("%s-%s", stamp, hex.EncodeToString(hash[:])[:8])
}

func slug(value string) string {
	value = strings.ToLower(value)
	var builder strings.Builder
	lastDash := false
	for _, r := range value {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			builder.WriteRune(r)
			lastDash = false
			continue
		}
		if !lastDash {
			builder.WriteRune('-')
			lastDash = true
		}
	}
	return strings.Trim(builder.String(), "-")
}
