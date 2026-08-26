import { useEffect, useMemo, useState } from "react";
import { Link, useParams } from "react-router-dom";
import { api, GameDetail, GamePDFEntry, PDFSummary, RunningStatus } from "../api";
import { EmptyState, PathBadge, RunningBadge } from "../components/Badges";
import { useToast } from "../components/Toast";

export default function GameDetailPage() {
  const { gameId } = useParams<{ gameId: string }>();
  const { showError, showInfo } = useToast();

  const [game, setGame] = useState<GameDetail | null>(null);
  const [library, setLibrary] = useState<PDFSummary[]>([]);
  const [attached, setAttached] = useState<GamePDFEntry[]>([]);
  const [name, setName] = useState("");
  const [notes, setNotes] = useState("");
  const [runningStatus, setRunningStatus] = useState<RunningStatus | null>(null);
  const [addPdfId, setAddPdfId] = useState("");
  const [dirty, setDirty] = useState(false);

  const availablePDFs = useMemo(
    () => library.filter((p) => !attached.some((a) => a.id === p.id)),
    [library, attached],
  );

  const load = async () => {
    if (!gameId) return;
    try {
      const [g, pdfs, running] = await Promise.all([
        api.getGame(gameId),
        api.listPDFs(),
        api.getRunning(),
      ]);
      setGame(g);
      setAttached(g.pdfs);
      setName(g.name);
      setNotes(g.notes);
      setLibrary(pdfs);
      setRunningStatus(running);
      setDirty(false);
    } catch (err) {
      showError(err instanceof Error ? err.message : "Failed to load game");
    }
  };

  useEffect(() => {
    load();
  }, [gameId]);

  const saveGame = async () => {
    if (!gameId) return;
    try {
      await api.updateGame(gameId, { name, notes });
      showInfo("Game saved");
      await load();
    } catch (err) {
      showError(err instanceof Error ? err.message : "Failed to save game");
    }
  };

  const savePDFOrder = async (next: GamePDFEntry[]) => {
    if (!gameId) return;
    try {
      const saved = await api.setGamePDFs(
        gameId,
        next.map((p) => p.id),
      );
      setAttached(saved);
      setDirty(false);
      showInfo("PDF order saved");
      await load();
    } catch (err) {
      showError(err instanceof Error ? err.message : "Failed to save PDF order");
    }
  };

  const addPDF = async () => {
    if (!addPdfId) return;
    const pdf = library.find((p) => p.id === addPdfId);
    if (!pdf) return;
    const next: GamePDFEntry[] = [
      ...attached,
      {
        ...pdf,
        sort_order: attached.length,
        path_status: pdf.path_status,
        section_count: pdf.section_count,
      },
    ];
    setAttached(next);
    setAddPdfId("");
    setDirty(true);
    await savePDFOrder(next);
  };

  const removePDF = async (pdfId: string) => {
    const next = attached.filter((p) => p.id !== pdfId);
    setAttached(next);
    setDirty(true);
    await savePDFOrder(next);
  };

  const movePDF = async (index: number, dir: -1 | 1) => {
    const nextIndex = index + dir;
    if (nextIndex < 0 || nextIndex >= attached.length) return;
    const next = [...attached];
    [next[index], next[nextIndex]] = [next[nextIndex], next[index]];
    setAttached(next);
    setDirty(true);
    await savePDFOrder(next);
  };

  const toggleRunning = async () => {
    if (!gameId || !game) return;
    try {
      const result = await api.setRunning(gameId, !game.running);
      setRunningStatus(result);
      showInfo(game.running ? "Game stopped" : "Game is now running");
      await load();
    } catch (err) {
      showError(err instanceof Error ? err.message : "Failed to update running status");
    }
  };

  const archive = async () => {
    if (!gameId) return;
    try {
      await api.archiveGame(gameId);
      showInfo("Game archived");
      await load();
    } catch (err) {
      showError(err instanceof Error ? err.message : "Failed to archive game");
    }
  };

  const restore = async () => {
    if (!gameId) return;
    try {
      await api.restoreGame(gameId);
      showInfo("Game restored");
      await load();
    } catch (err) {
      showError(err instanceof Error ? err.message : "Failed to restore game");
    }
  };

  if (!gameId) return null;

  return (
    <div className="page split-page">
      <aside className="list-pane compact">
        <Link to="/games" className="back-link">
          ← Back to games
        </Link>
      </aside>

      <section className="detail-pane">
        {!game ? (
          <p className="muted">Loading…</p>
        ) : (
          <>
            <div className="pane-header">
              <h1>
                {game.name}
                {game.running && <RunningBadge />}
              </h1>
              <div className="header-actions">
                {!game.archived ? (
                  <>
                    <button type="button" className="btn primary" onClick={toggleRunning}>
                      {game.running ? "Stop Running" : "Mark Running"}
                    </button>
                    <button type="button" className="btn" onClick={archive}>
                      Archive
                    </button>
                  </>
                ) : (
                  <button type="button" className="btn" onClick={restore}>
                    Restore
                  </button>
                )}
              </div>
            </div>

            {game.running && runningStatus?.player_url && (
              <div className="card running-panel">
                <h3>Player access</h3>
                <p className="mono">{runningStatus.player_url}</p>
                <p className="muted">{runningStatus.player_note}</p>
              </div>
            )}

            <div className="card form-grid">
              <label>
                Name
                <input value={name} onChange={(e) => setName(e.target.value)} disabled={game.archived} />
              </label>
              <label>
                Notes (optional rules in use)
                <textarea
                  rows={4}
                  value={notes}
                  onChange={(e) => setNotes(e.target.value)}
                  disabled={game.archived}
                  placeholder="Document which optional rules or house rules apply at this table."
                />
              </label>
              <div className="form-actions">
                <button type="button" className="btn primary" onClick={saveGame} disabled={game.archived}>
                  Save game
                </button>
              </div>
            </div>

            <div className="section-block">
              <div className="pane-header">
                <h2>PDFs (override order)</h2>
              </div>
              <p className="hint">Earlier PDFs are base rules; later PDFs override on conflicts.</p>

              {attached.length === 0 ? (
                <EmptyState
                  title="No PDFs attached"
                  description="Attach PDFs from your shared library."
                />
              ) : (
                <ul className="ordered-list">
                  {attached.map((pdf, i) => (
                    <li key={pdf.id} className="ordered-item">
                      <span className="order-index">{i + 1}</span>
                      <div className="ordered-body">
                        <div className="item-title">{pdf.title}</div>
                        <div className="item-meta">
                          <PathBadge status={pdf.path_status} />
                          <span>{pdf.section_count} sections</span>
                        </div>
                      </div>
                      {!game.archived && (
                        <div className="row-actions">
                          <button type="button" className="btn icon" onClick={() => movePDF(i, -1)} disabled={i === 0}>
                            ↑
                          </button>
                          <button
                            type="button"
                            className="btn icon"
                            onClick={() => movePDF(i, 1)}
                            disabled={i === attached.length - 1}
                          >
                            ↓
                          </button>
                          <button type="button" className="btn icon danger" onClick={() => removePDF(pdf.id)}>
                            ×
                          </button>
                        </div>
                      )}
                    </li>
                  ))}
                </ul>
              )}

              {!game.archived && (
                <div className="attach-row">
                  <select value={addPdfId} onChange={(e) => setAddPdfId(e.target.value)}>
                    <option value="">Add PDF from library…</option>
                    {availablePDFs.map((p) => (
                      <option key={p.id} value={p.id}>
                        {p.title}
                        {p.path_status === "missing" ? " (path missing)" : ""}
                      </option>
                    ))}
                  </select>
                  <button type="button" className="btn" onClick={addPDF} disabled={!addPdfId}>
                    Attach
                  </button>
                </div>
              )}

              {dirty && <p className="muted">Saving order…</p>}
            </div>
          </>
        )}
      </section>
    </div>
  );
}
