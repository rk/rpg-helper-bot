import { createContext, ReactNode, useCallback, useContext, useMemo, useRef, useState } from "react";
import { api, IndexProgress } from "../api";

type IndexingContextValue = {
  progress: IndexProgress | null;
  runIndex: (pdfId: string, task: () => Promise<void>) => Promise<void>;
};

const IndexingContext = createContext<IndexingContextValue | null>(null);

export function IndexingProvider({ children }: { children: ReactNode }) {
  const [progress, setProgress] = useState<IndexProgress | null>(null);
  const pollRef = useRef<number | null>(null);

  const stopPolling = useCallback(() => {
    if (pollRef.current != null) {
      window.clearInterval(pollRef.current);
      pollRef.current = null;
    }
  }, []);

  const runIndex = useCallback(
    async (pdfId: string, task: () => Promise<void>) => {
      stopPolling();
      setProgress({
        pdf_id: pdfId,
        phase: "thumbnail",
        current: 0,
        total: 0,
        percent: 2,
        message: "Starting index…",
        active: true,
      });

      pollRef.current = window.setInterval(async () => {
        try {
          const snap = await api.getIndexProgress(pdfId);
          if (snap.active || snap.phase !== "idle") {
            setProgress(snap);
          }
        } catch {
          // Ignore transient poll errors while index runs.
        }
      }, 350);

      try {
        await task();
        try {
          const final = await api.getIndexProgress(pdfId);
          setProgress(final.phase === "done" ? { ...final, percent: 100, active: false } : final);
        } catch {
          setProgress((p) => (p ? { ...p, percent: 100, phase: "done", active: false, message: "Complete" } : p));
        }
      } finally {
        stopPolling();
        window.setTimeout(() => setProgress(null), 600);
      }
    },
    [stopPolling],
  );

  const value = useMemo(() => ({ progress, runIndex }), [progress, runIndex]);

  return <IndexingContext.Provider value={value}>{children}</IndexingContext.Provider>;
}

export function useIndexing() {
  const ctx = useContext(IndexingContext);
  if (!ctx) throw new Error("useIndexing must be used within IndexingProvider");
  return ctx;
}
