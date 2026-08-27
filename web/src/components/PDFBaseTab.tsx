import { PDFSummary, api } from "../api";
import { PathBadge } from "./Badges";

type Props = {
  pdf: PDFSummary;
  pdfId: string;
  title: string;
  setTitle: (v: string) => void;
  filePath: string;
  setFilePath: (v: string) => void;
  pageCount: number;
  setPageCount: (v: number) => void;
  savePDF: () => void;
  probePath: () => void;
  deletePDF: () => void;
  indexPDF: () => void;
  indexing: boolean;
  sectionCount: number;
};

export default function PDFBaseTab({
  pdf,
  pdfId,
  title,
  setTitle,
  filePath,
  setFilePath,
  pageCount,
  setPageCount,
  savePDF,
  probePath,
  deletePDF,
  indexPDF,
  indexing,
  sectionCount,
}: Props) {
  return (
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
          <button type="button" className="btn primary" onClick={indexPDF} disabled={indexing || sectionCount === 0}>
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
    </>
  );
}
