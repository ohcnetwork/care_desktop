import { Database } from "lucide-react";
import { Fragment, useState } from "react";

import { InfoButton } from "@/components/field";
import { Alert } from "@/components/ui/alert";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { megabytes } from "@/lib/format";
import { cn } from "@/lib/utils";
import { useCare } from "@/state/care-store";

const RESTORE_INFO =
  "Restoring replaces today's data with the chosen copy. CARE pauses for a moment while it restores, and you confirm before anything changes.";

export function BackupsTab() {
  const { backups, busy, runAction, reloadBackups, restore } = useCare();
  const [showInfo, setShowInfo] = useState(false);
  const [confirming, setConfirming] = useState<string | null>(null);

  return (
    <div className="flex flex-col gap-3">
      <div className="flex items-center gap-[11px]">
        <Button variant="primary" disabled={busy} onClick={() => void runAction("backup-now")}>
          Back up now
        </Button>
        <span className="text-[13px] text-muted-foreground">Automatic, daily</span>
        <InfoButton
          title="Restoring replaces current data"
          pressed={showInfo}
          onClick={() => setShowInfo((v) => !v)}
        />
        <span className="flex-1" />
        <span className="text-[13px] text-muted-foreground">
          {backups.length ? `${backups.length} kept` : ""}
        </span>
        <Button
          disabled={busy}
          onClick={() => {
            setConfirming(null);
            void reloadBackups();
          }}
        >
          Refresh
        </Button>
      </div>

      {showInfo ? <Alert>{RESTORE_INFO}</Alert> : null}

      <div className="overflow-hidden rounded-xl border border-line bg-card shadow-card">
        {backups.length === 0 ? (
          <div className="p-5 text-center text-[13px] text-faint">
            No backups yet. Click <b>Back up now</b> or wait for the daily backup.
          </div>
        ) : (
          backups.map((backup, i) => {
            const meta = [
              megabytes(backup.size_bytes),
              backup.files_archive ? "database and files" : "database only",
              backup.encrypted ? "encrypted" : "",
            ]
              .filter(Boolean)
              .join("  ·  ");
            return (
              <Fragment key={backup.db_dump}>
                <div
                  className={cn(
                    "flex items-center gap-[13px] px-4 py-3.5",
                    i > 0 && "border-t border-hair",
                  )}
                >
                  <span className="flex size-[30px] flex-none items-center justify-center rounded-sm bg-hair text-muted-foreground">
                    <Database className="size-[15px]" strokeWidth={2} />
                  </span>
                  <div className="min-w-0 flex-1">
                    <div className="font-mono text-[13.5px] font-semibold text-ink">
                      {backup.label}
                    </div>
                    <div className="mt-0.5 text-[12.5px] text-muted-foreground">{meta}</div>
                  </div>
                  <Badge variant={backup.manual ? "plainOk" : "plain"} size="sm">
                    {backup.manual ? "Manual" : "Automatic"}
                  </Badge>
                  <Button disabled={busy} onClick={() => setConfirming(backup.db_dump)}>
                    Restore
                  </Button>
                </div>
                {confirming === backup.db_dump ? (
                  <div className="flex items-center gap-3 border-t border-danger-bg bg-danger-tint px-4 py-[13px] text-[12.5px] text-danger-ink">
                    <span className="flex-1">
                      Replace current data with this copy? This cannot be undone.
                    </span>
                    <Button onClick={() => setConfirming(null)}>Cancel</Button>
                    <Button
                      variant="destructive"
                      onClick={() => {
                        setConfirming(null);
                        void restore(backup);
                      }}
                    >
                      Yes, restore
                    </Button>
                  </div>
                ) : null}
              </Fragment>
            );
          })
        )}
      </div>
    </div>
  );
}
