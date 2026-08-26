import { useEffect, useState } from "react";
import { Link, useNavigate } from "react-router-dom";
import { api, PDFSummary } from "../api";
import { EmptyState, PathBadge, truncatePath } from "../components/Badges";
import { useToast } from "../components/Toast";

export default function LibraryPage() {
  const [pdfs, setPdfs] = useState<PDFSummary[]>([]);
  const [loading, setLoading] = useState(true);
  const [showForm, setShowForm] = useState(false);
  const [title, setTitle] = useState("");
  const [filePath, setFilePath] = useState("");
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
      const pdf = await api.createPDF({ title, file_path: filePath });
      showInfo("PDF added");
      setTitle("");
      setFilePath("");
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
