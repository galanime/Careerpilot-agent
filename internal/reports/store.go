package reports

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"careerpilot-agent/internal/domain"
)

type Store struct {
	reportsDir       string
	applicationsPath string
	now              func() time.Time
}

type SavedReport struct {
	ReportPath      string `json:"report_path"`
	ApplicationPath string `json:"application_path"`
	ApplicationRow  string `json:"application_row"`
}

func NewStore(reportsDir string, applicationsPath string) *Store {
	return &Store{reportsDir: reportsDir, applicationsPath: applicationsPath, now: time.Now}
}

func (s *Store) SaveRunReport(run domain.Run, opportunity domain.Opportunity) (SavedReport, error) {
	report := artifactContent(run, "ag_report")
	if strings.TrimSpace(report) == "" {
		return SavedReport{}, fmt.Errorf("run %s has no ag_report artifact", run.ID)
	}
	if err := os.MkdirAll(s.reportsDir, 0o755); err != nil {
		return SavedReport{}, err
	}
	name := fmt.Sprintf("%s-%s-%s.md", s.now().UTC().Format("20060102-150405"), slug(opportunity.CompanyName), slug(opportunity.JobTitle))
	reportPath := filepath.Join(s.reportsDir, name)
	if err := os.WriteFile(reportPath, []byte(report), 0o644); err != nil {
		return SavedReport{}, err
	}
	row, err := s.appendApplication(run, opportunity, reportPath)
	if err != nil {
		return SavedReport{}, err
	}
	return SavedReport{ReportPath: reportPath, ApplicationPath: s.applicationsPath, ApplicationRow: row}, nil
}

func (s *Store) appendApplication(run domain.Run, opportunity domain.Opportunity, reportPath string) (string, error) {
	if s.applicationsPath == "" {
		return "", nil
	}
	if err := os.MkdirAll(filepath.Dir(s.applicationsPath), 0o755); err != nil {
		return "", err
	}
	if _, err := os.Stat(s.applicationsPath); os.IsNotExist(err) {
		header := "# Applications Tracker\n\n| Date | Company | Role | Score | Decision | Status | Run | Report | Notes |\n| --- | --- | --- | --- | --- | --- | --- | --- | --- |\n"
		if err := os.WriteFile(s.applicationsPath, []byte(header), 0o644); err != nil {
			return "", err
		}
	}
	score := ""
	if opportunity.Score > 0 {
		score = fmt.Sprintf("%.1f", opportunity.Score)
	}
	row := fmt.Sprintf("| %s | %s | %s | %s | %s | %s | `%s` | %s | %s |\n",
		s.now().UTC().Format("2006-01-02"),
		escapeCell(opportunity.CompanyName),
		escapeCell(opportunity.JobTitle),
		score,
		escapeCell(opportunity.Decision),
		escapeCell(string(opportunity.Status)),
		run.ID,
		filepath.ToSlash(reportPath),
		escapeCell(strings.Join(opportunity.Notes, "; ")),
	)
	file, err := os.OpenFile(s.applicationsPath, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return "", err
	}
	defer file.Close()
	_, err = file.WriteString(row)
	return row, err
}

func artifactContent(run domain.Run, artifactType string) string {
	for _, artifact := range run.Artifacts {
		if artifact.Type == artifactType {
			return artifact.Content
		}
	}
	return ""
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
	out := strings.Trim(builder.String(), "-")
	if out == "" {
		return "item"
	}
	return out
}

func escapeCell(value string) string {
	value = strings.ReplaceAll(value, "|", "/")
	value = strings.ReplaceAll(value, "\n", " ")
	return value
}
