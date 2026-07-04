import type { Dispatch, FormEvent, SetStateAction } from 'react';
import { useEffect, useMemo, useState } from 'react';
import { createRoot } from 'react-dom/client';
import './styles.css';

const API_BASE = 'http://127.0.0.1:8788';

const stepPlan = [
  { name: 'planner', label: 'Planner', detail: '生成固定执行计划' },
  { name: 'parse_jd', label: 'Parse JD', detail: '抽取职责、技能和关键词' },
  { name: 'search_evidence', label: 'Search Evidence', detail: '检索候选人真实项目证据' },
  { name: 'score_match', label: 'Score Match', detail: '计算岗位匹配度和短板' },
  { name: 'generate_materials', label: 'Generate Materials', detail: '生成简历 bullet 与面试材料' },
  { name: 'evaluate_output', label: 'Evaluate Output', detail: '检查覆盖率与无证据风险' },
];

const eventTypes = [
  'run_created',
  'run_started',
  'step_started',
  'step_finished',
  'step_failed',
  'artifact_created',
  'run_finished',
  'run_failed',
] as const;

type RunStatus = 'created' | 'running' | 'done' | 'failed';
type StepStatus = 'pending' | 'running' | 'done' | 'failed';
type StreamState = 'idle' | 'connecting' | 'open' | 'closed' | 'error';

type RunInput = {
  company_name: string;
  job_title: string;
  target_role: string;
  jd_text: string;
};

type Run = {
  id: string;
  input: RunInput;
  status: RunStatus;
  steps: Step[];
  artifacts: Artifact[];
  created_at: string;
  updated_at: string;
};

type Step = {
  id: string;
  run_id: string;
  type: string;
  name: string;
  status: StepStatus;
  input?: unknown;
  output?: unknown;
  error?: string;
  started_at: string;
  ended_at: string;
};

type Artifact = {
  id: string;
  run_id: string;
  type: string;
  title: string;
  content: string;
  metadata?: unknown;
  created_at: string;
};

type AgentEvent = {
  id: string;
  type: (typeof eventTypes)[number];
  run_id: string;
  run_status?: RunStatus;
  step?: Step;
  artifact?: Artifact;
  error?: string;
  created_at: string;
};

type EvalReport = {
  passed: boolean;
  keyword_coverage: number;
  unsupported_claims: string[];
  improvement_warnings: string[];
};

type MatchReport = {
  score: number;
  strong_matches: string[];
  medium_matches: string[];
  gaps: string[];
  recommended_use: string[];
};

const sampleInput: RunInput = {
  company_name: 'ByteDance',
  job_title: 'AI Agent Engineer Intern',
  target_role: 'AI Agent Engineer',
  jd_text:
    '负责 AI Agent 平台开发，要求 Go、RAG、Tool Calling、Evaluation、后端工程、工作流编排和可观测能力。需要能设计 Agent Runtime、接入大模型 Provider，并保证输出可评测、可追踪。',
};

function App() {
  const [form, setForm] = useState<RunInput>(sampleInput);
  const [run, setRun] = useState<Run | null>(null);
  const [runs, setRuns] = useState<Run[]>([]);
  const [events, setEvents] = useState<AgentEvent[]>([]);
  const [selectedArtifactType, setSelectedArtifactType] = useState<string | null>(null);
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [error, setError] = useState('');
  const [streamState, setStreamState] = useState<StreamState>('idle');

  const stepsByName = useMemo(() => new Map((run?.steps ?? []).map((step) => [step.name, step])), [run?.steps]);
  const selectedArtifact = useMemo(
    () => run?.artifacts.find((artifact) => artifact.type === selectedArtifactType) ?? run?.artifacts[0] ?? null,
    [run?.artifacts, selectedArtifactType],
  );
  const evalReport = useMemo(
    () => parseJSON<EvalReport>(run?.artifacts.find((artifact) => artifact.type === 'eval_report')?.content),
    [run?.artifacts],
  );
  const matchReport = useMemo(
    () => parseJSON<MatchReport>(run?.artifacts.find((artifact) => artifact.type === 'match_report')?.content),
    [run?.artifacts],
  );
  const evalWarnings = evalReport?.improvement_warnings ?? [];
  const unsupportedClaims = evalReport?.unsupported_claims ?? [];

  useEffect(() => {
    void refreshRuns();
  }, []);

  useEffect(() => {
    const artifactTypes = run?.artifacts.map((artifact) => artifact.type) ?? [];
    if (artifactTypes.length === 0) {
      setSelectedArtifactType(null);
      return;
    }
    if (!selectedArtifactType || !artifactTypes.includes(selectedArtifactType)) {
      setSelectedArtifactType(artifactTypes[0]);
    }
  }, [run?.artifacts, selectedArtifactType]);

  useEffect(() => {
    if (!run?.id) {
      setStreamState('idle');
      return;
    }
    if (run.status === 'done' || run.status === 'failed') {
      setStreamState('closed');
      return;
    }

    const source = new EventSource(`${API_BASE}/api/runs/${run.id}/events`);
    setStreamState('connecting');

    const handleEvent = (message: Event) => {
      const parsed = JSON.parse((message as MessageEvent<string>).data) as AgentEvent;
      setEvents((current) => upsertEvent(current, parsed));
      setRun((current) => mergeRunEvent(current, parsed));

      if (parsed.type === 'run_finished' || parsed.type === 'run_failed') {
        source.close();
        setStreamState('closed');
        void loadRun(parsed.run_id, { silent: true });
        void refreshRuns();
      }
    };

    eventTypes.forEach((type) => source.addEventListener(type, handleEvent));
    source.onopen = () => setStreamState('open');
    source.onerror = () => {
      if (source.readyState === EventSource.CLOSED) {
        setStreamState('closed');
      } else {
        setStreamState('error');
      }
    };

    return () => {
      eventTypes.forEach((type) => source.removeEventListener(type, handleEvent));
      source.close();
    };
  }, [run?.id]);

  async function refreshRuns() {
    const response = await fetch(`${API_BASE}/api/runs`);
    if (!response.ok) {
      return;
    }
    const payload = (await response.json()) as { runs?: Run[] | null };
    setRuns((payload.runs ?? []).map(normalizeRun));
  }

  async function loadRun(runID: string, options?: { silent?: boolean }) {
    try {
      const response = await fetch(`${API_BASE}/api/runs/${runID}`);
      if (!response.ok) {
        throw new Error(await readAPIError(response));
      }
      const payload = (await response.json()) as Run;
      setRun(normalizeRun(payload));
      if (!options?.silent) {
        setEvents([]);
      }
    } catch (caught) {
      if (!options?.silent) {
        setError(caught instanceof Error ? caught.message : '加载 Run 失败');
      }
    }
  }

  async function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setIsSubmitting(true);
    setError('');
    setEvents([]);
    setSelectedArtifactType(null);

    try {
      const response = await fetch(`${API_BASE}/api/runs`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(form),
      });
      if (!response.ok) {
        throw new Error(await readAPIError(response));
      }
      const payload = (await response.json()) as Run;
      setRun(normalizeRun(payload));
      void refreshRuns();
    } catch (caught) {
      setError(caught instanceof Error ? caught.message : '创建 Run 失败');
      setStreamState('error');
    } finally {
      setIsSubmitting(false);
    }
  }

  return (
    <main className="shell">
      <section className="hero">
        <p className="eyebrow">Go Agent Runtime · Evidence Grounded · Traceable</p>
        <h1>CareerPilot Agent</h1>
        <p>
          面向 AI Agent 岗位投递的作品集 Demo：输入 JD 后，Agent 会解析岗位、检索候选人项目证据、生成匹配报告和投递材料，并保留完整执行 Trace。
        </p>
        <div className="hero-metrics">
          <Metric label="Runtime" value="Go" />
          <Metric label="Tools" value="5" />
          <Metric label="Trace" value="SSE" />
          <Metric label="Store" value="SQLite" />
        </div>
      </section>

      <section className="workspace">
        <form className="panel form-panel" onSubmit={handleSubmit}>
          <div className="section-heading">
            <div>
              <p className="eyebrow dark">Input</p>
              <h2>创建 Agent Run</h2>
            </div>
            <button type="button" className="ghost-button" onClick={() => setForm(sampleInput)}>
              填入示例 JD
            </button>
          </div>

          <div className="form-grid">
            <label>
              公司名
              <input value={form.company_name} onChange={(event) => setFormField('company_name', event.target.value, setForm)} />
            </label>
            <label>
              岗位名
              <input value={form.job_title} onChange={(event) => setFormField('job_title', event.target.value, setForm)} />
            </label>
          </div>
          <label>
            目标方向
            <input value={form.target_role} onChange={(event) => setFormField('target_role', event.target.value, setForm)} />
          </label>
          <label>
            JD 文本
            <textarea value={form.jd_text} onChange={(event) => setFormField('jd_text', event.target.value, setForm)} rows={10} />
          </label>

          {error && <p className="error-banner">{error}</p>}
          <button type="submit" disabled={isSubmitting || form.jd_text.trim().length === 0}>
            {isSubmitting ? '启动中...' : '启动 Agent'}
          </button>
        </form>

        <aside className="panel runs-panel">
          <div className="section-heading">
            <div>
              <p className="eyebrow dark">Runs</p>
              <h2>最近记录</h2>
            </div>
            <button type="button" className="ghost-button" onClick={() => void refreshRuns()}>
              刷新
            </button>
          </div>
          {runs.length === 0 ? (
            <p className="muted">后端启动后会在这里显示历史 Run。</p>
          ) : (
            <div className="run-list">
              {runs.slice(0, 8).map((item) => (
                <button key={item.id} type="button" className="run-card" onClick={() => void loadRun(item.id)}>
                  <span className={`status ${item.status}`}>{statusText(item.status)}</span>
                  <strong>{item.input.company_name || '未填写公司'} · {item.input.job_title || '未填写岗位'}</strong>
                  <small>{item.id} · {formatTime(item.created_at)}</small>
                </button>
              ))}
            </div>
          )}
        </aside>
      </section>

      <section className="panel dashboard-panel">
        <div className="section-heading">
          <div>
            <p className="eyebrow dark">Overview</p>
            <h2>Run 状态与评估</h2>
          </div>
          <div className="connection-state">
            <span className={`dot ${streamState}`} />
            {streamText(streamState)}
          </div>
        </div>

        <div className="dashboard-grid">
          <Metric label="Run 状态" value={run ? statusText(run.status) : '未启动'} />
          <Metric label="匹配分" value={matchReport ? `${matchReport.score}/100` : '--'} />
          <Metric label="关键词覆盖" value={evalReport ? formatPercent(evalReport.keyword_coverage) : '--'} />
          <Metric label="风险主张" value={evalReport ? String(unsupportedClaims.length) : '--'} />
        </div>

        {evalReport && (
          <div className={`eval-card ${evalReport.passed ? 'passed' : 'warning'}`}>
            <strong>{evalReport.passed ? '自检通过' : '需要复核'}</strong>
            <span>{evalWarnings.length > 0 ? evalWarnings.join('；') : '没有发现明显无证据风险。'}</span>
          </div>
        )}
      </section>

      <section className="panel trace-panel">
        <div className="section-heading">
          <div>
            <p className="eyebrow dark">Trace</p>
            <h2>Agent 执行时间线</h2>
          </div>
          {run && <code>{run.id}</code>}
        </div>

        <ol className="timeline">
          {stepPlan.map((plannedStep) => {
            const step = stepsByName.get(plannedStep.name);
            const status = step?.status ?? 'pending';
            return (
              <li key={plannedStep.name} className={`timeline-item ${status}`}>
                <div className="timeline-marker" />
                <div className="timeline-body">
                  <div className="timeline-title">
                    <strong>{plannedStep.label}</strong>
                    <span className={`status ${status}`}>{statusText(status)}</span>
                  </div>
                  <p>{plannedStep.detail}</p>
                  {step && (
                    <details>
                      <summary>{formatTime(step.started_at)} → {formatTime(step.ended_at)}</summary>
                      <pre>{JSON.stringify({ input: step.input, output: step.output, error: step.error || undefined }, null, 2)}</pre>
                    </details>
                  )}
                </div>
              </li>
            );
          })}
        </ol>
      </section>

      <section className="panel artifacts-panel">
        <div className="section-heading">
          <div>
            <p className="eyebrow dark">Artifacts</p>
            <h2>产物输出</h2>
          </div>
          <span className="muted">{run?.artifacts.length ?? 0} 个产物</span>
        </div>

        {run && run.artifacts.length > 0 ? (
          <>
            <div className="artifact-tabs">
              {run.artifacts.map((artifact) => (
                <button
                  key={artifact.id}
                  type="button"
                  className={artifact.type === selectedArtifact?.type ? 'active' : ''}
                  onClick={() => setSelectedArtifactType(artifact.type)}
                >
                  {artifact.title}
                </button>
              ))}
            </div>
            {selectedArtifact && (
              <article className="artifact-card">
                <div className="artifact-header">
                  <div>
                    <strong>{selectedArtifact.title}</strong>
                    <small>{selectedArtifact.type} · {formatTime(selectedArtifact.created_at)}</small>
                  </div>
                  {selectedArtifact.metadata ? <code>{JSON.stringify(selectedArtifact.metadata)}</code> : null}
                </div>
                <pre className="artifact-content">{selectedArtifact.content}</pre>
              </article>
            )}
          </>
        ) : (
          <p className="empty-state">启动一次 Run 后，JD 分析、匹配报告、投递材料和自检报告会显示在这里。</p>
        )}
      </section>

      <section className="panel events-panel">
        <div className="section-heading">
          <div>
            <p className="eyebrow dark">Events</p>
            <h2>SSE 事件流</h2>
          </div>
          <span className="muted">{events.length} events</span>
        </div>
        {events.length === 0 ? (
          <p className="empty-state">实时事件会在 Agent 执行时追加显示。</p>
        ) : (
          <div className="event-list">
            {events.map((event) => (
              <div key={event.id} className="event-row">
                <time>{formatTime(event.created_at)}</time>
                <code>{event.type}</code>
                <span>{event.step?.name ?? event.artifact?.title ?? event.error ?? event.run_status}</span>
              </div>
            ))}
          </div>
        )}
      </section>
    </main>
  );
}

function Metric({ label, value }: { label: string; value: string }) {
  return (
    <div className="metric">
      <span>{label}</span>
      <strong>{value}</strong>
    </div>
  );
}

function setFormField(key: keyof RunInput, value: string, setForm: Dispatch<SetStateAction<RunInput>>) {
  setForm((current) => ({ ...current, [key]: value }));
}

function normalizeRun(run: Run): Run {
  return {
    ...run,
    steps: Array.isArray(run.steps) ? run.steps : [],
    artifacts: Array.isArray(run.artifacts) ? run.artifacts : [],
  };
}

function mergeRunEvent(current: Run | null, event: AgentEvent): Run | null {
  if (!current || current.id !== event.run_id) {
    return current;
  }

  return {
    ...current,
    status: event.run_status ?? current.status,
    steps: event.step ? upsertByID(current.steps, event.step) : current.steps,
    artifacts: event.artifact ? upsertByID(current.artifacts, event.artifact) : current.artifacts,
    updated_at: event.created_at,
  };
}

function upsertByID<T extends { id: string }>(items: T[], nextItem: T): T[] {
  const index = items.findIndex((item) => item.id === nextItem.id);
  if (index === -1) {
    return [...items, nextItem];
  }
  return items.map((item, itemIndex) => (itemIndex === index ? nextItem : item));
}

function upsertEvent(events: AgentEvent[], event: AgentEvent): AgentEvent[] {
  if (events.some((item) => item.id === event.id)) {
    return events;
  }
  return [...events, event];
}

function parseJSON<T>(content?: string): T | null {
  if (!content) {
    return null;
  }
  try {
    return JSON.parse(content) as T;
  } catch {
    return null;
  }
}

async function readAPIError(response: Response): Promise<string> {
  try {
    const payload = (await response.json()) as { error?: string };
    return payload.error ?? `${response.status} ${response.statusText}`;
  } catch {
    return `${response.status} ${response.statusText}`;
  }
}

function statusText(status: RunStatus | StepStatus): string {
  const labels: Record<string, string> = {
    created: '已创建',
    running: '运行中',
    done: '已完成',
    failed: '失败',
    pending: '等待中',
  };
  return labels[status] ?? status;
}

function streamText(state: StreamState): string {
  const labels: Record<StreamState, string> = {
    idle: '未连接',
    connecting: '连接中',
    open: '实时连接',
    closed: '已关闭',
    error: '连接异常',
  };
  return labels[state];
}

function formatTime(value?: string): string {
  if (!value || value.startsWith('0001-')) {
    return '--';
  }
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return '--';
  }
  return date.toLocaleTimeString('zh-CN', { hour12: false });
}

function formatPercent(value: number): string {
  return `${Math.round(value * 100)}%`;
}

createRoot(document.getElementById('root')!).render(<App />);
