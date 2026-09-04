import { ChatSource, ChatSearchDebug } from "./api";
import { ChatActivityStep, parseChatActivityStep } from "./ChatActivityPanel";

function decodeBase64UTF8(raw: string): string {
  const binary = atob(raw);
  const bytes = Uint8Array.from(binary, (c) => c.charCodeAt(0));
  return new TextDecoder().decode(bytes);
}

function parseEncodedHeader<T>(response: Response, header: string, encHeader: string): T | null {
  const raw = response.headers.get(header);
  if (!raw) return null;
  try {
    const enc = response.headers.get(encHeader);
    const json = enc === "base64" ? decodeBase64UTF8(raw) : raw;
    return JSON.parse(json) as T;
  } catch {
    return null;
  }
}

export function parseSourcesHeader(response: Response): ChatSource[] {
  const parsed = parseEncodedHeader<ChatSource[]>(response, "X-RPG-Sources", "X-RPG-Sources-Enc");
  return Array.isArray(parsed) ? parsed : [];
}

export function parseSearchDebugHeader(response: Response): ChatSearchDebug | null {
  return parseEncodedHeader<ChatSearchDebug>(response, "X-RPG-Search-Debug", "X-RPG-Search-Debug-Enc");
}

type ChatDataPartHandlers = {
  onStatus?: (message: string) => void;
  onActivity?: (step: ChatActivityStep) => void;
  onSources?: (sources: ChatSource[]) => void;
  onSearchDebug?: (debug: ChatSearchDebug) => void;
};

function isRecord(value: unknown): value is Record<string, unknown> {
  return value !== null && typeof value === "object";
}

export function applyChatDataParts(
  parts: readonly unknown[] | undefined,
  handlers: ChatDataPartHandlers,
  fromIndex = 0,
): number {
  if (!parts?.length) {
    return fromIndex;
  }
  for (let i = fromIndex; i < parts.length; i++) {
    applyChatDataPart(parts[i], handlers);
  }
  return parts.length;
}

export function applyChatDataPart(dataPart: unknown, handlers: ChatDataPartHandlers): void {
  if (!isRecord(dataPart) || typeof dataPart.type !== "string") {
    return;
  }
  switch (dataPart.type) {
    case "chat-status":
      if (typeof dataPart.message === "string") {
        handlers.onStatus?.(dataPart.message);
      }
      break;
    case "chat-activity":
      if (isRecord(dataPart.step)) {
        const step = parseChatActivityStep(dataPart.step);
        if (step) {
          handlers.onActivity?.(step);
        }
      }
      break;
    case "sources": {
      const sources = dataPart.sources;
      if (Array.isArray(sources)) {
        handlers.onSources?.(sources as ChatSource[]);
      }
      break;
    }
    case "search-debug":
      if (isRecord(dataPart.debug)) {
        handlers.onSearchDebug?.(dataPart.debug as ChatSearchDebug);
      }
      break;
  }
}
