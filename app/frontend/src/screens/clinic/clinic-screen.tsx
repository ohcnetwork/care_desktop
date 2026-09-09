import { Download } from "lucide-react";
import { useState, type ReactNode } from "react";

import { FootNote, Screen, ScreenBody, ScreenFoot, ScreenHead } from "@/components/screen";
import { SectionTitle, StepDot } from "@/components/section-header";
import {
  Accordion,
  AccordionContent,
  AccordionItem,
  AccordionTrigger,
} from "@/components/ui/accordion";
import { Alert } from "@/components/ui/alert";
import { Button } from "@/components/ui/button";
import { Combobox } from "@/components/ui/combobox";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { cn } from "@/lib/utils";
import { FACILITY_TYPES } from "@/options";
import { useCare, type InstallParams } from "@/state/care-store";
import {
  newStaffMember,
  type ClinicForm,
  type StaffMemberForm,
} from "@/state/forms";
import type { ClinicSeed, SeedMember } from "@/types";
import { StaffMemberCard } from "./staff-member-card";

export const EMPTY_SEED: ClinicSeed = {
  geo_organization: "",
  facility: {
    name: "",
    facility_type: "",
    address: "",
    pincode: "",
    phone_number: "",
    description: "",
  },
  members: [],
};

const STAFF_INFO = (
  <>
    Everyone here starts with the same password and signs in with their own username. The{" "}
    <b>admin</b> login you just set is separate and always works.
  </>
);

const REQUIRED_MEMBER_FIELDS = [
  "first_name",
  "last_name",
  "username",
  "email",
  "phone_number",
] as const;

function FormField({
  label,
  htmlFor,
  className,
  children,
}: {
  label: string;
  htmlFor?: string;
  className?: string;
  children: ReactNode;
}) {
  return (
    <div className={cn("flex min-w-0 flex-col gap-[7px]", className)}>
      <Label htmlFor={htmlFor}>{label}</Label>
      {children}
    </div>
  );
}

export function ClinicScreen({
  form,
  patch,
  installParams,
}: {
  form: ClinicForm;
  patch: (values: Partial<ClinicForm>) => void;
  installParams: () => InstallParams;
}) {
  const { backToSetupForm, startInstall } = useCare();
  const [note, setNote] = useState("");
  const [bad, setBad] = useState<ReadonlySet<string>>(new Set());

  const setMember = (id: number, values: Partial<StaffMemberForm>) =>
    patch({
      members: form.members.map((m) => (m.id === id ? { ...m, ...values } : m)),
    });

  const addMember = () =>
    patch({
      members: [...form.members, newStaffMember(form.nextMemberId)],
      nextMemberId: form.nextMemberId + 1,
    });

  const removeMember = (id: number) =>
    patch({ members: form.members.filter((m) => m.id !== id) });

  /**
   * Returns the form as a seed, or null after posting the reason. Only shape is
   * checked here — CARE owns the real rules (unique usernames, valid pincodes)
   * and reports them at the end of the install.
   */
  const collectSeed = (): ClinicSeed | null => {
    const fail = (message: string, field?: string, focusId?: string) => {
      setNote(message);
      setBad(field ? new Set([field]) : new Set());
      if (focusId) document.getElementById(focusId)?.focus();
      return null;
    };

    if (!form.name.trim()) {
      return fail("Give the facility a name, or choose Skip.", "name", "cf-name");
    }
    if (!form.region.trim()) {
      return fail("Give the region the facility is in.", "region", "cf-region");
    }
    if (form.members.length && form.staffPassword.trim().length < 8) {
      return fail(
        "Staff need a starting password of at least 8 characters.",
        "staffPassword",
        "cf-staffpw",
      );
    }

    const members: SeedMember[] = [];
    for (const [i, member] of form.members.entries()) {
      for (const field of REQUIRED_MEMBER_FIELDS) {
        if (!member[field].trim()) {
          return fail(`Staff member ${i + 1} needs every field filled in.`, `${member.id}.${field}`);
        }
      }
      members.push({
        username: member.username.trim(),
        first_name: member.first_name.trim(),
        last_name: member.last_name.trim(),
        email: member.email.trim(),
        phone_number: member.phone_number.trim(),
        gender: member.gender,
        role: member.role,
        password: form.staffPassword.trim(),
      });
    }

    setNote("");
    setBad(new Set());
    return {
      geo_organization: form.region.trim(),
      facility: {
        name: form.name.trim(),
        facility_type: form.facilityType,
        address: form.address.trim(),
        pincode: form.pincode.trim(),
        phone_number: form.phone.trim(),
        description: "",
      },
      members,
    };
  };

  return (
    <Screen>
      <ScreenHead
        kicker="Step 2 of 2"
        title="Your clinic's details"
        subtitle="Added to CARE once the install finishes. You can skip this and add it later."
      />

      <ScreenBody>
        <Accordion type="multiple" defaultValue={["facility", "staff"]}>
          <AccordionItem value="facility">
            <AccordionTrigger>
              <StepDot done />
              <SectionTitle title="Facility" summary="The clinic this computer serves" />
            </AccordionTrigger>
            <AccordionContent>
              <div className="grid grid-cols-2 gap-3.5">
                <FormField label="Facility name" htmlFor="cf-name" className="col-span-full">
                  <Input
                    id="cf-name"
                    placeholder="Town Clinic"
                    autoComplete="off"
                    aria-invalid={bad.has("name")}
                    value={form.name}
                    onChange={(e) => patch({ name: e.target.value })}
                  />
                </FormField>
                <FormField label="Facility type" htmlFor="cf-type">
                  <Combobox
                    id="cf-type"
                    aria-label="Facility type"
                    options={FACILITY_TYPES}
                    value={form.facilityType}
                    onChange={(facilityType) => patch({ facilityType })}
                  />
                </FormField>
                <FormField label="Region" htmlFor="cf-region">
                  <Input
                    id="cf-region"
                    placeholder="District or state"
                    autoComplete="off"
                    aria-invalid={bad.has("region")}
                    value={form.region}
                    onChange={(e) => patch({ region: e.target.value })}
                  />
                </FormField>
                <FormField label="Address" htmlFor="cf-address" className="col-span-full">
                  <Input
                    id="cf-address"
                    placeholder="Street, area"
                    autoComplete="off"
                    value={form.address}
                    onChange={(e) => patch({ address: e.target.value })}
                  />
                </FormField>
                <FormField label="Pincode" htmlFor="cf-pincode">
                  <Input
                    id="cf-pincode"
                    placeholder="682030"
                    inputMode="numeric"
                    autoComplete="off"
                    value={form.pincode}
                    onChange={(e) => patch({ pincode: e.target.value })}
                  />
                </FormField>
                <FormField label="Phone number" htmlFor="cf-phone">
                  <Input
                    id="cf-phone"
                    placeholder="+919999999999"
                    autoComplete="off"
                    value={form.phone}
                    onChange={(e) => patch({ phone: e.target.value })}
                  />
                </FormField>
              </div>
            </AccordionContent>
          </AccordionItem>

          <AccordionItem value="staff">
            <AccordionTrigger>
              <StepDot done />
              <SectionTitle
                title="Staff"
                summary={
                  form.members.length
                    ? `${form.members.length} ${form.members.length === 1 ? "person" : "people"}`
                    : "Nobody added yet"
                }
              />
            </AccordionTrigger>
            <AccordionContent>
              <Alert>{STAFF_INFO}</Alert>

              <FormField
                label="Starting password for staff"
                htmlFor="cf-staffpw"
                className="mb-3.5 max-w-1/2"
              >
                <Input
                  id="cf-staffpw"
                  type="text"
                  placeholder="At least 8 characters"
                  autoComplete="off"
                  aria-invalid={bad.has("staffPassword")}
                  value={form.staffPassword}
                  onChange={(e) => patch({ staffPassword: e.target.value })}
                />
              </FormField>

              <div>
                {form.members.map((member, i) => (
                  <StaffMemberCard
                    key={member.id}
                    index={i}
                    member={member}
                    bad={bad}
                    onChange={(values) => setMember(member.id, values)}
                    onRemove={() => removeMember(member.id)}
                  />
                ))}
              </div>

              <Button className="self-start" onClick={addMember}>
                + Add a staff member
              </Button>
            </AccordionContent>
          </AccordionItem>
        </Accordion>
      </ScreenBody>

      <ScreenFoot>
        <Button size="lg" onClick={backToSetupForm}>
          Back
        </Button>
        <Button
          variant="primary"
          size="lg"
          className="shadow-lift"
          onClick={() => {
            const seed = collectSeed();
            if (seed) startInstall(installParams(), seed);
          }}
        >
          <Download className="size-[17px]" strokeWidth={2.2} />
          Install and start
        </Button>
        <Button size="lg" onClick={() => startInstall(installParams(), EMPTY_SEED)}>
          Skip and install
        </Button>
        <FootNote>{note}</FootNote>
      </ScreenFoot>
    </Screen>
  );
}
