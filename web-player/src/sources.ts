import { ChatSource, ChatSearchDebug } from "./api";

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
