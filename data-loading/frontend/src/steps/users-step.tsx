import { Plus } from "lucide-react";
import { useEffect, useMemo, useState } from "react";

import { ROLE_ORGANIZATIONS, sameName } from "@/care/organizations";
import {
  addUserToDepartment,
  createUser,
  findUser,
  GENDERS,
  listRoles,
  type Gender,
  type Role,
} from "@/care/users";
import { BatchPanel } from "@/components/batch-panel";
import { Field } from "@/components/field";
import { Screen, ScreenBody, ScreenHead } from "@/components/screen";
import { StepFoot } from "@/components/step-foot";
import { Alert } from "@/components/ui/alert";
import { Button } from "@/components/ui/button";
import { Checkbox } from "@/components/ui/checkbox";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { runBatch, type BatchProgress } from "@/lib/batch";
import { errorText, normalisePhone } from "@/lib/format";
import { cn } from "@/lib/utils";
import { useWizard } from "@/state/wizard";

type Row = {
  key: number;
  first_name: string;
  last_name: string;
  username: string;
  email: string;
  phone: string;
  gender: Gender | "";
  role: string;
  departments: string[];
  facilityAdmin: boolean;
};

const FACILITY_ADMIN = "Facility Admin";
const USERNAME = /^[a-zA-Z0-9_-]{3,}$/;
const EMAIL = /^[^\s@]+@[^\s@]+\.[^\s@]+$/;

let nextKey = 1;
const blankRow = (): Row => ({
  key: nextKey++,
  first_name: "",
  last_name: "",
  username: "",
  email: "",
  phone: "",
  gender: "",
  role: "",
  departments: [],
  facilityAdmin: false,
});

function rowErrors(r: Row, all: Row[]): string[] {
  const errors: string[] = [];
  if (!r.first_name.trim()) errors.push("first name");
  if (!r.last_name.trim()) errors.push("last name");
  if (!USERNAME.test(r.username.trim())) errors.push("username (letters, digits, - or _, at least 3)");
  else if (all.some((o) => o !== r && sameName(o.username, r.username))) errors.push("username is repeated");
  if (!EMAIL.test(r.email.trim())) errors.push("email");
  else if (all.some((o) => o !== r && sameName(o.email, r.email))) errors.push("email is repeated");
  if (!/^\+?\d{10,14}$/.test(normalisePhone(r.phone))) errors.push("phone");
  else if (all.some((o) => o !== r && normalisePhone(o.phone) === normalisePhone(r.phone))) errors.push("phone is repeated");
  if (!r.gender) errors.push("gender");
  if (!r.role) errors.push("role");
  return errors;
}

function passwordProblem(pw: string): string {
  if (pw.length < 8) return "At least 8 characters.";
  if (/^\d+$/.test(pw)) return "Not only digits.";
  return "";
}

export function UsersStep() {
  const { progress, complete, skip, update } = useWizard();
  const [roles, setRoles] = useState<Role[]>([]);
  const [rolesProblem, setRolesProblem] = useState("");
  const [password, setPassword] = useState("");
  const [showPassword, setShowPassword] = useState(false);
  const [rows, setRows] = useState<Row[]>([blankRow()]);
  const [touched, setTouched] = useState(false);
  const [busy, setBusy] = useState(false);
  const [batch, setBatch] = useState<BatchProgress | null>(null);
  const [problem, setProblem] = useState("");
  const [finished, setFinished] = useState(false);

  useEffect(() => {
    let cancelled = false;
    listRoles()
      .then((list) => {
        if (cancelled) return;
        const known = list.filter((r) => ROLE_ORGANIZATIONS.some((n) => sameName(n, r.name ?? "")));
        const offered = known.length ? known : list;
        offered.sort(
          (a, b) => ROLE_ORGANIZATIONS.findIndex((n) => sameName(n, a.name)) - ROLE_ORGANIZATIONS.findIndex((n) => sameName(n, b.name)),
        );
        setRoles(offered);
      })
      .catch((e) => !cancelled && setRolesProblem(errorText(e)));
    return () => {
      cancelled = true;
    };
  }, []);

  const facilityAdminRole = useMemo(() => roles.find((r) => sameName(r.name, FACILITY_ADMIN)), [roles]);
  const pwProblem = passwordProblem(password);
  const allErrors = rows.map((r) => rowErrors(r, rows));
  const valid = !pwProblem && allErrors.every((e) => e.length === 0);

  const patch = (key: number, p: Partial<Row>) =>
    setRows((list) => list.map((r) => (r.key === key ? { ...r, ...p } : r)));

  const toggleDept = (key: number, name: string) =>
    setRows((list) =>
      list.map((r) =>
        r.key === key
          ? { ...r, departments: r.departments.includes(name) ? r.departments.filter((d) => d !== name) : [...r.departments, name] }
          : r,
      ),
    );

  const run = async () => {
    setTouched(true);
    if (!valid) return;
    setBusy(true);
    setProblem("");
    try {
      const facilityId = progress.facilityId;
      const created: string[] = [];
      const report = await runBatch(
        rows,
        (r) => r.username,
        async (r) => {
          const role = roles.find((x) => x.id === r.role);
          if (!role) throw new Error("role not found");
          const roleOrg = progress.roleOrganizations[role.name] ?? Object.entries(progress.roleOrganizations).find(([n]) => sameName(n, role.name))?.[1];
          let user = await findUser(r.username.trim());
          let outcome: "created" | "skipped" = "skipped";
          if (!user) {
            user = await createUser({
              username: r.username.trim(),
              first_name: r.first_name.trim(),
              last_name: r.last_name.trim(),
              email: r.email.trim(),
              phone_number: normalisePhone(r.phone),
              gender: r.gender as Gender,
              password,
              geo_organization: progress.districtId,
              role_orgs: roleOrg ? [{ organization: roleOrg, role: role.id }] : [],
            });
            outcome = "created";
          }
          for (const name of r.departments) {
            const dept = progress.departments.find((d) => d.name === name);
            if (!dept) continue;
            await addUserToDepartment(facilityId, dept.organizationId, user.id, role.id).catch((e) => {
              if (!/already/i.test(errorText(e))) throw e;
            });
          }
          if (r.facilityAdmin && progress.administrationId && facilityAdminRole) {
            await addUserToDepartment(facilityId, progress.administrationId, user.id, facilityAdminRole.id).catch((e) => {
              if (!/already/i.test(errorText(e))) throw e;
            });
          }
          created.push(user.username);
          return outcome;
        },
        setBatch,
        2,
      );
      update({ users: [...new Set([...progress.users, ...created])] });
      if (report.failed === 0) setFinished(true);
    } catch (e) {
      setProblem(errorText(e));
    } finally {
      setBusy(false);
    }
  };

  return (
    <Screen>
      <ScreenHead
        kicker="Step 5"
        title="Staff accounts"
        subtitle="Everyone gets the same starting password and should change it after their first sign-in."
      />
      <ScreenBody>
        <div className="flex max-w-[760px] flex-col gap-5">
          <Field
            label="Starting password for all users"
            htmlFor="pw"
            required
            error={touched ? pwProblem : undefined}
            hint="At least 8 characters, not only digits, and not too common."
          >
            <div className="flex gap-2">
              <Input
                id="pw"
                type={showPassword ? "text" : "password"}
                value={password}
                autoComplete="new-password"
                disabled={finished}
                onChange={(e) => setPassword(e.target.value)}
                className="max-w-[320px] font-mono"
              />
              <Button type="button" onClick={() => setShowPassword((v) => !v)}>
                {showPassword ? "Hide" : "Show"}
              </Button>
            </div>
          </Field>

          {rolesProblem ? <Alert variant="danger">{rolesProblem}</Alert> : null}

          {rows.map((r, i) => {
            const errors = touched ? allErrors[i] : [];
            return (
              <div key={r.key} className="rounded-xl border border-line bg-white p-4">
                <div className="mb-3 flex items-center justify-between">
                  <span className="text-[11.5px] font-bold tracking-[0.03em] text-faint uppercase">User {i + 1}</span>
                  {rows.length > 1 && !finished ? (
                    <button type="button" className="text-[12.5px] font-semibold text-danger-ink" onClick={() => setRows((l) => l.filter((x) => x.key !== r.key))}>
                      Remove
                    </button>
                  ) : null}
                </div>
                <div className="grid grid-cols-2 gap-x-4 gap-y-4">
                  <Field label="First name" required>
                    <Input value={r.first_name} disabled={finished} onChange={(e) => patch(r.key, { first_name: e.target.value })} />
                  </Field>
                  <Field label="Last name" required>
                    <Input value={r.last_name} disabled={finished} onChange={(e) => patch(r.key, { last_name: e.target.value })} />
                  </Field>
                  <Field label="Username" required>
                    <Input value={r.username} autoCapitalize="none" spellCheck={false} disabled={finished} onChange={(e) => patch(r.key, { username: e.target.value })} />
                  </Field>
                  <Field label="Role" required>
                    <Select value={r.role} disabled={finished} onValueChange={(v) => patch(r.key, { role: v })}>
                      <SelectTrigger>
                        <SelectValue placeholder="Select role" />
                      </SelectTrigger>
                      <SelectContent>
                        {roles.map((role) => (
                          <SelectItem key={role.id} value={role.id}>
                            {role.name}
                          </SelectItem>
                        ))}
                      </SelectContent>
                    </Select>
                  </Field>
                  <Field label="Email" required>
                    <Input type="email" value={r.email} disabled={finished} onChange={(e) => patch(r.key, { email: e.target.value })} />
                  </Field>
                  <Field label="Phone" required>
                    <Input inputMode="tel" placeholder="+91 90000 00000" value={r.phone} disabled={finished} onChange={(e) => patch(r.key, { phone: e.target.value })} />
                  </Field>
                  <Field label="Gender" required>
                    <Select value={r.gender} disabled={finished} onValueChange={(v) => patch(r.key, { gender: v as Gender })}>
                      <SelectTrigger>
                        <SelectValue placeholder="Select" />
                      </SelectTrigger>
                      <SelectContent>
                        {GENDERS.map((g) => (
                          <SelectItem key={g.value} value={g.value}>
                            {g.label}
                          </SelectItem>
                        ))}
                      </SelectContent>
                    </Select>
                  </Field>
                  <div className="flex flex-col justify-end gap-1 pb-1">
                    <label className="flex cursor-pointer items-center gap-2 text-[13px]">
                      <Checkbox checked={r.facilityAdmin} disabled={finished || !facilityAdminRole} onCheckedChange={(v) => patch(r.key, { facilityAdmin: v === true })} />
                      Also make this user a facility admin
                    </label>
                    <span className="pl-6 text-[12px] text-faint">Facility Admin in Administration only; the role above applies in the departments picked below.</span>
                  </div>
                </div>
                <div className="mt-4">
                  <Label className="mb-2 block">
                    Departments <span className="font-normal text-faint">(pick any number)</span>
                  </Label>
                  {progress.departments.length ? (
                    <div className="flex flex-wrap gap-2">
                      {progress.departments.map((d) => {
                        const on = r.departments.includes(d.name);
                        return (
                          <button
                            key={d.name}
                            type="button"
                            disabled={finished}
                            onClick={() => toggleDept(r.key, d.name)}
                            className={cn(
                              "rounded-full border px-3 py-1.5 text-[12.5px] font-medium",
                              on ? "border-brand bg-brand-bg text-brand-ink" : "border-line bg-white text-muted-foreground hover:border-brand",
                            )}
                          >
                            {on ? "✓ " : ""}
                            {d.name}
                          </button>
                        );
                      })}
                    </div>
                  ) : (
                    <div className="text-[12.5px] text-muted-foreground">No departments were created in the previous step.</div>
                  )}
                </div>
                {errors.length ? (
                  <div className="mt-3 text-[12.5px] text-danger-ink">Check: {errors.join(", ")}.</div>
                ) : null}
              </div>
            );
          })}

          {!finished ? (
            <Button type="button" className="justify-center" onClick={() => setRows((l) => [...l, blankRow()])}>
              <Plus className="size-4" /> Add another user
            </Button>
          ) : null}

          {batch ? <BatchPanel title="Staff accounts" progress={batch} running={busy} /> : null}
          {problem ? <Alert variant="danger">{problem}</Alert> : null}
          {finished ? <Alert>Done. Share the starting password with each person privately.</Alert> : null}
        </div>
      </ScreenBody>
      <StepFoot
        primary={finished ? "Continue" : "Create users"}
        primaryDisabled={!finished && (rows.length === 0 || roles.length === 0)}
        onPrimary={finished ? () => complete("users") : () => void run()}
        busy={busy}
        onSkip={finished ? undefined : () => skip("users")}
      />
    </Screen>
  );
}
