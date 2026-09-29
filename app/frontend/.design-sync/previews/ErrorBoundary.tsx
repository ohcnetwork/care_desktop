import { ErrorBoundary } from "care-desktop-ui";

function Broken(): never {
  const error = new Error("Cannot read properties of undefined (reading 'drives')");
  error.stack = [
    "TypeError: Cannot read properties of undefined (reading 'drives')",
    "    at StorageCard (overview-tab.tsx:214:31)",
    "    at OverviewTab (overview-tab.tsx:61:7)",
    "    at PanelScreen (panel-screen.tsx:88:11)",
  ].join("\n");
  throw error;
}

export const Crashed = () => (
  <div className="h-[420px] w-[680px]">
    <ErrorBoundary>
      <Broken />
    </ErrorBoundary>
  </div>
);
