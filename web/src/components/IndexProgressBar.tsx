import { useIndexing } from "./IndexingContext";

export default function IndexProgressBar() {
  const { progress } = useIndexing();
  if (!progress) return null;

  const pct = Math.min(100, Math.max(0, progress.percent));
  const indeterminate = progress.active && pct < 3;

  return (
    <div className="index-progress" role="progressbar" aria-valuenow={pct} aria-valuemin={0} aria-valuemax={100}>
      <div
        className={`index-progress-bar${indeterminate ? " indeterminate" : ""}`}
        style={indeterminate ? undefined : { width: `${pct}%` }}
      />
      {progress.message && progress.active && (
        <span className="index-progress-label">{progress.message}</span>
      )}
    </div>
  );
}
