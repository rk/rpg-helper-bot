import { ChatSearchDebug } from "./api";
import { SourceFlyover } from "./SourceFlyover";

interface SearchDebugPanelProps {
  debug: ChatSearchDebug | null;
}

function fmtScore(score: number): string {
  return score.toFixed(3);
}

export function SearchDebugPanel({ debug }: SearchDebugPanelProps) {
  if (!debug) return null;

  return (
    <details className="search-debug">
      <summary>Search details</summary>
      <div className="search-debug-body">
        <dl className="search-debug-meta">
          <div>
            <dt>Your question</dt>
            <dd>{debug.original_query}</dd>
          </div>
          <div>
            <dt>Full-text keywords</dt>
            <dd>{debug.fts_query}</dd>
          </div>
          {debug.query_rewritten && (
            <div>
              <dt>Query expanded</dt>
              <dd>Yes — AI added related keywords before searching</dd>
            </div>
          )}
          {debug.rewrite_error && (
            <div>
              <dt>Query expansion</dt>
              <dd className="warn">Failed ({debug.rewrite_error}) — used original wording</dd>
            </div>
          )}
          <div>
            <dt>FTS candidates</dt>
            <dd>{debug.fts_candidate_count}</dd>
          </div>
        </dl>

        <p className="hint search-debug-note">
          Sections sent to the AI are ranked by embedding similarity, then PDF override order (higher PDF #
          wins ties). Lower FTS rank is a stronger keyword match.
        </p>

        {debug.hits.length === 0 ? (
          <p className="hint">No matching sections were found.</p>
        ) : (
          <div className="search-debug-table-wrap">
            <table className="search-debug-table">
              <thead>
                <tr>
                  <th>#</th>
                  <th>Section</th>
                  <th>Embed</th>
                  <th>FTS</th>
                  <th>PDF #</th>
                </tr>
              </thead>
              <tbody>
                {debug.hits.map((hit) => (
                  <tr key={hit.section_id}>
                    <td>{hit.rank}</td>
                    <td>
                      <SourceFlyover
                        source={{
                          section_id: hit.section_id,
                          pdf_title: hit.pdf_title,
                          section_title: hit.section_title,
                          start_page: hit.start_page,
                          end_page: hit.end_page,
                          snippet: hit.snippet,
                        }}
                      >
                        <span className="search-hit-link">
                          <strong>{hit.section_title}</strong>
                          <span className="search-hit-pdf">{hit.pdf_title}</span>
                        </span>
                      </SourceFlyover>
                    </td>
                    <td>{fmtScore(hit.embed_score)}</td>
                    <td>{fmtScore(hit.fts_rank)}</td>
                    <td>{hit.pdf_sort_order + 1}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </div>
    </details>
  );
}
