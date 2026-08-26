import { createContext, ReactNode, useCallback, useContext, useState } from "react";

type Toast = { id: number; message: string; kind: "error" | "info" };

const ToastContext = createContext<{
  showError: (message: string) => void;
  showInfo: (message: string) => void;
} | null>(null);

export function ToastProvider({ children }: { children: ReactNode }) {
  const [toasts, setToasts] = useState<Toast[]>([]);

  const push = useCallback((message: string, kind: Toast["kind"]) => {
    const id = Date.now();
    setToasts((prev) => [...prev, { id, message, kind }]);
    setTimeout(() => {
      setToasts((prev) => prev.filter((t) => t.id !== id));
    }, 5000);
  }, []);

  const showError = useCallback((message: string) => push(message, "error"), [push]);
  const showInfo = useCallback((message: string) => push(message, "info"), [push]);

  return (
    <ToastContext.Provider value={{ showError, showInfo }}>
      {children}
      <div className="toast-stack">
        {toasts.map((t) => (
          <div key={t.id} className={`toast toast-${t.kind}`}>
            {t.message}
          </div>
        ))}
      </div>
    </ToastContext.Provider>
  );
}

export function useToast() {
  const ctx = useContext(ToastContext);
  if (!ctx) throw new Error("useToast must be used within ToastProvider");
  return ctx;
}
