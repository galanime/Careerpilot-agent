package scanner

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"careerpilot-agent/internal/domain"
	"careerpilot-agent/internal/opportunities"
)

type Scanner struct {
	client *http.Client
	store  *opportunities.Store
	now    func() time.Time
}

func New(store *opportunities.Store) *Scanner {
	return &Scanner{
		client: &http.Client{Timeout: 12 * time.Second},
		store:  store,
		now:    time.Now,
	}
}

func (s *Scanner) Scan(ctx context.Context, request domain.ScanRequest) domain.ScanResult {
	result := domain.ScanResult{CreatedAt: s.now().UTC()}
	for _, company := range request.Companies {
		if !company.Enabled {
			continue
		}
		jobs, err := s.fetchCompanyJobs(ctx, company)
		if err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("%s: %v", company.Name, err))
			continue
		}
		for _, job := range jobs {
			result.Found++
			if !titleAllowed(job.JobTitle, request.TitleAllow, request.TitleBlock) {
				job.Skipped = "title_filter"
				result.Filtered++
				result.Jobs = append(result.Jobs, job)
				continue
			}
			liveness := s.CheckLiveness(ctx, job.URL)
			job.Liveness = liveness
			if liveness.Status == domain.LivenessClosed {
				job.Skipped = "closed"
				result.Closed++
				result.Jobs = append(result.Jobs, job)
				continue
			}
			opportunity, err := s.store.Add(domain.OpportunityInput{
				CompanyName: job.CompanyName,
				JobTitle:    job.JobTitle,
				TargetRole:  request.TargetRole,
				Location:    job.Location,
				URL:         job.URL,
				JDText:      fallbackJD(job),
				Source:      domain.OpportunitySourceScan,
				Notes:       []string{"imported_by_scanner", "provider=" + string(job.Provider)},
			})
			if err != nil {
				if errors.Is(err, opportunities.ErrDuplicateOpportunity) {
					job.Skipped = "duplicate"
					result.Duplicates++
					result.Jobs = append(result.Jobs, job)
					continue
				}
				job.Skipped = "add_error"
				result.Errors = append(result.Errors, fmt.Sprintf("%s %s: %v", job.CompanyName, job.JobTitle, err))
				result.Jobs = append(result.Jobs, job)
				continue
			}
			job.Added = true
			job.Opportunity = &opportunity
			result.Added++
			result.Jobs = append(result.Jobs, job)
		}
	}
	return result
}

func (s *Scanner) CheckLiveness(ctx context.Context, rawURL string) domain.LivenessReport {
	report := domain.LivenessReport{URL: rawURL, Status: domain.LivenessUnknown, CheckedAt: s.now().UTC()}
	if strings.TrimSpace(rawURL) == "" {
		report.Status = domain.LivenessUnknown
		report.Signals = append(report.Signals, "missing_url")
		return report
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		report.Status = domain.LivenessError
		report.Error = err.Error()
		return report
	}
	req.Header.Set("User-Agent", "CareerPilotAgent/0.1 personal job scanner")
	resp, err := s.client.Do(req)
	if err != nil {
		report.Status = domain.LivenessError
		report.Error = err.Error()
		return report
	}
	defer resp.Body.Close()

	report.HTTPStatus = resp.StatusCode
	report.FinalURL = resp.Request.URL.String()
	limited := io.LimitReader(resp.Body, 256*1024)
	body, _ := io.ReadAll(limited)
	text := strings.ToLower(stripHTML(string(body)))

	if resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusGone {
		report.Status = domain.LivenessClosed
		report.Signals = append(report.Signals, fmt.Sprintf("http_%d", resp.StatusCode))
		return report
	}
	closedMarkers := []string{"no longer available", "no longer accepting", "job has expired", "position has been filled", "this job is closed", "page not found", "职位已关闭", "招聘已结束"}
	for _, marker := range closedMarkers {
		if strings.Contains(text, marker) {
			report.Status = domain.LivenessClosed
			report.Signals = append(report.Signals, marker)
			return report
		}
	}
	activeMarkers := []string{"apply", "submit application", "job description", "responsibilities", "requirements", "申请", "岗位职责", "任职要求"}
	for _, marker := range activeMarkers {
		if strings.Contains(text, marker) {
			report.Status = domain.LivenessActive
			report.Signals = append(report.Signals, marker)
			return report
		}
	}
	if resp.StatusCode >= 200 && resp.StatusCode < 400 {
		report.Status = domain.LivenessUnknown
		report.Signals = append(report.Signals, "http_ok_without_clear_job_signal")
		return report
	}
	report.Status = domain.LivenessError
	report.Signals = append(report.Signals, fmt.Sprintf("http_%d", resp.StatusCode))
	return report
}

func (s *Scanner) fetchCompanyJobs(ctx context.Context, company domain.TrackedCompany) ([]domain.ScannedJob, error) {
	switch company.Provider {
	case domain.ATSProviderGreenhouse:
		return s.fetchGreenhouse(ctx, company)
	case domain.ATSProviderLever:
		return s.fetchLever(ctx, company)
	case domain.ATSProviderAshby:
		return s.fetchAshby(ctx, company)
	case domain.ATSProviderDirect:
		return s.fetchDirect(ctx, company)
	default:
		return nil, fmt.Errorf("unsupported provider %q", company.Provider)
	}
}

func (s *Scanner) fetchGreenhouse(ctx context.Context, company domain.TrackedCompany) ([]domain.ScannedJob, error) {
	endpoint := fmt.Sprintf("https://boards-api.greenhouse.io/v1/boards/%s/jobs?content=true", url.PathEscape(company.Slug))
	var payload struct {
		Jobs []struct {
			Title       string `json:"title"`
			AbsoluteURL string `json:"absolute_url"`
			Content     string `json:"content"`
			Location    struct {
				Name string `json:"name"`
			} `json:"location"`
		} `json:"jobs"`
	}
	if err := s.getJSON(ctx, endpoint, &payload); err != nil {
		return nil, err
	}
	jobs := make([]domain.ScannedJob, 0, len(payload.Jobs))
	for _, item := range payload.Jobs {
		jobs = append(jobs, domain.ScannedJob{
			CompanyName: company.Name,
			JobTitle:    item.Title,
			Location:    item.Location.Name,
			URL:         item.AbsoluteURL,
			JDText:      stripHTML(item.Content),
			Provider:    company.Provider,
		})
	}
	return jobs, nil
}

func (s *Scanner) fetchLever(ctx context.Context, company domain.TrackedCompany) ([]domain.ScannedJob, error) {
	endpoint := fmt.Sprintf("https://api.lever.co/v0/postings/%s?mode=json", url.PathEscape(company.Slug))
	var payload []struct {
		Text        string `json:"text"`
		HostedURL   string `json:"hostedUrl"`
		Description string `json:"descriptionPlain"`
		Categories  struct {
			Location string `json:"location"`
		} `json:"categories"`
	}
	if err := s.getJSON(ctx, endpoint, &payload); err != nil {
		return nil, err
	}
	jobs := make([]domain.ScannedJob, 0, len(payload))
	for _, item := range payload {
		jobs = append(jobs, domain.ScannedJob{
			CompanyName: company.Name,
			JobTitle:    item.Text,
			Location:    item.Categories.Location,
			URL:         item.HostedURL,
			JDText:      item.Description,
			Provider:    company.Provider,
		})
	}
	return jobs, nil
}

func (s *Scanner) fetchAshby(ctx context.Context, company domain.TrackedCompany) ([]domain.ScannedJob, error) {
	endpoint := fmt.Sprintf("https://api.ashbyhq.com/posting-api/job-board/%s?includeCompensation=true", url.PathEscape(company.Slug))
	var payload struct {
		Jobs []struct {
			Title    string `json:"title"`
			JobURL   string `json:"jobUrl"`
			Location string `json:"location"`
		} `json:"jobs"`
	}
	if err := s.getJSON(ctx, endpoint, &payload); err != nil {
		return nil, err
	}
	jobs := make([]domain.ScannedJob, 0, len(payload.Jobs))
	for _, item := range payload.Jobs {
		jobs = append(jobs, domain.ScannedJob{
			CompanyName: company.Name,
			JobTitle:    item.Title,
			Location:    item.Location,
			URL:         item.JobURL,
			JDText:      item.Title + "\n" + item.Location,
			Provider:    company.Provider,
		})
	}
	return jobs, nil
}

func (s *Scanner) fetchDirect(ctx context.Context, company domain.TrackedCompany) ([]domain.ScannedJob, error) {
	if company.CareersURL == "" {
		return nil, fmt.Errorf("direct provider requires careers_url")
	}
	liveness := s.CheckLiveness(ctx, company.CareersURL)
	title := company.Name + " Careers"
	return []domain.ScannedJob{{
		CompanyName: company.Name,
		JobTitle:    title,
		Location:    "",
		URL:         company.CareersURL,
		JDText:      title,
		Provider:    company.Provider,
		Liveness:    liveness,
	}}, nil
}

func (s *Scanner) getJSON(ctx context.Context, endpoint string, target any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "CareerPilotAgent/0.1 personal job scanner")
	resp, err := s.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("GET %s returned %d", endpoint, resp.StatusCode)
	}
	return json.NewDecoder(resp.Body).Decode(target)
}

func titleAllowed(title string, allow []string, block []string) bool {
	lower := strings.ToLower(title)
	for _, blocked := range block {
		if blocked != "" && strings.Contains(lower, strings.ToLower(blocked)) {
			return false
		}
	}
	if len(allow) == 0 {
		return true
	}
	for _, allowed := range allow {
		if allowed != "" && strings.Contains(lower, strings.ToLower(allowed)) {
			return true
		}
	}
	return false
}

func fallbackJD(job domain.ScannedJob) string {
	if strings.TrimSpace(job.JDText) != "" {
		return job.JDText
	}
	return strings.Join([]string{job.CompanyName, job.JobTitle, job.Location, job.URL}, "\n")
}

func stripHTML(value string) string {
	value = html.UnescapeString(value)
	re := regexp.MustCompile(`<[^>]+>`)
	value = re.ReplaceAllString(value, " ")
	return strings.Join(strings.Fields(value), " ")
}
