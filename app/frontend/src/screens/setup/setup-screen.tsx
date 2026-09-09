import { ChevronRight, Server } from "lucide-react";
import { useCallback, useEffect, useRef, useState } from "react";

import { Field } from "@/components/field";
import { BoxNote, InputBox } from "@/components/input-box";
import { FootNote, Screen, ScreenBody, ScreenFoot, ScreenHead } from "@/components/screen";
import { SectionTitle, StepDot } from "@/components/section-header";
import { toast } from "@/components/ui/sonner";
import {
  Accordion,
  AccordionContent,
  AccordionItem,
  AccordionTrigger,
} from "@/components/ui/accordion";
import { Alert } from "@/components/ui/alert";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { usePasswordStrength } from "@/hooks/use-password-strength";
import { bridge } from "@/lib/bridge";
import { errorText, normaliseHost } from "@/lib/format";
import { cn } from "@/lib/utils";
import { useCare, type SetupStep } from "@/state/care-store";
import type { SetupForm } from "@/state/forms";
import { CheckRows } from "./check-rows";
import { PasswordPair } from "./password-pair";
import { useRequirementChecks } from "./use-requirement-checks";

const ADDRESS_INFO =
  "The address staff type in their browser. Change it only if another CARE clinic is already running on this WiFi, so the two names do not clash.";
const BACKUP_INFO =
  "Daily backups are encrypted with this password. If it is lost, the backups cannot be opened by anyone, including us. Store it somewhere safe.";
const ADMIN_INFO =
  "This is the first login for the clinic. Add staff accounts later from inside CARE.";

export function SetupScreen({
  form,
  patch,
}: {
  form: SetupForm;
  patch: (values: Partial<SetupForm>) => void;
}) {
  const { openStep, setOpenStep, setStepDone, goToClinicDetails } = useCare();
  const host = normaliseHost(form.hostInput);
  const { checks, overall, recheckAll, checkMDNS, fixNetwork } = useRequirementChecks(host);

  const [expanded, setExpanded] = useState<string>(openStep);
  const [hostProblem, setHostProblem] = useState("");
  const [verifying, setVerifying] = useState(false);
  const [verifyNote, setVerifyNote] = useState("");
  const [showAdminInfo, setShowAdminInfo] = useState(false);

  const adminStrength = usePasswordStrength(form.adminPassword);
  const backupStrength = usePasswordStrength(form.backupPassword);

  const hostOk = hostProblem === "";
  const adminDone =
    adminStrength.strong && form.adminConfirm !== "" && form.adminConfirm === form.adminPassword;
  const backupDone =
    backupStrength.strong &&
    form.backupConfirm !== "" &&
    form.backupConfirm === form.backupPassword;
  const ready = overall === "ok" && hostOk && backupDone && adminDone;

  useEffect(() => setStepDone("checks", overall === "ok"), [overall, setStepDone]);
  useEffect(() => setStepDone("backup", backupDone), [backupDone, setStepDone]);
  useEffect(() => setStepDone("admin", adminDone), [adminDone, setStepDone]);

  // The address is applied before the name check runs, so step 1 verifies the
  // name the clinic actually picked rather than the default.
  const hostTimer = useRef(0);
  const applyHost = useCallback(
    async (raw: string) => {
      const trimmed = raw.trim();
      const problem = await bridge.ValidateDomain(trimmed);
      if (problem) {
        setHostProblem(problem);
        return;
      }
      try {
        await bridge.SetMDNSName(normaliseHost(trimmed));
      } catch (e) {
        setHostProblem(errorText(e));
        return;
      }
      setHostProblem("");
      void checkMDNS();
    },
    [checkMDNS],
  );

  useEffect(() => () => window.clearTimeout(hostTimer.current), []);

  const onHostChange = (value: string) => {
    setVerifyNote("");
    patch({ hostInput: value });
    window.clearTimeout(hostTimer.current);
    hostTimer.current = window.setTimeout(() => void applyHost(value), 400);
  };

  const chooseBackupFolder = async () => {
    const chosen = await bridge.ChooseFolder("Choose backup folder");
    if (chosen) patch({ backupDir: chosen });
  };

  const onContinue = async () => {
    setVerifying(true);
    setVerifyNote("");
    const state = await recheckAll();
    setVerifying(false);
    if (state !== "ok" || !hostOk || !adminDone || !backupDone) {
      setVerifyNote("A step is no longer met — fix it and try again.");
      return;
    }
    // Record the pass before navigating: this screen unmounts on the next
    // commit, so the effect that mirrors `overall` into the rail would never
    // see the result and the step would keep the "Checking" state it was put
    // into a moment ago.
    setStepDone("checks", true);
    goToClinicDetails();
  };

  const issues = checks.filter((c) => c.state === "bad").length;
  const note = verifying
    ? "Re-checking your computer…"
    : verifyNote ||
      (overall !== "ok"
        ? "Waiting for your computer to be ready…"
        : !hostOk
          ? "Fix the clinic address to continue."
          : !backupDone
            ? "Set and confirm the backup password to continue."
            : !adminDone
              ? "Set and confirm the admin password to continue."
              : "Ready. This takes about 10 to 20 minutes.");

  return (
    <Screen>
      <ScreenHead
        title="Set up your clinic"
        subtitle="One time, on this computer. About 15 minutes."
      />

      <ScreenBody>
        <Accordion
          type="single"
          collapsible
          value={expanded}
          onValueChange={(value) => {
            setExpanded(value);
            // Closing a section leaves the rail pointing at it, the way the
            // guided flow reads: you are still on that step.
            if (value) setOpenStep(value as SetupStep);
          }}
        >
          <AccordionItem
            value="checks"
            className={cn(overall === "bad" && "border-danger-line")}
          >
            <AccordionTrigger>
              <StepDot done={overall === "ok"}>1</StepDot>
              <SectionTitle
                title="Computer check"
                summary={
                  overall === "wait"
                    ? "Checking this computer"
                    : overall === "ok"
                      ? "Everything this clinic needs is ready"
                      : "Open to see what failed"
                }
              />
              <Badge variant={overall === "wait" ? "default" : overall}>
                {overall === "wait"
                  ? "Checking"
                  : overall === "ok"
                    ? "All good"
                    : `${issues} issue${issues > 1 ? "s" : ""}`}
              </Badge>
            </AccordionTrigger>
            <AccordionContent>
              <Field
                label="Clinic address"
                htmlFor="mdnsname"
                info={ADDRESS_INFO}
                infoTitle="The address staff type in their browser"
                messages={
                  <span className={hostOk ? undefined : "text-danger-ink"}>
                    {hostOk ? `Staff will open https://${host}` : hostProblem}
                  </span>
                }
              >
                <InputBox tone={hostOk ? "neutral" : "bad"}>
                  <Input
                    id="mdnsname"
                    value={form.hostInput}
                    spellCheck={false}
                    autoCapitalize="none"
                    autoComplete="off"
                    onChange={(e) => onHostChange(e.target.value)}
                    className="h-full flex-1 rounded-none border-none bg-transparent px-0 focus-visible:border-none"
                  />
                  <BoxNote>.local</BoxNote>
                </InputBox>
              </Field>

              <CheckRows
                checks={checks}
                onFixNetwork={async () => {
                  toast(await fixNetwork());
                }}
              />

              <div className="flex items-center gap-2.5">
                <Button
                  onClick={(e) => {
                    e.stopPropagation();
                    setVerifyNote("");
                    void recheckAll();
                  }}
                >
                  Check again
                </Button>
              </div>
            </AccordionContent>
          </AccordionItem>

          <AccordionItem value="backup">
            <AccordionTrigger>
              <StepDot done={backupDone}>2</StepDot>
              <SectionTitle
                title="Backup"
                summary={
                  backupDone
                    ? `${form.backupDir || "Desktop (default)"}, password set`
                    : "Drive and password for daily backups"
                }
              />
              <Badge variant={backupDone ? "ok" : "default"}>
                {backupDone ? "Done" : "To do"}
              </Badge>
            </AccordionTrigger>
            <AccordionContent>
              <div className="flex items-center gap-3 rounded-lg border border-line bg-background px-3.5 py-[13px]">
                <span className="flex size-[30px] flex-none items-center justify-center rounded-sm bg-brand-bg text-brand-ink">
                  <Server className="size-4" strokeWidth={2} />
                </span>
                <div className="min-w-0 flex-1">
                  <div className="text-xs font-semibold tracking-[0.04em] text-muted-foreground uppercase">
                    Drive
                  </div>
                  <div className="truncate font-mono text-[13.5px] font-semibold text-ink">
                    {form.backupDir || "Desktop (default)"}
                  </div>
                </div>
                <Button onClick={() => void chooseBackupFolder()}>Choose</Button>
              </div>

              <Field
                label="Backup password"
                htmlFor="backuppw"
                info={BACKUP_INFO}
                infoTitle="Backups are encrypted with this password"
              >
                <PasswordPair
                  id="backuppw"
                  password={form.backupPassword}
                  confirm={form.backupConfirm}
                  strength={backupStrength}
                  onPasswordChange={(v) => {
                    setVerifyNote("");
                    patch({ backupPassword: v });
                  }}
                  onConfirmChange={(v) => {
                    setVerifyNote("");
                    patch({ backupConfirm: v });
                  }}
                />
              </Field>
            </AccordionContent>
          </AccordionItem>

          <AccordionItem value="admin">
            <AccordionTrigger>
              <StepDot done={adminDone}>3</StepDot>
              <SectionTitle
                title="Admin login"
                summary={adminDone ? "admin, password set" : "The first login for this clinic"}
              />
              <Badge variant={adminDone ? "ok" : "default"}>{adminDone ? "Done" : "To do"}</Badge>
            </AccordionTrigger>
            <AccordionContent>
              <div className="flex items-center gap-2.5">
                <span className="text-xs font-semibold tracking-[0.04em] text-muted-foreground uppercase">
                  Username
                </span>
                <span className="rounded-full bg-brand-bg px-3 py-1 font-mono text-[13.5px] font-semibold text-brand-ink">
                  admin
                </span>
                <button
                  type="button"
                  title="Staff accounts are added later inside CARE"
                  aria-expanded={showAdminInfo}
                  onClick={() => setShowAdminInfo((v) => !v)}
                  className="flex size-[17px] shrink-0 cursor-pointer items-center justify-center rounded-full border border-[#d1d5db] bg-white p-0 font-mono text-[11px] leading-none font-bold text-muted-foreground hover:border-brand hover:text-brand-ink"
                >
                  i
                </button>
              </div>
              {showAdminInfo ? <Alert>{ADMIN_INFO}</Alert> : null}

              <div>
                <Label htmlFor="adminpw" className="mb-2 block">
                  Password
                </Label>
                <PasswordPair
                  id="adminpw"
                  password={form.adminPassword}
                  confirm={form.adminConfirm}
                  strength={adminStrength}
                  onPasswordChange={(v) => {
                    setVerifyNote("");
                    patch({ adminPassword: v });
                  }}
                  onConfirmChange={(v) => {
                    setVerifyNote("");
                    patch({ adminConfirm: v });
                  }}
                />
              </div>
            </AccordionContent>
          </AccordionItem>
        </Accordion>
      </ScreenBody>

      <ScreenFoot>
        <Button
          variant="primary"
          size="lg"
          className="shadow-lift disabled:shadow-none"
          disabled={!ready || verifying}
          onClick={() => void onContinue()}
        >
          Continue
          <ChevronRight className="size-[17px]" strokeWidth={2.2} />
        </Button>
        <FootNote>{note}</FootNote>
      </ScreenFoot>
    </Screen>
  );
}
