import { StrictMode } from "react";
import { createRoot } from "react-dom/client";
import { MuiKit } from "@dsherwin/mui-kit";
import "./styles.css";
import App from "./App";

createRoot(document.getElementById("root")!).render(
  <StrictMode>
    <MuiKit>
      <App />
    </MuiKit>
  </StrictMode>,
);
