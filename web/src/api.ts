export type PathStatus = "ok" | "missing";
export type IndexStatus = "none" | "indexing" | "indexed" | "error";

export interface PDF {
  id: string;
  title: string;
  file_path: string;
  page_count: number;
  thumbnail_path?: string;
  index_status: IndexStatus;
  indexed_at?: string;
  created_at: string;
  updated_at: string;
}

export interface PDFSummary extends PDF {
  path_status: PathStatus;
  section_count: number;
  game_count: number;
}

export interface TOCSection {
  id: string;
  pdf_id: string;
  title: string;
  start_page: number;
  end_page: number;
  sort_order: number;
  indexed?: boolean;
}

export interface CreatePDFResult extends PDFSummary {
  toc_extract_error?: string;
}

export interface ExtractTOCResult {
  sections: TOCSection[];
  page_count: number;
  source?: string;
}

export type TOCImportSource = "pages" | "bookmarks";

export interface IndexResult {
  pdf_id: string;
  index_status: IndexStatus;
  page_count: number;
  sections_indexed: number;
}

export interface IndexProgress {
  pdf_id: string;
  phase: string;
  current: number;
  total: number;
  percent: number;
  message: string;
  active: boolean;
}

export interface PDFGlossaryEntry {
  feature_id: string;
  terms: string[];
}

export interface CheatsheetCitation {
  section_id?: string;
  section_title: string;
  start_page: number;
  end_page?: number;
}

export interface CheatsheetEntry {
  feature_id: string;
  definition: string;
  citations: CheatsheetCitation[];
}

export interface PDFIndexMeta {
  glossary: PDFGlossaryEntry[];
  features: string[];
  cheatsheet: CheatsheetEntry[];
  llm_learnings_skipped?: boolean;
}

export interface Game {
  id: string;
  name: string;
  notes: string;
  archived: boolean;
  running: boolean;
  created_at: string;
  updated_at: string;
}

export interface GameSummary extends Game {
  pdf_count: number;
}

export interface GamePDFEntry extends PDF {
  sort_order: number;
  path_status: PathStatus;
  section_count: number;
}

export interface GameDetail extends GameSummary {
  pdfs: GamePDFEntry[];
}

export interface RunningStatus {
  game: Game | null;
  player_url: string;
  player_note: string;
}

export class ApiError extends Error {
  constructor(message: string) {
    super(message);
    this.name = "ApiError";
  }
}

async function request<T>(path: string, init?: RequestInit): Promise<T> {
  const res = await fetch(path, {
    headers: { "Content-Type": "application/json", ...init?.headers },
    ...init,
  });
  if (!res.ok) {
    let message = res.statusText;
    try {
      const body = await res.json();
      if (body?.error) message = body.error;
    } catch {
      // ignore
    }
    throw new ApiError(message);
  }
  if (res.status === 204) return undefined as T;
  return res.json();
}

async function uploadPDF(data: {
  title: string;
  file: File;
  toc_source?: TOCImportSource;
  toc_start_page?: number;
  toc_end_page?: number;
  toc_bookmark_depth?: number;
  /** @deprecated use toc_bookmark_depth */
  toc_include_children?: boolean;
}): Promise<CreatePDFResult> {
  const form = new FormData();
  form.append("file", data.file);
  form.append("title", data.title);
  if (data.toc_source) form.append("toc_source", data.toc_source);
  if (data.toc_start_page != null) form.append("toc_start_page", String(data.toc_start_page));
  if (data.toc_end_page != null) form.append("toc_end_page", String(data.toc_end_page));
  if (data.toc_bookmark_depth != null) {
    form.append("toc_bookmark_depth", String(data.toc_bookmark_depth));
  }

  const res = await fetch("/api/pdfs/upload", { method: "POST", body: form });
  if (!res.ok) {
    let message = res.statusText;
    try {
      const body = await res.json();
      if (body?.error) message = body.error;
    } catch {
      // ignore
    }
    throw new ApiError(message);
  }
  return res.json();
}

export const api = {
  listPDFs: () => request<PDFSummary[]>("/api/pdfs"),
  getPDF: (id: string) => request<PDFSummary>(`/api/pdfs/${id}`),
  createPDF: (data: {
    title: string;
    file_path: string;
    toc_source?: TOCImportSource;
    toc_start_page?: number;
    toc_end_page?: number;
    toc_bookmark_depth?: number;
    /** @deprecated use toc_bookmark_depth */
    toc_include_children?: boolean;
  }) => request<CreatePDFResult>("/api/pdfs", { method: "POST", body: JSON.stringify(data) }),
  uploadPDF,
  updatePDF: (id: string, data: Partial<Pick<PDF, "title" | "file_path" | "page_count">>) =>
    request<PDFSummary>(`/api/pdfs/${id}`, { method: "PATCH", body: JSON.stringify(data) }),
  deletePDF: (id: string) => request<void>(`/api/pdfs/${id}`, { method: "DELETE" }),
  probePDF: (id: string) => request<{ path_status: PathStatus; page_count: number }>(`/api/pdfs/${id}/probe`, { method: "POST" }),
  getTOC: (id: string) => request<TOCSection[]>(`/api/pdfs/${id}/toc`),
  saveTOC: (id: string, sections: TOCSection[]) =>
    request<TOCSection[]>(`/api/pdfs/${id}/toc`, { method: "PUT", body: JSON.stringify({ sections }) }),
  extractTOC: (id: string, options: {
    source: TOCImportSource;
    start_page?: number;
    end_page?: number;
    bookmark_max_depth?: number;
    /** @deprecated use bookmark_max_depth */
    include_children?: boolean;
  }) =>
    request<ExtractTOCResult>(`/api/pdfs/${id}/toc/extract`, {
      method: "POST",
      body: JSON.stringify(options),
    }),
  indexPDF: (id: string) => request<IndexResult>(`/api/pdfs/${id}/index`, { method: "POST" }),
  getIndexProgress: (id: string) => request<IndexProgress>(`/api/pdfs/${id}/index/progress`),
  getIndexMeta: (id: string) => request<PDFIndexMeta>(`/api/pdfs/${id}/index-meta`),
  saveIndexMeta: (id: string, meta: PDFIndexMeta) =>
    request<PDFIndexMeta>(`/api/pdfs/${id}/index-meta`, { method: "PUT", body: JSON.stringify(meta) }),
  rebuildCheatsheet: (id: string) =>
    request<PDFIndexMeta>(`/api/pdfs/${id}/index-meta/rebuild-cheatsheet`, { method: "POST" }),
  thumbnailURL: (id: string) => `/api/pdfs/${id}/thumbnail`,

  listGames: (filter: "active" | "archived" | "all" = "active") =>
    request<GameSummary[]>(`/api/games?filter=${filter}`),
  getGame: (id: string) => request<GameDetail>(`/api/games/${id}`),
  createGame: (data: { name: string }) =>
    request<GameSummary>("/api/games", { method: "POST", body: JSON.stringify(data) }),
  updateGame: (id: string, data: Partial<Pick<Game, "name" | "notes">>) =>
    request<Game>(`/api/games/${id}`, { method: "PATCH", body: JSON.stringify(data) }),
  archiveGame: (id: string) => request<Game>(`/api/games/${id}/archive`, { method: "POST" }),
  restoreGame: (id: string) => request<Game>(`/api/games/${id}/restore`, { method: "POST" }),
  setGamePDFs: (id: string, pdf_ids: string[]) =>
    request<GamePDFEntry[]>(`/api/games/${id}/pdfs`, { method: "PUT", body: JSON.stringify({ pdf_ids }) }),
  setRunning: (id: string, running: boolean) =>
    request<RunningStatus>(`/api/games/${id}/running`, { method: "POST", body: JSON.stringify({ running }) }),
  getRunning: () => request<RunningStatus>("/api/running"),
};
