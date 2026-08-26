import { PathStatus } from "../api";

export function PathBadge({ status }: { status: PathStatus }) {
  if (status === "ok") return <span className="badge badge-ok">Path OK</span>;
  return <span className="badge badge-missing">Path missing</span>;
}

export function RunningBadge() {
  return <span className="badge badge-running">Running</span>;
}

export function EmptyState({
  title,
  description,
  action,
}: {
  title: string;
  description: string;
  action?: React.ReactNode;
}) {
  return (
    <div className="empty-state">
      <h2>{title}</h2>
      <p>{description}</p>
      {action}
    </div>
  );
}

export function truncatePath(path: string, max = 48) {
  if (path.length <= max) return path;
  const head = Math.ceil(max * 0.4);
  const tail = max - head - 1;
  return `${path.slice(0, head)}…${path.slice(-tail)}`;
}
