import { useEffect, useState } from "react";
import { api, CheatsheetCitation, CheatsheetEntry, PDFGlossaryEntry, PDFIndexMeta } from "../api";
import { EmptyState } from "./Badges";
import { useIndexing } from "./IndexingContext";
import { useToast } from "./Toast";

function emptyMeta(): PDFIndexMeta {
  return { glossary: [], features: [], cheatsheet: [], llm_learnings_skipped: false };
}

function newGlossaryRow(): PDFGlossaryEntry {
  return { feature_id: "", terms: [] };
}

function newCheatsheetRow(): CheatsheetEntry {
  return { feature_id: "", definition: "", citations: [] };
}

function newCitation(): CheatsheetCitation {
  return { section_title: "", start_page: 1 };
}

type LearningsPass = "glossary" | "features" | "cheatsheet";

type RebuildState =
  | { kind: "pass"; pass: LearningsPass }
  | { kind: "cheatsheet-missing" }
  | { kind: "cheatsheet-feature"; featureId: string }
  | null;

export default function PDFLearningsEditor({ pdfId, indexed }: { pdfId: string; indexed: boolean }) {
  const { showError, showInfo } = useToast();
  const { runIndex } = useIndexing();
  const [meta, setMeta] = useState<PDFIndexMeta | null>(null);
  const [dirty, setDirty] = useState(false);
  const [loading, setLoading] = useState(true);
  const [rebuilding, setRebuilding] = useState<RebuildState>(null);
  const [featureInput, setFeatureInput] = useState("");

  const load = async () => {
    setLoading(true);
    try {
      const data = await api.getIndexMeta(pdfId);
      setMeta(data);
      setFeatureInput(data.features.join(", "));
      setDirty(false);
    } catch (err) {
      showError(err instanceof Error ? err.message : "Failed to load learnings");
      setMeta(emptyMeta());
      setFeatureInput("");
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
      setFeatureInput(saved.features.join(", "));
      setDirty(false);
      showInfo("Index learnings saved");
    } catch (err) {
      showError(err instanceof Error ? err.message : "Failed to save learnings");
    }
  };

  const updateGlossary = (index: number, patch: Partial<PDFGlossaryEntry>) => {
    setMeta((prev) => {
      if (!prev) return prev;
      const glossary = prev.glossary.map((row, i) => (i === index ? { ...row, ...patch } : row));
      return { ...prev, glossary };
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

  const updateCitation = (cheatIndex: number, citIndex: number, patch: Partial<CheatsheetCitation>) => {
    setMeta((prev) => {
      if (!prev) return prev;
      const cheatsheet = prev.cheatsheet.map((row, i) => {
        if (i !== cheatIndex) return row;
        const citations = row.citations.map((cit, j) => (j === citIndex ? { ...cit, ...patch } : cit));
        return { ...row, citations };
      });
      return { ...prev, cheatsheet };
    });
    setDirty(true);
  };

  const syncFeatures = (value: string) => {
    setFeatureInput(value);
    const features = value
      .split(",")
      .map((s) => s.trim())
      .filter(Boolean);
    setMeta((prev) => (prev ? { ...prev, features } : prev));
    setDirty(true);
  };

  const cheatsheetFeatureIds = (data: PDFIndexMeta) =>
    new Set(data.cheatsheet.map((row) => row.feature_id.trim()).filter(Boolean));

  const needsCheatsheetRetry = (data: PDFIndexMeta) => {
    if (!indexed || data.features.length === 0) return false;
    const done = cheatsheetFeatureIds(data);
    return data.llm_learnings_skipped || data.features.some((id) => !done.has(id));
  };

  const missingCheatsheetFeatures = (data: PDFIndexMeta) => {
    const done = cheatsheetFeatureIds(data);
    return data.features.filter((id) => !done.has(id));
  };

  const isRebuilding = rebuilding !== null;

  const rerunPass = async (pass: LearningsPass, label: string) => {
    setRebuilding({ kind: "pass", pass });
    try {
      await runIndex(pdfId, async () => {
        if (pass === "glossary") await api.rebuildGlossary(pdfId);
        else if (pass === "features") await api.rebuildFeatures(pdfId);
        else await api.rebuildCheatsheet(pdfId);
      });
      await load();
      showInfo(`${label} finished`);
    } catch (err) {
      await load();
      showError(err instanceof Error ? err.message : `${label} failed`);
    } finally {
      setRebuilding(null);
    }
  };

  const applySavedMeta = (saved: PDFIndexMeta) => {
    setMeta(saved);
    setFeatureInput(saved.features.join(", "));
    setDirty(false);
  };

  const rerunCheatsheetMissing = async () => {
    setRebuilding({ kind: "cheatsheet-missing" });
    try {
      let saved: PDFIndexMeta | null = null;
      await runIndex(pdfId, async () => {
        saved = await api.rebuildCheatsheetMissing(pdfId);
      });
      if (saved) {
        applySavedMeta(saved);
      } else {
        await load();
      }
      const stillMissing = saved ? missingCheatsheetFeatures(saved) : [];
      if (stillMissing.length > 0) {
        showError(`Still missing cheatsheet entries for: ${stillMissing.join(", ")}`);
      } else {
        showInfo("Missing cheatsheet entries rebuilt");
      }
    } catch (err) {
      await load();
      showError(err instanceof Error ? err.message : "Missing cheatsheet rebuild failed");
    } finally {
      setRebuilding(null);
    }
  };

  const rerunCheatsheetFeature = async (featureId: string) => {
    const id = featureId.trim();
    if (!id) {
      showError("Feature ID is required to retry cheatsheet entry");
      return;
    }
    setRebuilding({ kind: "cheatsheet-feature", featureId: id });
    try {
      let saved: PDFIndexMeta | null = null;
      await runIndex(pdfId, async () => {
        saved = await api.rebuildCheatsheetFeature(pdfId, id);
      });
      if (saved) {
        applySavedMeta(saved);
      } else {
        await load();
      }
      showInfo(`Cheatsheet entry for ${id} rebuilt`);
    } catch (err) {
      await load();
      showError(err instanceof Error ? err.message : `Cheatsheet rebuild for ${id} failed`);
    } finally {
      setRebuilding(null);
    }
  };
  const isPassRebuilding = (pass: LearningsPass) =>
    rebuilding?.kind === "pass" && rebuilding.pass === pass;

  const isFeatureRebuilding = (featureId: string) =>
    rebuilding?.kind === "cheatsheet-feature" && rebuilding.featureId === featureId;

  const canRerun = indexed && !dirty && !isRebuilding;

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
            LLM-extracted terminology, detected features, and cheatsheet definitions. Re-run individual
            passes after saving manual edits, or to retry a failed step. Cheatsheet uses hybrid search
            plus one LLM summarize call per feature.
          </p>
        </div>
        <div className="header-actions">
          {meta.llm_learnings_skipped && (
            <span className="badge badge-warn" title="LLM learnings were skipped or failed">
              LLM skipped
            </span>
          )}
          {needsCheatsheetRetry(meta) && (
            <span className="badge badge-warn" title="One or more cheatsheet entries are missing">
              Cheatsheet incomplete
            </span>
          )}
          <button type="button" className="btn" onClick={load} disabled={isRebuilding}>
            Reload
          </button>
          <button type="button" className="btn primary" onClick={save} disabled={!dirty || isRebuilding}>
            Save learnings
          </button>
        </div>
      </div>

      {dirty && indexed && (
        <p className="hint learnings-dirty-hint">Save manual edits before re-running LLM passes.</p>
      )}

      {!indexed && !hasContent ? (
        <EmptyState
          title="No learnings yet"
          description="Index this PDF to run the learnings pipeline (glossary, features, search-based cheatsheet)."
        />
      ) : !hasContent ? (
        <EmptyState
          title="Empty learnings"
          description="Indexing completed but no learnings were produced. Re-index with LLM configured, or add entries manually."
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
          <div className="header-actions">
            <button
              type="button"
              className="btn"
              disabled={!canRerun}
              title={dirty ? "Save learnings before re-running" : undefined}
              onClick={() => rerunPass("glossary", "Glossary rebuild")}
            >
              {isPassRebuilding("glossary") ? "Re-running…" : "Re-run LLM"}
            </button>
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
        </div>
        <p className="hint">Book terms grouped by canonical feature ID for FTS query expansion.</p>
        {meta.glossary.length === 0 ? (
          <p className="muted">No glossary mappings.</p>
        ) : (
          <div className="table-wrap">
            <table>
              <thead>
                <tr>
                  <th>Feature ID</th>
                  <th>Book terms (comma-separated)</th>
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
                        value={row.terms.join(", ")}
                        onChange={(e) =>
                          updateGlossary(i, {
                            terms: e.target.value
                              .split(",")
                              .map((s) => s.trim())
                              .filter(Boolean),
                          })
                        }
                        placeholder="Tests, Trait roll"
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
            disabled={!canRerun}
            title={dirty ? "Save learnings before re-running" : undefined}
            onClick={() => rerunPass("features", "Features rebuild")}
          >
            {isPassRebuilding("features") ? "Re-running…" : "Re-run LLM"}
          </button>
        </div>
        <p className="hint">Feature IDs present in this PDF. Re-run uses the saved glossary above.</p>
        <input
          value={featureInput}
          onChange={(e) => syncFeatures(e.target.value)}
          placeholder="skill_check, wild_die, rate_of_fire"
        />
      </div>

      <div className="learnings-section card">
        <div className="learnings-section-header">
          <h3>Cheatsheet</h3>
          <div className="header-actions">
            {missingCheatsheetFeatures(meta).length > 0 && (
              <button
                type="button"
                className="btn"
                disabled={!canRerun}
                title={
                  dirty
                    ? "Save learnings before re-running"
                    : `Retry ${missingCheatsheetFeatures(meta).length} missing feature(s)`
                }
                onClick={rerunCheatsheetMissing}
              >
                {rebuilding?.kind === "cheatsheet-missing" ? "Retrying…" : "Retry missing"}
              </button>
            )}
            <button
              type="button"
              className="btn"
              disabled={!canRerun || meta.features.length === 0}
              title={
                dirty
                  ? "Save learnings before re-running"
                  : meta.features.length === 0
                    ? "Detect features first"
                    : "Rebuild all cheatsheet entries from scratch"
              }
              onClick={() => rerunPass("cheatsheet", "Cheatsheet rebuild")}
            >
              {isPassRebuilding("cheatsheet") ? "Re-running…" : "Re-run all"}
            </button>
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
        </div>
        {missingCheatsheetFeatures(meta).length > 0 && (
          <p className="hint">
            Missing cheatsheet entries for: {missingCheatsheetFeatures(meta).join(", ")}
          </p>
        )}
        {meta.cheatsheet.length === 0 ? (
          <p className="muted">No cheatsheet rows.</p>
        ) : (
          meta.cheatsheet.map((row, i) => (
            <div key={`c-${i}`} className="cheatsheet-row">
              <div className="cheatsheet-row-header">
                <label>
                  Feature ID
                  <input
                    value={row.feature_id}
                    onChange={(e) => updateCheatsheet(i, { feature_id: e.target.value })}
                  />
                </label>
                <div className="header-actions">
                  <button
                    type="button"
                    className="btn"
                    disabled={!canRerun || !row.feature_id.trim()}
                    title={
                      dirty
                        ? "Save learnings before re-running"
                        : !row.feature_id.trim()
                          ? "Set a feature ID first"
                          : "Re-run LLM for this feature only"
                    }
                    onClick={() => rerunCheatsheetFeature(row.feature_id)}
                  >
                    {isFeatureRebuilding(row.feature_id.trim()) ? "Retrying…" : "Retry LLM"}
                  </button>
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
                </div>
              </div>
              <label>
                Definition
                <textarea
                  rows={3}
                  value={row.definition}
                  onChange={(e) => updateCheatsheet(i, { definition: e.target.value })}
                />
              </label>
              <div className="citations-block">
                <div className="learnings-section-header">
                  <h4>Citations</h4>
                  <button
                    type="button"
                    className="btn"
                    onClick={() => {
                      updateCheatsheet(i, { citations: [...row.citations, newCitation()] });
                    }}
                  >
                    + Add citation
                  </button>
                </div>
                {row.citations.length === 0 ? (
                  <p className="muted">No citations.</p>
                ) : (
                  <div className="table-wrap">
                    <table>
                      <thead>
                        <tr>
                          <th>Section title</th>
                          <th>Start page</th>
                          <th>End page</th>
                          <th></th>
                        </tr>
                      </thead>
                      <tbody>
                        {row.citations.map((cit, j) => (
                          <tr key={`cit-${i}-${j}`}>
                            <td>
                              <input
                                value={cit.section_title}
                                onChange={(e) => updateCitation(i, j, { section_title: e.target.value })}
                              />
                            </td>
                            <td>
                              <input
                                type="number"
                                min={1}
                                value={cit.start_page}
                                onChange={(e) => updateCitation(i, j, { start_page: Number(e.target.value) })}
                              />
                            </td>
                            <td>
                              <input
                                type="number"
                                min={0}
                                value={cit.end_page ?? ""}
                                placeholder="optional"
                                onChange={(e) =>
                                  updateCitation(i, j, {
                                    end_page: e.target.value ? Number(e.target.value) : undefined,
                                  })
                                }
                              />
                            </td>
                            <td>
                              <button
                                type="button"
                                className="btn icon danger"
                                onClick={() => {
                                  updateCheatsheet(i, {
                                    citations: row.citations.filter((_, k) => k !== j),
                                  });
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
          ))
        )}
      </div>
    </div>
  );
}
