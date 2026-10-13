import React from "react";
import { createRoot } from "react-dom/client";
import App from "./App";
import OverlayPage from "./pages/OverlayPage";
import "./style.css";

// จุดเริ่ม frontend: mount React ลง #root และเลือกหน้า OBS ตาม URL
createRoot(document.getElementById("root")).render(
  location.pathname === "/overlay" ? <OverlayPage /> : <App />,
);
