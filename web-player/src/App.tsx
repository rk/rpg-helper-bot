import { useChat } from "@ai-sdk/react";
import { FormEvent, useCallback, useEffect, useRef, useState } from "react";
import { ChatSource, fetchRunning, RunningGame, ChatSearchDebug } from "./api";
import { MarkdownMessage } from "./MarkdownMessage";
import { SearchDebugPanel } from "./SearchDebugPanel";
import { SourceList } from "./SourceFlyover";
import {
  applyChatDataParts,
  parseSearchDebugHeader,
  parseSourcesHeader,
} from "./sources";
import { ChatActivityPanel, ChatActivityStep } from "./ChatActivityPanel";

export default function App() {
  const [game, setGame] = useState<RunningGame | null>(null);
  const [sourcesByMessageId, setSourcesByMessageId] = useState<Record<string, ChatSource[]>>({});
  const [searchDebugByMessageId, setSearchDebugByMessageId] = useState<Record<string, ChatSearchDebug | null>>({});
  const [activityByMessageId, setActivityByMessageId] = useState<Record<string, ChatActivityStep[]>>({});
  const [chatStatus, setChatStatus] = useState<string | null>(null);
  const [chatActivity, setChatActivity] = useState<ChatActivityStep[]>([]);
  const [loadError, setLoadError] = useState("");
  const pendingSources = useRef<ChatSource[]>([]);
  const pendingSearchDebug = useRef<ChatSearchDebug | null>(null);
  const pendingActivity = useRef<ChatActivityStep[]>([]);
  const processedStreamDataCount = useRef(0);
  const streamDataBaseIndex = useRef(0);
  const streamDataRef = useRef<readonly unknown[] | undefined>();

  const appendActivity = (step: ChatActivityStep) => {
    pendingActivity.current = [...pendingActivity.current, step];
    setChatActivity(pendingActivity.current);
  };

  const streamDataHandlers = useCallback(
    () => ({
      onStatus: setChatStatus,
      onActivity: appendActivity,
      onSources: (sources: ChatSource[]) => {
        pendingSources.current = sources;
      },
      onSearchDebug: (debug: ChatSearchDebug) => {
        pendingSearchDebug.current = debug;
      },
    }),
    [],
  );

  const consumeStreamData = useCallback(
    (parts: readonly unknown[] | undefined, fromIndex: number) =>
      applyChatDataParts(parts, streamDataHandlers(), fromIndex),
    [streamDataHandlers],
  );

  const { messages, data: streamData, input, handleInputChange, handleSubmit, isLoading, error } =
    useChat({
    api: "/api/chat",
    streamProtocol: "data",
    onResponse(response) {
      pendingSources.current = parseSourcesHeader(response);
      pendingSearchDebug.current = parseSearchDebugHeader(response);
    },
    onFinish(message) {
      processedStreamDataCount.current = consumeStreamData(
        streamDataRef.current,
        processedStreamDataCount.current,
      );
      setChatStatus(null);
      if (message.role === "assistant") {
        setSourcesByMessageId((prev) => ({
          ...prev,
          [message.id]: pendingSources.current,
        }));
        setSearchDebugByMessageId((prev) => ({
          ...prev,
          [message.id]: pendingSearchDebug.current,
        }));
        setActivityByMessageId((prev) => ({
          ...prev,
          [message.id]: pendingActivity.current,
        }));
      }
    },
  });

  streamDataRef.current = streamData;

  useEffect(() => {
    processedStreamDataCount.current = consumeStreamData(
      streamData,
      processedStreamDataCount.current,
    );
  }, [streamData, consumeStreamData]);

  useEffect(() => {
    fetchRunning()
      .then((r) => setGame(r.game))
      .catch((e) => setLoadError(e instanceof Error ? e.message : "Failed to load"));
  }, []);

  const onSubmit = (e: FormEvent) => {
    e.preventDefault();
    setChatStatus(null);
    setChatActivity([]);
    pendingActivity.current = [];
    streamDataBaseIndex.current = streamDataRef.current?.length ?? 0;
    processedStreamDataCount.current = streamDataBaseIndex.current;
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
          const msgActivity = m.role === "assistant" ? activityByMessageId[m.id] ?? [] : [];
          return (
            <div key={m.id} className={`msg msg-${m.role}`}>
              <div className="msg-role">{m.role === "user" ? "You" : "Assistant"}</div>
              {m.role === "assistant" ? (
                <>
                  <ChatActivityPanel steps={msgActivity} />
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
        {isLoading && (
          <ChatActivityPanel steps={chatActivity} live status={chatStatus} />
        )}
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
