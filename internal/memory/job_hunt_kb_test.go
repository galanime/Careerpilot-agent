package memory_test

import (
	"os"
	"path/filepath"
	"testing"

	"careerpilot-agent/internal/memory"
)

func TestLoadJobHuntProjects(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "projects.yaml")
	content := `projects:
  - id: "project_lifehelper"
    canonical_name: "LifeHelper"
    tags:
      - "python"
      - "fastapi"
    strengths:
      - "意图识别与任务拆解"
    tech_stack:
      - "React"
    form_variants:
      project_300char: "独立恢复并完善 LifeHelper 全栈 Agent 项目。"
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	items, err := memory.LoadJobHuntProjects(path)
	if err != nil {
		t.Fatalf("LoadJobHuntProjects returned error: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %#v", items)
	}
	if items[0].Project != "LifeHelper" {
		t.Fatalf("unexpected project: %#v", items[0])
	}
	if len(items[0].Tags) == 0 || len(items[0].Details) == 0 {
		t.Fatalf("expected tags and details: %#v", items[0])
	}
}
