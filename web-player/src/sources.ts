import { ChatSource } from "./api";

function decodeBase64UTF8(raw: string): string {
  const binary = atob(raw);
  const bytes = Uint8Array.from(binary, (c) => c.charCodeAt(0));
  return new TextDecoder().decode(bytes);
}

export function parseSourcesHeader(response: Response): ChatSource[] {
  const raw = response.headers.get("X-RPG-Sources");
  if (!raw) return [];

  try {
    const enc = response.headers.get("X-RPG-Sources-Enc");
    const json = enc === "base64" ? decodeBase64UTF8(raw) : raw;
    const parsed = JSON.parse(json);
    return Array.isArray(parsed) ? parsed : [];
  } catch {
    return [];
  }
}
