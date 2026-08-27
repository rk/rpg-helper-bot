import { useEffect, useState } from "react";
import { Link, useNavigate, useParams } from "react-router-dom";
import { api, PDFSummary, TOCSection, TOCImportSource } from "../api";
import { EmptyState, PathBadge } from "../components/Badges";
import { useIndexing } from "../components/IndexingContext";
import PDFLearningsEditor from "../components/PDFLearningsEditor";
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
  const { runIndex } = useIndexing();

  const [pdf, setPdf] = useState<PDFSummary | null>(null);
  const [sections, setSections] = useState<TOCSection[]>([]);
  const [title, setTitle] = useState("");
  const [filePath, setFilePath] = useState("");
  const [pageCount, setPageCount] = useState(0);
  const [dirty, setDirty] = useState(false);
  const [indexing, setIndexing] = useState(false);
  const [tocStartPage, setTocStartPage] = useState(1);
  const [tocEndPage, setTocEndPage] = useState(3);
  const [tocImportSource, setTocImportSource] = useState<TOCImportSource>("bookmarks");
  const [includeBookmarkChildren, setIncludeBookmarkChildren] = useState(true);
  const [extracting, setExtracting] = useState(false);

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

  const indexPDF = async () => {
    if (!pdfId) return;
    setIndexing(true);
    try {
      await runIndex(pdfId, async () => {
        if (dirty) await saveTOC();
        await api.indexPDF(pdfId);
      });
      showInfo("PDF indexed");
      await load();
    } catch (err) {
      showError(err instanceof Error ? err.message : "Indexing failed");
    } finally {
      setIndexing(false);
    }
  };

  const probePath = async () => {
    if (!pdfId) return;
    try {
      const result = await api.probePDF(pdfId);
      if (pdf) setPdf({ ...pdf, path_status: result.path_status });
      if (result.page_count > 0) setPageCount(result.page_count);
      showInfo(result.path_status === "ok" ? "Path is reachable" : "Path is missing");
    } catch (err) {
      showError(err instanceof Error ? err.message : "Probe failed");
    }
  };

  const importSections = async () => {
    if (!pdfId) return;
    setExtracting(true);
    try {
      const result = await api.extractTOC(pdfId, {
        source: tocImportSource,
        start_page: tocStartPage,
        end_page: tocEndPage,
        include_children: includeBookmarkChildren,
      });
      setSections(result.sections.length ? result.sections : [newSection()]);
      if (result.page_count > 0) setPageCount(result.page_count);
      setDirty(true);
      const label = result.source === "bookmarks" ? "bookmarks" : "ToC pages";
      showInfo(`Imported ${result.sections.length} sections from ${label}`);
    } catch (err) {
      showError(err instanceof Error ? err.message : "Section import failed");
    } finally {
      setExtracting(false);
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
              <div className="title-row">
                {pdf.index_status === "indexed" && pdfId && (
                  <img src={api.thumbnailURL(pdfId)} alt="" className="pdf-thumb" />
                )}
                <h1>{pdf.title}</h1>
              </div>
              <div className="header-actions">
                <PathBadge status={pdf.path_status} />
                <span className="badge badge-index">{pdf.index_status}</span>
                <button type="button" className="btn primary" onClick={indexPDF} disabled={indexing || sections.length === 0}>
                  {indexing ? "Indexing…" : "Index PDF"}
                </button>
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

              <div className="toc-extract-panel card">
                <p className="hint">
                  Import sections automatically, then review and save. End pages are set to the page before the next
                  section starts.
                </p>
                <div className="toc-import-modes">
                  <label className="radio-row">
                    <input
                      type="radio"
                      name="toc-import-detail"
                      checked={tocImportSource === "bookmarks"}
                      onChange={() => setTocImportSource("bookmarks")}
                    />
                    PDF bookmarks
                  </label>
                  <label className="radio-row">
                    <input
                      type="radio"
                      name="toc-import-detail"
                      checked={tocImportSource === "pages"}
                      onChange={() => setTocImportSource("pages")}
                    />
                    ToC pages
                  </label>
                </div>
                {tocImportSource === "bookmarks" ? (
                  <label className="checkbox-row">
                    <input
                      type="checkbox"
                      checked={includeBookmarkChildren}
                      onChange={(e) => setIncludeBookmarkChildren(e.target.checked)}
                    />
                    Include nested bookmarks
                  </label>
                ) : (
                  <div className="toc-extract-row">
                    <label>
                      ToC start
                      <input
                        type="number"
                        min={1}
                        value={tocStartPage}
                        onChange={(e) => setTocStartPage(Number(e.target.value))}
                      />
                    </label>
                    <label>
                      ToC end
                      <input
                        type="number"
                        min={1}
                        value={tocEndPage}
                        onChange={(e) => setTocEndPage(Number(e.target.value))}
                      />
                    </label>
                  </div>
                )}
                <div className="form-actions">
                  <button
                    type="button"
                    className="btn primary"
                    onClick={importSections}
                    disabled={extracting || pdf.path_status === "missing"}
                  >
                    {extracting ? "Importing…" : "Import sections"}
                  </button>
                </div>
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

            <PDFLearningsEditor pdfId={pdfId} indexed={pdf.index_status === "indexed"} />
          </>
        )}
      </section>
    </div>
  );
}
