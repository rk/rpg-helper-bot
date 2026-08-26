import { NavLink, Navigate, Route, Routes } from "react-router-dom";
import Layout from "./components/Layout";
import GameDetailPage from "./pages/GameDetailPage";
import GamesPage from "./pages/GamesPage";
import LibraryDetailPage from "./pages/LibraryDetailPage";
import LibraryPage from "./pages/LibraryPage";

export default function App() {
  return (
    <Layout>
      <Routes>
        <Route path="/" element={<Navigate to="/library" replace />} />
        <Route path="/library" element={<LibraryPage />} />
        <Route path="/library/:pdfId" element={<LibraryDetailPage />} />
        <Route path="/games" element={<GamesPage />} />
        <Route path="/games/:gameId" element={<GameDetailPage />} />
      </Routes>
    </Layout>
  );
}

export function NavTabs() {
  return (
    <nav className="nav-tabs">
      <NavLink to="/library" className={({ isActive }) => (isActive ? "active" : "")}>
        PDF Library
      </NavLink>
      <NavLink to="/games" className={({ isActive }) => (isActive ? "active" : "")}>
        Games
      </NavLink>
    </nav>
  );
}
