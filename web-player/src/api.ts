export interface ChatSource {
  section_id: string;
  pdf_title: string;
  section_title: string;
  start_page: number;
  end_page: number;
  snippet?: string;
}

export interface RunningGame {
  id: string;
  name: string;
  notes: string;
}

export async function fetchRunning(): Promise<{ game: RunningGame | null; player_url: string }> {
  const res = await fetch("/api/running");
  if (!res.ok) throw new Error("Failed to load running game");
  return res.json();
}
