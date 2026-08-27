import { EmptyState } from "./Badges";
import { TOCSection, TOCImportSource } from "../api";

type Props = {
  sections: TOCSection[];
  dirty: boolean;
  tocStartPage: number;
  setTocStartPage: (v: number) => void;
  tocEndPage: number;
  setTocEndPage: (v: number) => void;
  tocImportSource: TOCImportSource;
  setTocImportSource: (v: TOCImportSource) => void;
  includeBookmarkChildren: boolean;
  setIncludeBookmarkChildren: (v: boolean) => void;
  extracting: boolean;
  pathMissing: boolean;
  importSections: () => void;
  saveTOC: () => void;
  addSection: () => void;
  updateSection: (index: number, patch: Partial<TOCSection>) => void;
  moveSection: (index: number, dir: -1 | 1) => void;
  removeSection: (index: number) => void;
};

export default function PDFSectionsTab({
  sections,
  dirty,
  tocStartPage,
  setTocStartPage,
  tocEndPage,
  setTocEndPage,
  tocImportSource,
  setTocImportSource,
  includeBookmarkChildren,
  setIncludeBookmarkChildren,
  extracting,
  pathMissing,
  importSections,
  saveTOC,
  addSection,
  updateSection,
  moveSection,
  removeSection,
}: Props) {
  return (
    <div className="section-block">
      <div className="pane-header">
        <h2>Table of Contents</h2>
        <button type="button" className="btn" onClick={addSection}>
          + Add section
        </button>
      </div>

      <div className="toc-extract-panel card">
        <p className="hint">
          Import sections automatically, then review and save. End pages are set to the page before the next section
          starts.
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
          <button type="button" className="btn primary" onClick={importSections} disabled={extracting || pathMissing}>
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
                    <input value={sec.title} onChange={(e) => updateSection(i, { title: e.target.value })} />
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
  );
}
