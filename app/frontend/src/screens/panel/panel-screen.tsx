import { ArrowUpRight } from "lucide-react";
import { useEffect } from "react";

import { Screen, ScreenBody } from "@/components/screen";
import { Button } from "@/components/ui/button";
import { bridge } from "@/lib/bridge";
import { useCare, type PanelTab } from "@/state/care-store";
import { AdvancedTab } from "./advanced-tab";
import { BackupsTab } from "./backups-tab";
import { OverviewTab } from "./overview-tab";

const TAB_META: Record<PanelTab, { title: string; subtitle: string }> = {
  overview: { title: "Overview", subtitle: "Your clinic server at a glance." },
  backups: { title: "Backups", subtitle: "Safe copies of your patient data." },
  advanced: { title: "Advanced", subtitle: "Technical options for this clinic." },
};

export function PanelScreen() {
  const { tab, mdnsName, reloadBackups } = useCare();
  const meta = TAB_META[tab];

  // Opening the tab is the refresh gesture, as it was before.
  useEffect(() => {
    if (tab === "backups") void reloadBackups();
  }, [tab, reloadBackups]);

  return (
    <Screen>
      <div className="flex items-center gap-4 px-[34px] pt-[26px] pb-[18px]">
        <div className="min-w-0 flex-1">
          <h1 className="text-[23px] font-bold tracking-[-0.015em] text-ink">{meta.title}</h1>
          <p className="mt-[5px] text-[13.5px] text-muted-foreground">{meta.subtitle}</p>
        </div>
        <Button variant="soft" onClick={() => void bridge.OpenURL(`https://${mdnsName}/`)}>
          <span className="font-mono">{mdnsName}</span>
          <ArrowUpRight className="size-3.5" strokeWidth={2.2} />
        </Button>
      </div>

      {/* All three stay mounted so half-finished edits survive a tab switch. */}
      <ScreenBody>
        <div hidden={tab !== "overview"}>
          <OverviewTab />
        </div>
        <div hidden={tab !== "backups"}>
          <BackupsTab />
        </div>
        <div hidden={tab !== "advanced"}>
          <AdvancedTab />
        </div>
      </ScreenBody>
    </Screen>
  );
}
