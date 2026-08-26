import { useEffect, useState } from "react";
import { Link, useNavigate, useParams } from "react-router-dom";
import { api, PDFSummary, TOCSection } from "../api";
import { EmptyState, PathBadge } from "../components/Badges";
import { useToast } from "../components/Toast";

function newSection(): TOCSection {
  return {
    id: "",
    pdf_id: "",
    title: "",
    start_page: 1,
    end_page: 1,
    sort_order: 0,
  };
}

export default function LibraryDetailPage() {
  const { pdfId } = useParams<{ pdfId: string }>();
  const navigate = useNavigate();
  const { showError, showInfo } = useToast();

  const [pdf, setPdf] = useState<PDFSummary | null>(null);
  const [sections, setSections] = useState<TOCSection[]>([]);
  const [title, setTitle] = useState("");
  const [filePath, setFilePath] = useState("");
  const [pageCount, setPageCount] = useState(0);
  const [dirty, setDirty] = useState(false);

  const load = async () => {
    if (!pdfId) return;
    try {
      const [p, toc] = await Promise.all([api.getPDF(pdfId), api.getTOC(pdfId)]);
      setPdf(p);
      setTitle(p.title);
      setFilePath(p.file_path);
      setPageCount(p.page_count);
      setSections(toc.length ? toc : [newSection()]);
      setDirty(false);
    } catch (err) {
      showError(err instanceof Error ? err.message : "Failed to load PDF");
    }
  };

  useEffect(() => {
    load();
  }, [pdfId]);

  const savePDF = async () => {
    if (!pdfId) return;
    try {
      const updated = await api.updatePDF(pdfId, {
        title,
        file_path: filePath,
        page_count: pageCount,
      });
      setPdf(updated);
      showInfo("PDF saved");
    } catch (err) {
      showError(err instanceof Error ? err.message : "Failed to save PDF");
    }
  };

  const saveTOC = async () => {
    if (!pdfId) return;
    try {
      const saved = await api.saveTOC(pdfId, sections);
      setSections(saved);
      setDirty(false);
      showInfo("Table of contents saved");
      await load();
    } catch (err) {
      showError(err instanceof Error ? err.message : "Failed to save sections");
    }
  };

  const probePath = async () => {
    if (!pdfId) return;
    try {
      const result = await api.probePDF(pdfId);
      if (pdf) setPdf({ ...pdf, path_status: result.path_status });
      showInfo(result.path_status === "ok" ? "Path is reachable" : "Path is missing");
    } catch (err) {
      showError(err instanceof Error ? err.message : "Probe failed");
    }
  };

  const deletePDF = async () => {
    if (!pdfId || !confirm("Delete this PDF from the library?")) return;
    try {
      await api.deletePDF(pdfId);
      showInfo("PDF deleted");
      navigate("/library");
    } catch (err) {
      showError(err instanceof Error ? err.message : "Failed to delete PDF");
    }
  };

  const updateSection = (index: number, patch: Partial<TOCSection>) => {
    setSections((prev) => prev.map((s, i) => (i === index ? { ...s, ...patch } : s)));
    setDirty(true);
  };

  const moveSection = (index: number, dir: -1 | 1) => {
    const next = index + dir;
    if (next < 0 || next >= sections.length) return;
    setSections((prev) => {
      const copy = [...prev];
      [copy[index], copy[next]] = [copy[next], copy[index]];
      return copy;
    });
    setDirty(true);
  };

  const removeSection = (index: number) => {
    setSections((prev) => prev.filter((_, i) => i !== index));
    setDirty(true);
  };

  const addSection = () => {
    setSections((prev) => [...prev, newSection()]);
    setDirty(true);
  };

  if (!pdfId) return null;

  return (
    <div className="page split-page">
      <aside className="list-pane compact">
        <Link to="/library" className="back-link">
          ← Back to library
        </Link>
      </aside>

      <section className="detail-pane">
        {!pdf ? (
          <p className="muted">Loading…</p>
        ) : (
          <>
            <div className="pane-header">
              <h1>{pdf.title}</h1>
              <div className="header-actions">
                <PathBadge status={pdf.path_status} />
                <button type="button" className="btn" onClick={probePath}>
                  Check path
                </button>
                <button type="button" className="btn danger" onClick={deletePDF} disabled={pdf.game_count > 0}>
                  Delete
                </button>
              </div>
            </div>

            <div className="card form-grid">
              <label>
                Title
                <input value={title} onChange={(e) => setTitle(e.target.value)} />
              </label>
              <label>
                File path
                <input
                  value={filePath}
                  onChange={(e) => setFilePath(e.target.value)}
                  className={pdf.path_status === "missing" ? "input-error" : ""}
                />
              </label>
              <label>
                Page count
                <input
                  type="number"
                  min={0}
                  value={pageCount}
                  onChange={(e) => setPageCount(Number(e.target.value))}
                />
              </label>
              <div className="form-actions">
                <button type="button" className="btn primary" onClick={savePDF}>
                  Save PDF
                </button>
              </div>
              {pdf.path_status === "missing" && (
                <p className="warning">This file path could not be found. Update the path and save.</p>
              )}
            </div>

            <div className="section-block">
              <div className="pane-header">
                <h2>Table of Contents</h2>
                <button type="button" className="btn" onClick={addSection}>
                  + Add section
                </button>
              </div>

              {sections.length === 0 ? (
                <EmptyState
                  title="No sections"
                  description="Define sections with title and page ranges for future search."
                  action={
                    <button type="button" className="btn primary" onClick={addSection}>
                      Add first section
                    </button>
                  }
                />
              ) : (
                <div className="table-wrap">
                  <table>
                    <thead>
                      <tr>
                        <th>Title</th>
                        <th>Start</th>
                        <th>End</th>
                        <th></th>
                      </tr>
                    </thead>
                    <tbody>
                      {sections.map((sec, i) => (
                        <tr key={sec.id || `new-${i}`}>
                          <td>
                            <input
                              value={sec.title}
                              onChange={(e) => updateSection(i, { title: e.target.value })}
                            />
                          </td>
                          <td>
                            <input
                              type="number"
                              min={1}
                              value={sec.start_page}
                              onChange={(e) => updateSection(i, { start_page: Number(e.target.value) })}
                            />
                          </td>
                          <td>
                            <input
                              type="number"
                              min={1}
                              value={sec.end_page}
                              onChange={(e) => updateSection(i, { end_page: Number(e.target.value) })}
                            />
                          </td>
                          <td className="row-actions">
                            <button type="button" className="btn icon" onClick={() => moveSection(i, -1)} disabled={i === 0}>
                              ↑
                            </button>
                            <button
                              type="button"
                              className="btn icon"
                              onClick={() => moveSection(i, 1)}
                              disabled={i === sections.length - 1}
                            >
                              ↓
                            </button>
                            <button type="button" className="btn icon danger" onClick={() => removeSection(i)}>
                              ×
                            </button>
                          </td>
                        </tr>
                      ))}
                    </tbody>
                  </table>
                </div>
              )}

              <div className="form-actions">
                <button type="button" className="btn primary" onClick={saveTOC} disabled={!dirty}>
                  Save sections
                </button>
              </div>
            </div>
          </>
        )}
      </section>
    </div>
  );
}
