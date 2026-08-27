export const BOOKMARK_DEPTH_OPTIONS = [
  { value: 1, label: "Top level only" },
  { value: 2, label: "2 levels deep" },
  { value: 3, label: "3 levels deep" },
  { value: 4, label: "4 levels deep" },
  { value: 5, label: "5 levels deep" },
  { value: 0, label: "All nested levels" },
] as const;

export const DEFAULT_BOOKMARK_MAX_DEPTH = 2;

export function bookmarkDepthLabel(depth: number): string {
  const opt = BOOKMARK_DEPTH_OPTIONS.find((o) => o.value === depth);
  return opt?.label ?? `${depth} levels`;
}
