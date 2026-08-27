import { useChat } from "@ai-sdk/react";
import { FormEvent, useEffect, useRef, useState } from "react";
import { ChatSource, fetchRunning, RunningGame, ChatSearchDebug } from "./api";
import { MarkdownMessage } from "./MarkdownMessage";
import { SearchDebugPanel } from "./SearchDebugPanel";
import { SourceList } from "./SourceFlyover";
import { parseSearchDebugHeader, parseSourcesHeader } from "./sources";

export default function App() {
  const [game, setGame] = useState<RunningGame | null>(null);
  const [sourcesByMessageId, setSourcesByMessageId] = useState<Record<string, ChatSource[]>>({});
  const [searchDebugByMessageId, setSearchDebugByMessageId] = useState<Record<string, ChatSearchDebug | null>>({});
  const [loadError, setLoadError] = useState("");
  const pendingSources = useRef<ChatSource[]>([]);
  const pendingSearchDebug = useRef<ChatSearchDebug | null>(null);

  const { messages, input, handleInputChange, handleSubmit, isLoading, error } = useChat({
    api: "/api/chat",
    streamProtocol: "text",
    onResponse(response) {
      pendingSources.current = parseSourcesHeader(response);
      pendingSearchDebug.current = parseSearchDebugHeader(response);
    },
    onFinish(message) {
      if (message.role === "assistant") {
        setSourcesByMessageId((prev) => ({
          ...prev,
          [message.id]: pendingSources.current,
        }));
        setSearchDebugByMessageId((prev) => ({
          ...prev,
          [message.id]: pendingSearchDebug.current,
        }));
      }
    },
  });

  useEffect(() => {
    fetchRunning()
      .then((r) => setGame(r.game))
      .catch((e) => setLoadError(e instanceof Error ? e.message : "Failed to load"));
  }, []);

  const onSubmit = (e: FormEvent) => {
    e.preventDefault();
    handleSubmit(e);
  };

  return (
    <div className="player-shell">
      <header className="player-header">
        <h1>Rules Q&amp;A</h1>
        {game ? <p className="game-name">{game.name}</p> : <p className="warn">No running game</p>}
      </header>

      {loadError && <p className="error">{loadError}</p>}

      <main className="chat-area">
        {messages.length === 0 && (
          <p className="hint">Ask a rules question. Answers cite indexed PDF sections from this game.</p>
        )}
        {messages.map((m) => {
          const msgSources = m.role === "assistant" ? sourcesByMessageId[m.id] ?? [] : [];
          const msgSearchDebug = m.role === "assistant" ? searchDebugByMessageId[m.id] ?? null : null;
          return (
            <div key={m.id} className={`msg msg-${m.role}`}>
              <div className="msg-role">{m.role === "user" ? "You" : "Assistant"}</div>
              {m.role === "assistant" ? (
                <>
                  <SearchDebugPanel debug={msgSearchDebug} />
                  <MarkdownMessage content={m.content} sources={msgSources} />
                  <SourceList sources={msgSources} />
                </>
              ) : (
                <div className="msg-body">{m.content}</div>
              )}
            </div>
          );
        })}
        {isLoading && <p className="hint">Searching rules and composing answer…</p>}
        {error && <p className="error">{error.message}</p>}
      </main>

      <form className="chat-form" onSubmit={onSubmit}>
        <input
          value={input}
          onChange={handleInputChange}
          placeholder="How does…?"
          disabled={!game || isLoading}
        />
        <button type="submit" disabled={!game || isLoading || !input.trim()}>
          Ask
        </button>
      </form>
    </div>
  );
}
