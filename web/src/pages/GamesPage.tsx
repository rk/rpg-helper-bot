import { useEffect, useState } from "react";
import { Link, useNavigate } from "react-router-dom";
import { api, GameSummary, PDFSummary } from "../api";
import { EmptyState, RunningBadge } from "../components/Badges";
import { useToast } from "../components/Toast";

type Filter = "active" | "archived" | "all";

export default function GamesPage() {
  const [games, setGames] = useState<GameSummary[]>([]);
  const [pdfs, setPdfs] = useState<PDFSummary[]>([]);
  const [filter, setFilter] = useState<Filter>("active");
  const [loading, setLoading] = useState(true);
  const [name, setName] = useState("");
  const [showForm, setShowForm] = useState(false);
  const navigate = useNavigate();
  const { showError, showInfo } = useToast();

  const load = async () => {
    setLoading(true);
    try {
      const [g, p] = await Promise.all([api.listGames(filter), api.listPDFs()]);
      setGames(g);
      setPdfs(p);
    } catch (err) {
      showError(err instanceof Error ? err.message : "Failed to load games");
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    load();
  }, [filter]);

  const handleCreate = async (e: React.FormEvent) => {
    e.preventDefault();
    try {
      const game = await api.createGame({ name });
      showInfo("Game created");
      setName("");
      setShowForm(false);
      await load();
      navigate(`/games/${game.id}`);
    } catch (err) {
      showError(err instanceof Error ? err.message : "Failed to create game");
    }
  };

  const canCreateGame = pdfs.length > 0;

  return (
    <div className="page split-page">
      <aside className="list-pane">
        <div className="pane-header">
          <h1>Games</h1>
          <button
            type="button"
            className="btn primary"
            onClick={() => setShowForm(true)}
            disabled={!canCreateGame}
            title={canCreateGame ? undefined : "Add a PDF to the library first"}
          >
            + New Game
          </button>
        </div>

        <div className="filter-row">
          {(["active", "archived", "all"] as Filter[]).map((f) => (
            <button
              key={f}
              type="button"
              className={`btn filter ${filter === f ? "active" : ""}`}
              onClick={() => setFilter(f)}
            >
              {f[0].toUpperCase() + f.slice(1)}
            </button>
          ))}
        </div>

        {!canCreateGame && (
          <p className="hint">
            Add at least one PDF in the <Link to="/library">library</Link> before creating a game.
          </p>
        )}

        {showForm && (
          <form className="inline-form card" onSubmit={handleCreate}>
            <label>
              Game name
              <input value={name} onChange={(e) => setName(e.target.value)} required />
            </label>
            <div className="form-actions">
              <button type="submit" className="btn primary">
                Create
              </button>
              <button type="button" className="btn" onClick={() => setShowForm(false)}>
                Cancel
              </button>
            </div>
          </form>
        )}

        {loading ? (
          <p className="muted">Loading…</p>
        ) : games.length === 0 ? (
          <EmptyState
            title="No games yet"
            description={
              canCreateGame
                ? "Create a game and attach PDFs in override order."
                : "Start by adding PDFs to your library."
            }
            action={
              canCreateGame ? (
                <button type="button" className="btn primary" onClick={() => setShowForm(true)}>
                  Create your first game
                </button>
              ) : (
                <Link to="/library" className="btn primary">
                  Go to PDF Library
                </Link>
              )
            }
          />
        ) : (
          <ul className="item-list">
            {games.map((game) => (
              <li key={game.id}>
                <Link to={`/games/${game.id}`} className="item-link">
                  <div className="item-title">
                    {game.name}
                    {game.running && <RunningBadge />}
                    {game.archived && <span className="badge badge-archived">Archived</span>}
                  </div>
                  <div className="item-meta">
                    <span>{game.pdf_count} PDFs</span>
                    <span>{new Date(game.updated_at).toLocaleDateString()}</span>
                  </div>
                </Link>
              </li>
            ))}
          </ul>
        )}
      </aside>

      <section className="detail-pane muted-pane">
        <EmptyState title="Select a game" description="Choose a game to manage attached PDFs, notes, and running status." />
      </section>
    </div>
  );
}
