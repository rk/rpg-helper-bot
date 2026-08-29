export type ChatActivityStep =
  | { kind: "status"; message: string }
  | { kind: "thinking"; content: string }
  | { kind: "tool-call"; call: { tool: string; args: Record<string, unknown> } }
  | {
      kind: "tool-result";
      call: { tool: string; args: Record<string, unknown> };
      preview: string;
      error?: boolean;
    };

interface ChatActivityPanelProps {
  steps: ChatActivityStep[];
  live?: boolean;
  status?: string | null;
}

const toolLabels: Record<string, string> = {
  lookup_glossary: "Glossary lookup",
  lookup_cheatsheet: "Cheatsheet lookup",
  search: "Rule search",
};

function formatToolArgs(args: Record<string, unknown>): string {
  if (typeof args.query === "string" && args.query.trim()) {
    return args.query.trim();
  }
  if (typeof args.feature_id === "string" && args.feature_id.trim()) {
    return args.feature_id.trim();
  }
  if (typeof args.term === "string" && args.term.trim()) {
    return args.term.trim();
  }
  if (typeof args.raw === "string" && args.raw.trim()) {
    return args.raw.trim();
  }
  try {
    return JSON.stringify(args);
  } catch {
    return "";
  }
}

function stepKey(step: ChatActivityStep, index: number): string {
  switch (step.kind) {
    case "status":
      return `status-${index}-${step.message}`;
    case "thinking":
      return `thinking-${index}-${step.content.slice(0, 40)}`;
    case "tool-call":
      return `call-${index}-${step.call.tool}`;
    case "tool-result":
      return `result-${index}-${step.call.tool}`;
  }
}

export function ChatActivityPanel({ steps, live, status }: ChatActivityPanelProps) {
  if (steps.length === 0) {
    if (!live) return null;
    return (
      <div className="chat-activity chat-activity-live">
        <p className="hint chat-status">{status ?? "Working…"}</p>
      </div>
    );
  }

  return (
    <details className={`chat-activity${live ? " chat-activity-live" : ""}`} open={live}>
      <summary>{live ? (status ?? "Working…") : "How this answer was prepared"}</summary>
      <ol className="chat-activity-steps">
        {steps.map((step, index) => (
          <li key={stepKey(step, index)} className={`chat-activity-step chat-activity-${step.kind}`}>
            {step.kind === "status" && <span className="chat-activity-status">{step.message}</span>}
            {step.kind === "thinking" && (
              <>
                <span className="chat-activity-label">Reasoning</span>
                <pre className="chat-activity-thinking">{step.content}</pre>
              </>
            )}
            {step.kind === "tool-call" && (
              <>
                <span className="chat-activity-label">
                  {toolLabels[step.call.tool] ?? step.call.tool}
                </span>
                <span className="chat-activity-detail">{formatToolArgs(step.call.args)}</span>
              </>
            )}
            {step.kind === "tool-result" && (
              <>
                <span className="chat-activity-label">
                  {toolLabels[step.call.tool] ?? step.call.tool} result
                  {step.error ? " (error)" : ""}
                </span>
                <pre className="chat-activity-result">{step.preview}</pre>
              </>
            )}
          </li>
        ))}
      </ol>
    </details>
  );
}

export function parseChatActivityStep(raw: unknown): ChatActivityStep | null {
  if (!raw || typeof raw !== "object") return null;
  const step = raw as Record<string, unknown>;
  if (step.kind === "status" && typeof step.message === "string") {
    return { kind: "status", message: step.message };
  }
  if (step.kind === "thinking" && typeof step.content === "string") {
    return { kind: "thinking", content: step.content };
  }
  if (
    (step.kind === "tool-call" || step.kind === "tool-result") &&
    step.call &&
    typeof step.call === "object"
  ) {
    const call = step.call as Record<string, unknown>;
    if (typeof call.tool !== "string") return null;
    const args = call.args && typeof call.args === "object" ? (call.args as Record<string, unknown>) : {};
    const base = { call: { tool: call.tool, args } };
    if (step.kind === "tool-call") {
      return { kind: "tool-call", ...base };
    }
    if (typeof step.preview === "string") {
      return {
        kind: "tool-result",
        ...base,
        preview: step.preview,
        error: step.error === true,
      };
    }
  }
  return null;
}
