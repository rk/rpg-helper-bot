import { ReactNode } from "react";
import { ChatSource } from "./api";

interface SourceFlyoverProps {
  source: ChatSource;
  children: ReactNode;
  className?: string;
}

export function SourceFlyover({ source, children, className = "" }: SourceFlyoverProps) {
  return (
    <span className={`source-ref ${className}`.trim()} tabIndex={0}>
      {children}
      <span className="source-flyover" role="tooltip">
        <strong>
          {source.pdf_title} — {source.section_title}
        </strong>
        <span className="pages">
          pp. {source.start_page}–{source.end_page}
        </span>
        {source.snippet && <p>{source.snippet}</p>}
      </span>
    </span>
  );
}

interface SourceListProps {
  sources: ChatSource[];
}

export function SourceList({ sources }: SourceListProps) {
  if (sources.length === 0) return null;

  return (
    <div className="msg-sources">
      <span className="msg-sources-label">Sources</span>
      <ul>
        {sources.map((s, i) => (
          <li key={s.section_id}>
            <SourceFlyover source={s}>
              <button type="button" className="source-chip">
                [{i + 1}] {s.section_title}
              </button>
            </SourceFlyover>
          </li>
        ))}
      </ul>
    </div>
  );
}
