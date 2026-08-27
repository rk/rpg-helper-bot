import { ReactNode } from "react";
import { NavTabs } from "../App";
import IndexProgressBar from "./IndexProgressBar";

export default function Layout({ children }: { children: ReactNode }) {
  return (
    <div className="app-shell">
      <IndexProgressBar />
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
