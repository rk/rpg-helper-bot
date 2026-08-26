import { ReactNode } from "react";
import { NavTabs } from "../App";

export default function Layout({ children }: { children: ReactNode }) {
  return (
    <div className="app-shell">
      <header className="app-header">
        <div className="brand">
          <span className="brand-mark">RPG</span>
          <span className="brand-name">Helper Bot</span>
        </div>
        <NavTabs />
      </header>
      <main className="app-main">{children}</main>
    </div>
  );
}
