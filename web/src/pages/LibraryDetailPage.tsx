import { useEffect, useState } from "react";
import { Link, useNavigate, useParams, useSearchParams } from "react-router-dom";
import { api, PDFSummary, TOCSection, TOCImportSource } from "../api";
import { useIndexing } from "../components/IndexingContext";
import PDFBaseTab from "../components/PDFBaseTab";
import PDFLearningsEditor from "../components/PDFLearningsEditor";
import PDFSectionsTab from "../components/PDFSectionsTab";
import { useToast } from "../components/Toast";

type DetailTab = "base" | "sections" | "learnings";

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

function parseTab(value: string | null): DetailTab {
  if (value === "sections" || value === "learnings") return value;
  return "base";
}

export default function LibraryDetailPage() {
  const { pdfId } = useParams<{ pdfId: string }>();
  const navigate = useNavigate();
  const [searchParams, setSearchParams] = useSearchParams();
  const { showError, showInfo } = useToast();
  const { runIndex } = useIndexing();

  const [tab, setTab] = useState<DetailTab>(() => parseTab(searchParams.get("tab")));
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

  useEffect(() => {
    setTab(parseTab(searchParams.get("tab")));
  }, [searchParams]);

  const selectTab = (next: DetailTab) => {
    setTab(next);
    setSearchParams(next === "base" ? {} : { tab: next }, { replace: true });
  };

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
            <nav className="nav-tabs detail-tabs" aria-label="PDF detail sections">
              <button type="button" className={tab === "base" ? "active" : ""} onClick={() => selectTab("base")}>
                Base
              </button>
              <button type="button" className={tab === "sections" ? "active" : ""} onClick={() => selectTab("sections")}>
                Sections
              </button>
              <button
                type="button"
                className={tab === "learnings" ? "active" : ""}
                onClick={() => selectTab("learnings")}
              >
                Learnings
              </button>
            </nav>

            {tab === "base" && (
              <PDFBaseTab
                pdf={pdf}
                pdfId={pdfId}
                title={title}
                setTitle={setTitle}
                filePath={filePath}
                setFilePath={setFilePath}
                pageCount={pageCount}
                setPageCount={setPageCount}
                savePDF={savePDF}
                probePath={probePath}
                deletePDF={deletePDF}
                indexPDF={indexPDF}
                indexing={indexing}
                sectionCount={sections.length}
              />
            )}

            {tab === "sections" && (
              <PDFSectionsTab
                sections={sections}
                dirty={dirty}
                tocStartPage={tocStartPage}
                setTocStartPage={setTocStartPage}
                tocEndPage={tocEndPage}
                setTocEndPage={setTocEndPage}
                tocImportSource={tocImportSource}
                setTocImportSource={setTocImportSource}
                includeBookmarkChildren={includeBookmarkChildren}
                setIncludeBookmarkChildren={setIncludeBookmarkChildren}
                extracting={extracting}
                pathMissing={pdf.path_status === "missing"}
                importSections={importSections}
                saveTOC={saveTOC}
                addSection={addSection}
                updateSection={updateSection}
                moveSection={moveSection}
                removeSection={removeSection}
              />
            )}

            {tab === "learnings" && <PDFLearningsEditor pdfId={pdfId} indexed={pdf.index_status === "indexed"} />}
          </>
        )}
      </section>
    </div>
  );
}
