package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"sort"

	"careerpilot-agent/internal/domain"

	_ "modernc.org/sqlite"
)

type SQLiteStore struct {
	db *sql.DB
}

func NewSQLiteStore(ctx context.Context, path string) (*SQLiteStore, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	store := &SQLiteStore{db: db}
	if err := store.migrate(ctx); err != nil {
		_ = db.Close()
		return nil, err
	}
	return store, nil
}

func (s *SQLiteStore) Close() error {
	return s.db.Close()
}

func (s *SQLiteStore) SaveRun(run domain.Run) error {
	ctx := context.Background()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()

	inputJSON, err := marshal(run.Input)
	if err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `
INSERT INTO runs (id, input_json, company_name, job_title, target_role, jd_text, status, created_at, updated_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(id) DO UPDATE SET
  input_json = excluded.input_json,
  company_name = excluded.company_name,
  job_title = excluded.job_title,
  target_role = excluded.target_role,
  jd_text = excluded.jd_text,
  status = excluded.status,
  updated_at = excluded.updated_at
`, run.ID, inputJSON, run.Input.CompanyName, run.Input.JobTitle, run.Input.TargetRole, run.Input.JDText, string(run.Status), run.CreatedAt, run.UpdatedAt); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `DELETE FROM steps WHERE run_id = ?`, run.ID); err != nil {
		return err
	}
	for _, step := range run.Steps {
		input, err := marshal(step.Input)
		if err != nil {
			return err
		}
		output, err := marshal(step.Output)
		if err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `
INSERT INTO steps (id, run_id, type, name, status, input_json, output_json, error, started_at, ended_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
`, step.ID, step.RunID, string(step.Type), step.Name, string(step.Status), input, output, step.Error, step.StartedAt, step.EndedAt); err != nil {
			return err
		}
	}

	if _, err := tx.ExecContext(ctx, `DELETE FROM artifacts WHERE run_id = ?`, run.ID); err != nil {
		return err
	}
	for _, artifact := range run.Artifacts {
		metadata, err := marshal(artifact.Metadata)
		if err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `
INSERT INTO artifacts (id, run_id, type, title, content, metadata_json, created_at)
VALUES (?, ?, ?, ?, ?, ?, ?)
`, artifact.ID, artifact.RunID, artifact.Type, artifact.Title, artifact.Content, metadata, artifact.CreatedAt); err != nil {
			return err
		}
	}

	if err := tx.Commit(); err != nil {
		return err
	}
	committed = true
	return nil
}

func (s *SQLiteStore) GetRun(id string) (domain.Run, error) {
	row := s.db.QueryRowContext(context.Background(), `
SELECT id, input_json, status, created_at, updated_at
FROM runs
WHERE id = ?
`, id)
	run, err := scanRun(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.Run{}, ErrRunNotFound
		}
		return domain.Run{}, err
	}

	steps, err := s.listSteps(run.ID)
	if err != nil {
		return domain.Run{}, err
	}
	artifacts, err := s.listArtifacts(run.ID)
	if err != nil {
		return domain.Run{}, err
	}
	run.Steps = steps
	run.Artifacts = artifacts
	return run, nil
}

func (s *SQLiteStore) ListRuns() ([]domain.Run, error) {
	rows, err := s.db.QueryContext(context.Background(), `
SELECT id, input_json, status, created_at, updated_at
FROM runs
ORDER BY created_at DESC
`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var runs []domain.Run
	for rows.Next() {
		run, err := scanRun(rows)
		if err != nil {
			return nil, err
		}
		runs = append(runs, run)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	sort.Slice(runs, func(i, j int) bool {
		return runs[i].CreatedAt.After(runs[j].CreatedAt)
	})
	return runs, nil
}

func (s *SQLiteStore) migrate(ctx context.Context) error {
	_, err := s.db.ExecContext(ctx, `
PRAGMA journal_mode = WAL;
PRAGMA foreign_keys = ON;

CREATE TABLE IF NOT EXISTS runs (
  id TEXT PRIMARY KEY,
  input_json TEXT NOT NULL,
  company_name TEXT NOT NULL,
  job_title TEXT NOT NULL,
  target_role TEXT NOT NULL,
  jd_text TEXT NOT NULL,
  status TEXT NOT NULL,
  created_at DATETIME NOT NULL,
  updated_at DATETIME NOT NULL
);

CREATE TABLE IF NOT EXISTS steps (
  id TEXT PRIMARY KEY,
  run_id TEXT NOT NULL,
  type TEXT NOT NULL,
  name TEXT NOT NULL,
  status TEXT NOT NULL,
  input_json TEXT,
  output_json TEXT,
  error TEXT,
  started_at DATETIME NOT NULL,
  ended_at DATETIME NOT NULL,
  FOREIGN KEY(run_id) REFERENCES runs(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_steps_run_id ON steps(run_id);

CREATE TABLE IF NOT EXISTS artifacts (
  id TEXT PRIMARY KEY,
  run_id TEXT NOT NULL,
  type TEXT NOT NULL,
  title TEXT NOT NULL,
  content TEXT NOT NULL,
  metadata_json TEXT,
  created_at DATETIME NOT NULL,
  FOREIGN KEY(run_id) REFERENCES runs(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_artifacts_run_id ON artifacts(run_id);
`)
	return err
}

func (s *SQLiteStore) listSteps(runID string) ([]domain.Step, error) {
	rows, err := s.db.QueryContext(context.Background(), `
SELECT id, run_id, type, name, status, input_json, output_json, error, started_at, ended_at
FROM steps
WHERE run_id = ?
ORDER BY started_at ASC
`, runID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var steps []domain.Step
	for rows.Next() {
		var step domain.Step
		var stepType string
		var status string
		var inputJSON sql.NullString
		var outputJSON sql.NullString
		if err := rows.Scan(&step.ID, &step.RunID, &stepType, &step.Name, &status, &inputJSON, &outputJSON, &step.Error, &step.StartedAt, &step.EndedAt); err != nil {
			return nil, err
		}
		step.Type = domain.StepType(stepType)
		step.Status = domain.StepStatus(status)
		step.Input = unmarshalAny(inputJSON.String)
		step.Output = unmarshalAny(outputJSON.String)
		steps = append(steps, step)
	}
	return steps, rows.Err()
}

func (s *SQLiteStore) listArtifacts(runID string) ([]domain.Artifact, error) {
	rows, err := s.db.QueryContext(context.Background(), `
SELECT id, run_id, type, title, content, metadata_json, created_at
FROM artifacts
WHERE run_id = ?
ORDER BY created_at ASC
`, runID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var artifacts []domain.Artifact
	for rows.Next() {
		var artifact domain.Artifact
		var metadataJSON sql.NullString
		if err := rows.Scan(&artifact.ID, &artifact.RunID, &artifact.Type, &artifact.Title, &artifact.Content, &metadataJSON, &artifact.CreatedAt); err != nil {
			return nil, err
		}
		artifact.Metadata = unmarshalAny(metadataJSON.String)
		artifacts = append(artifacts, artifact)
	}
	return artifacts, rows.Err()
}

type runScanner interface {
	Scan(dest ...any) error
}

func scanRun(scanner runScanner) (domain.Run, error) {
	var run domain.Run
	var inputJSON string
	var status string
	if err := scanner.Scan(&run.ID, &inputJSON, &status, &run.CreatedAt, &run.UpdatedAt); err != nil {
		return domain.Run{}, err
	}
	if err := json.Unmarshal([]byte(inputJSON), &run.Input); err != nil {
		return domain.Run{}, fmt.Errorf("decode run input: %w", err)
	}
	run.Status = domain.RunStatus(status)
	return run, nil
}

func marshal(value any) (string, error) {
	if value == nil {
		return "null", nil
	}
	payload, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	return string(payload), nil
}

func unmarshalAny(raw string) any {
	if raw == "" || raw == "null" {
		return nil
	}
	var value any
	if err := json.Unmarshal([]byte(raw), &value); err != nil {
		return raw
	}
	return value
}

var _ RunStore = (*SQLiteStore)(nil)
var _ RunStore = (*Store)(nil)
