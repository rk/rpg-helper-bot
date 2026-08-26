import { useEffect, useState } from "react";
import { Link, useNavigate } from "react-router-dom";
import { api, PDFSummary, TOCImportSource } from "../api";
import { EmptyState, PathBadge, truncatePath } from "../components/Badges";
import { useToast } from "../components/Toast";

export default function LibraryPage() {
  const [pdfs, setPdfs] = useState<PDFSummary[]>([]);
  const [loading, setLoading] = useState(true);
  const [showForm, setShowForm] = useState(false);
  const [title, setTitle] = useState("");
  const [filePath, setFilePath] = useState("");
  const [tocStartPage, setTocStartPage] = useState<number | "">("");
  const [tocEndPage, setTocEndPage] = useState<number | "">("");
  const [tocImportSource, setTocImportSource] = useState<"none" | TOCImportSource>("none");
  const [includeBookmarkChildren, setIncludeBookmarkChildren] = useState(true);
  const navigate = useNavigate();
  const { showError, showInfo } = useToast();

  const load = async () => {
    setLoading(true);
    try {
      setPdfs(await api.listPDFs());
    } catch (err) {
      showError(err instanceof Error ? err.message : "Failed to load PDFs");
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    load();
  }, []);

  const handleCreate = async (e: React.FormEvent) => {
    e.preventDefault();
    try {
      const payload: {
        title: string;
        file_path: string;
        toc_source?: TOCImportSource;
        toc_start_page?: number;
        toc_end_page?: number;
        toc_include_children?: boolean;
      } = { title, file_path: filePath };

      if (tocImportSource === "pages") {
        if (tocStartPage === "" || tocEndPage === "") {
          showError("Enter ToC start and end pages, or choose a different import method");
          return;
        }
        payload.toc_source = "pages";
        payload.toc_start_page = Number(tocStartPage);
        payload.toc_end_page = Number(tocEndPage);
      } else if (tocImportSource === "bookmarks") {
        payload.toc_source = "bookmarks";
        payload.toc_include_children = includeBookmarkChildren;
      }

      const pdf = await api.createPDF(payload);
      if (pdf.toc_extract_error) {
        showError(`PDF added, but section import failed: ${pdf.toc_extract_error}`);
      } else if (payload.toc_source) {
        showInfo("PDF added with imported sections");
      } else {
        showInfo("PDF added");
      }
      setTitle("");
      setFilePath("");
      setTocStartPage("");
      setTocEndPage("");
      setTocImportSource("none");
      setIncludeBookmarkChildren(true);
      setShowForm(false);
      await load();
      navigate(`/library/${pdf.id}`);
    } catch (err) {
      showError(err instanceof Error ? err.message : "Failed to add PDF");
    }
  };

  return (
    <div className="page split-page">
      <aside className="list-pane">
        <div className="pane-header">
          <h1>PDF Library</h1>
          <button type="button" className="btn primary" onClick={() => setShowForm(true)}>
            + Add PDF
          </button>
        </div>

        {showForm && (
          <form className="inline-form card" onSubmit={handleCreate}>
            <label>
              Title
              <input value={title} onChange={(e) => setTitle(e.target.value)} required />
            </label>
            <label>
              Absolute file path
              <input value={filePath} onChange={(e) => setFilePath(e.target.value)} required />
            </label>
            <fieldset className="toc-import-fieldset">
              <legend>Sections on import (optional)</legend>
              <label className="radio-row">
                <input
                  type="radio"
                  name="toc-import"
                  checked={tocImportSource === "none"}
                  onChange={() => setTocImportSource("none")}
                />
                Manual only
              </label>
              <label className="radio-row">
                <input
                  type="radio"
                  name="toc-import"
                  checked={tocImportSource === "bookmarks"}
                  onChange={() => setTocImportSource("bookmarks")}
                />
                From PDF bookmarks
              </label>
              <label className="radio-row">
                <input
                  type="radio"
                  name="toc-import"
                  checked={tocImportSource === "pages"}
                  onChange={() => setTocImportSource("pages")}
                />
                From ToC pages
              </label>
              {tocImportSource === "bookmarks" && (
                <label className="checkbox-row">
                  <input
                    type="checkbox"
                    checked={includeBookmarkChildren}
                    onChange={(e) => setIncludeBookmarkChildren(e.target.checked)}
                  />
                  Include nested bookmarks
                </label>
              )}
              {tocImportSource === "pages" && (
                <div className="toc-extract-row">
                  <label>
                    ToC start page
                    <input
                      type="number"
                      min={1}
                      value={tocStartPage}
                      onChange={(e) => setTocStartPage(e.target.value === "" ? "" : Number(e.target.value))}
                      placeholder="e.g. 5"
                    />
                  </label>
                  <label>
                    ToC end page
                    <input
                      type="number"
                      min={1}
                      value={tocEndPage}
                      onChange={(e) => setTocEndPage(e.target.value === "" ? "" : Number(e.target.value))}
                      placeholder="e.g. 6"
                    />
                  </label>
                </div>
              )}
            </fieldset>
            <div className="form-actions">
              <button type="submit" className="btn primary">
                Save
              </button>
              <button type="button" className="btn" onClick={() => setShowForm(false)}>
                Cancel
              </button>
            </div>
          </form>
        )}

        {loading ? (
          <p className="muted">Loading…</p>
        ) : pdfs.length === 0 ? (
          <EmptyState
            title="No PDFs yet"
            description="Add your first rules PDF to define table-of-contents sections."
            action={
              <button type="button" className="btn primary" onClick={() => setShowForm(true)}>
                Add your first PDF
              </button>
            }
          />
        ) : (
          <ul className="item-list">
            {pdfs.map((pdf) => (
              <li key={pdf.id}>
                <Link to={`/library/${pdf.id}`} className="item-link">
                  <div className="item-title">{pdf.title}</div>
                  <div className="item-meta">
                    <PathBadge status={pdf.path_status} />
                    <span>{pdf.section_count} sections</span>
                    <span>{pdf.game_count} games</span>
                  </div>
                  <div className="item-path">{truncatePath(pdf.file_path)}</div>
                </Link>
              </li>
            ))}
          </ul>
        )}
      </aside>

      <section className="detail-pane muted-pane">
        <EmptyState
          title="Select a PDF"
          description="Choose a PDF from the library to edit its path and table of contents."
        />
      </section>
    </div>
  );
}
