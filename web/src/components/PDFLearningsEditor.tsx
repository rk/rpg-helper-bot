import { useEffect, useState } from "react";
import { api, CheatsheetEntry, PDFGlossary, PDFIndexMeta, PDFFeature } from "../api";
import { EmptyState } from "./Badges";
import { useToast } from "./Toast";

function emptyMeta(): PDFIndexMeta {
  return { features: [], glossary: [], cheatsheet: [], llm_glossary_skipped: false };
}

function newGlossaryRow(): PDFGlossary {
  return { feature_id: "", pdf_term: "", evidence: "" };
}

function newFeatureRow(): PDFFeature {
  return { feature_id: "", sections: [], terms: [] };
}

function newCheatsheetRow(): CheatsheetEntry {
  return { feature_id: "", feature_name: "", pdf_terms: [], section: "", start_page: 1 };
}

export default function PDFLearningsEditor({ pdfId, indexed }: { pdfId: string; indexed: boolean }) {
  const { showError, showInfo } = useToast();
  const [meta, setMeta] = useState<PDFIndexMeta | null>(null);
  const [dirty, setDirty] = useState(false);
  const [loading, setLoading] = useState(true);

  const load = async () => {
    setLoading(true);
    try {
      const data = await api.getIndexMeta(pdfId);
      setMeta(data);
      setDirty(false);
    } catch (err) {
      showError(err instanceof Error ? err.message : "Failed to load learnings");
      setMeta(emptyMeta());
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    load();
  }, [pdfId]);

  const save = async () => {
    if (!meta) return;
    try {
      const saved = await api.saveIndexMeta(pdfId, meta);
      setMeta(saved);
      setDirty(false);
      showInfo("Index learnings saved");
    } catch (err) {
      showError(err instanceof Error ? err.message : "Failed to save learnings");
    }
  };

  const updateGlossary = (index: number, patch: Partial<PDFGlossary>) => {
    setMeta((prev) => {
      if (!prev) return prev;
      const glossary = prev.glossary.map((row, i) => (i === index ? { ...row, ...patch } : row));
      return { ...prev, glossary };
    });
    setDirty(true);
  };

  const updateFeature = (index: number, patch: Partial<PDFFeature>) => {
    setMeta((prev) => {
      if (!prev) return prev;
      const features = prev.features.map((row, i) => (i === index ? { ...row, ...patch } : row));
      return { ...prev, features };
    });
    setDirty(true);
  };

  const updateCheatsheet = (index: number, patch: Partial<CheatsheetEntry>) => {
    setMeta((prev) => {
      if (!prev) return prev;
      const cheatsheet = prev.cheatsheet.map((row, i) => (i === index ? { ...row, ...patch } : row));
      return { ...prev, cheatsheet };
    });
    setDirty(true);
  };

  const listField = (values: string[], onChange: (values: string[]) => void, placeholder: string) => (
    <input
      value={values.join(", ")}
      placeholder={placeholder}
      onChange={(e) =>
        onChange(
          e.target.value
            .split(",")
            .map((s) => s.trim())
            .filter(Boolean),
        )
      }
    />
  );

  if (loading) {
    return <p className="muted">Loading index learnings…</p>;
  }

  if (!meta) return null;

  const hasContent = meta.glossary.length > 0 || meta.features.length > 0 || meta.cheatsheet.length > 0;

  return (
    <div className="section-block learnings-panel">
      <div className="pane-header">
        <div>
          <h2>Index learnings</h2>
          <p className="hint">
            Terminology and features detected during indexing. Used for search keyword expansion and chat context.
          </p>
        </div>
        <div className="header-actions">
          {meta.llm_glossary_skipped && (
            <span className="badge badge-warn" title="LLM glossary refinement was skipped or failed">
              offline only
            </span>
          )}
          <button type="button" className="btn" onClick={load}>
            Reload
          </button>
          <button type="button" className="btn primary" onClick={save} disabled={!dirty}>
            Save learnings
          </button>
        </div>
      </div>

      {!indexed && !hasContent ? (
        <EmptyState
          title="No learnings yet"
          description="Index this PDF to build glossary mappings and feature detections from your concept cheatsheet."
        />
      ) : !hasContent ? (
        <EmptyState
          title="Empty learnings"
          description="Indexing completed but no glossary entries were found. Check data/rpg-concepts.yaml and re-index, or add mappings manually below."
          action={
            <button
              type="button"
              className="btn"
              onClick={() => {
                setMeta({ ...meta, glossary: [newGlossaryRow()] });
                setDirty(true);
              }}
            >
              Add glossary entry
            </button>
          }
        />
      ) : null}

      <div className="learnings-section card">
        <div className="learnings-section-header">
          <h3>Glossary</h3>
          <button
            type="button"
            className="btn"
            onClick={() => {
              setMeta({ ...meta, glossary: [...meta.glossary, newGlossaryRow()] });
              setDirty(true);
            }}
          >
            + Add mapping
          </button>
        </div>
        <p className="hint">Maps this book&apos;s terms to canonical feature IDs from rpg-concepts.yaml.</p>
        {meta.glossary.length === 0 ? (
          <p className="muted">No glossary mappings.</p>
        ) : (
          <div className="table-wrap">
            <table>
              <thead>
                <tr>
                  <th>Feature ID</th>
                  <th>Book term</th>
                  <th>Evidence</th>
                  <th></th>
                </tr>
              </thead>
              <tbody>
                {meta.glossary.map((row, i) => (
                  <tr key={`g-${i}`}>
                    <td>
                      <input
                        value={row.feature_id}
                        onChange={(e) => updateGlossary(i, { feature_id: e.target.value })}
                        placeholder="skill_check"
                      />
                    </td>
                    <td>
                      <input
                        value={row.pdf_term}
                        onChange={(e) => updateGlossary(i, { pdf_term: e.target.value })}
                        placeholder="Tests"
                      />
                    </td>
                    <td>
                      <input
                        value={row.evidence ?? ""}
                        onChange={(e) => updateGlossary(i, { evidence: e.target.value })}
                        placeholder="Section or quote"
                      />
                    </td>
                    <td>
                      <button
                        type="button"
                        className="btn icon danger"
                        onClick={() => {
                          setMeta({ ...meta, glossary: meta.glossary.filter((_, j) => j !== i) });
                          setDirty(true);
                        }}
                      >
                        ×
                      </button>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </div>

      <div className="learnings-section card">
        <div className="learnings-section-header">
          <h3>Features detected</h3>
          <button
            type="button"
            className="btn"
            onClick={() => {
              setMeta({ ...meta, features: [...meta.features, newFeatureRow()] });
              setDirty(true);
            }}
          >
            + Add feature
          </button>
        </div>
        {meta.features.length === 0 ? (
          <p className="muted">No features detected.</p>
        ) : (
          <div className="table-wrap">
            <table>
              <thead>
                <tr>
                  <th>Feature ID</th>
                  <th>Terms found</th>
                  <th>Sections</th>
                  <th></th>
                </tr>
              </thead>
              <tbody>
                {meta.features.map((row, i) => (
                  <tr key={`f-${i}`}>
                    <td>
                      <input
                        value={row.feature_id}
                        onChange={(e) => updateFeature(i, { feature_id: e.target.value })}
                      />
                    </td>
                    <td>{listField(row.terms, (terms) => updateFeature(i, { terms }), "rate of fire, rof")}</td>
                    <td>{listField(row.sections, (sections) => updateFeature(i, { sections }), "Ranged Attacks")}</td>
                    <td>
                      <button
                        type="button"
                        className="btn icon danger"
                        onClick={() => {
                          setMeta({ ...meta, features: meta.features.filter((_, j) => j !== i) });
                          setDirty(true);
                        }}
                      >
                        ×
                      </button>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </div>

      <div className="learnings-section card">
        <div className="learnings-section-header">
          <h3>Cheatsheet</h3>
          <button
            type="button"
            className="btn"
            onClick={() => {
              setMeta({ ...meta, cheatsheet: [...meta.cheatsheet, newCheatsheetRow()] });
              setDirty(true);
            }}
          >
            + Add row
          </button>
        </div>
        {meta.cheatsheet.length === 0 ? (
          <p className="muted">No cheatsheet rows.</p>
        ) : (
          <div className="table-wrap">
            <table>
              <thead>
                <tr>
                  <th>Feature</th>
                  <th>Name</th>
                  <th>Book terms</th>
                  <th>Section</th>
                  <th>Page</th>
                  <th></th>
                </tr>
              </thead>
              <tbody>
                {meta.cheatsheet.map((row, i) => (
                  <tr key={`c-${i}`}>
                    <td>
                      <input
                        value={row.feature_id}
                        onChange={(e) => updateCheatsheet(i, { feature_id: e.target.value })}
                      />
                    </td>
                    <td>
                      <input
                        value={row.feature_name ?? ""}
                        onChange={(e) => updateCheatsheet(i, { feature_name: e.target.value })}
                      />
                    </td>
                    <td>
                      {listField(row.pdf_terms, (pdf_terms) => updateCheatsheet(i, { pdf_terms }), "Tests, Test")}
                    </td>
                    <td>
                      <input
                        value={row.section}
                        onChange={(e) => updateCheatsheet(i, { section: e.target.value })}
                      />
                    </td>
                    <td>
                      <input
                        type="number"
                        min={1}
                        value={row.start_page}
                        onChange={(e) => updateCheatsheet(i, { start_page: Number(e.target.value) })}
                      />
                    </td>
                    <td>
                      <button
                        type="button"
                        className="btn icon danger"
                        onClick={() => {
                          setMeta({ ...meta, cheatsheet: meta.cheatsheet.filter((_, j) => j !== i) });
                          setDirty(true);
                        }}
                      >
                        ×
                      </button>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </div>
    </div>
  );
}
