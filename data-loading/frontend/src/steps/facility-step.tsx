import { useState } from "react";

import { createFacility, FACILITY_TYPES } from "@/care/facility";
import { Field } from "@/components/field";
import { Screen, ScreenBody, ScreenHead } from "@/components/screen";
import { StepFoot } from "@/components/step-foot";
import { Alert } from "@/components/ui/alert";
import { Input } from "@/components/ui/input";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { errorText, initialsOf, normalisePhone } from "@/lib/format";
import { useWizard } from "@/state/wizard";

type Form = {
  name: string;
  facility_type: string;
  phone: string;
  pincode: string;
  address: string;
  description: string;
};

const EMPTY: Form = { name: "", facility_type: "", phone: "", pincode: "", address: "", description: "" };

function validate(f: Form): Partial<Record<keyof Form, string>> {
  const errors: Partial<Record<keyof Form, string>> = {};
  if (!f.name.trim()) errors.name = "Give the facility a name.";
  if (!f.facility_type) errors.facility_type = "Pick a facility type.";
  if (!/^\+?\d{10,14}$/.test(normalisePhone(f.phone))) errors.phone = "Enter a 10-digit phone number.";
  if (!/^\d{6}$/.test(f.pincode.trim())) errors.pincode = "Enter the 6-digit PIN code.";
  if (!f.address.trim()) errors.address = "Enter the address.";
  return errors;
}

export function FacilityStep() {
  const { progress, complete, goTo } = useWizard();
  const [form, setForm] = useState<Form>(EMPTY);
  const [touched, setTouched] = useState(false);
  const [busy, setBusy] = useState(false);
  const [problem, setProblem] = useState("");

  const errors = validate(form);
  const set = (patch: Partial<Form>) => setForm((f) => ({ ...f, ...patch }));
  const show = (k: keyof Form) => (touched ? errors[k] : undefined);

  const create = async () => {
    setTouched(true);
    if (Object.keys(errors).length) return;
    setBusy(true);
    setProblem("");
    try {
      const facility = await createFacility({
        name: form.name.trim(),
        facility_type: form.facility_type,
        phone_number: normalisePhone(form.phone),
        pincode: Number(form.pincode.trim()),
        address: form.address.trim(),
        description: form.description.trim(),
        geo_organization: progress.districtId,
      });
      complete("facility", {
        facilityId: facility.id,
        facilityName: facility.name,
        initials: initialsOf(facility.name),
      });
    } catch (e) {
      setProblem(errorText(e));
    } finally {
      setBusy(false);
    }
  };

  return (
    <Screen>
      <ScreenHead
        kicker="Step 3"
        title="Your facility"
        subtitle={`Created under ${progress.districtName}, ${progress.stateName}. It is listed publicly so staff and patients can find it.`}
      />
      <ScreenBody>
        <div className="grid max-w-[720px] grid-cols-2 gap-x-4 gap-y-5">
          <Field label="Facility type" htmlFor="ftype" required error={show("facility_type")}>
            <Select value={form.facility_type} onValueChange={(v) => set({ facility_type: v })}>
              <SelectTrigger id="ftype">
                <SelectValue placeholder="Select facility type" />
              </SelectTrigger>
              <SelectContent>
                {FACILITY_TYPES.map((t) => (
                  <SelectItem key={t} value={t}>
                    {t}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </Field>
          <Field label="Facility name" htmlFor="fname" required error={show("name")}>
            <Input id="fname" value={form.name} placeholder="e.g. PHC Aluva" onChange={(e) => set({ name: e.target.value })} />
          </Field>
          <Field label="Phone number" htmlFor="fphone" required error={show("phone")}>
            <Input id="fphone" value={form.phone} placeholder="+91 90000 00000" inputMode="tel" onChange={(e) => set({ phone: e.target.value })} />
          </Field>
          <Field label="PIN code" htmlFor="fpin" required error={show("pincode")}>
            <Input id="fpin" value={form.pincode} placeholder="683101" inputMode="numeric" onChange={(e) => set({ pincode: e.target.value })} />
          </Field>
          <Field label="Address" htmlFor="faddress" required error={show("address")} className="col-span-2">
            <textarea
              id="faddress"
              rows={2}
              value={form.address}
              placeholder="Municipal Building Road, Aluva, Ernakulam"
              onChange={(e) => set({ address: e.target.value })}
              className="w-full rounded-md border border-line bg-white px-3 py-2.5 text-sm text-ink outline-none placeholder:text-faint focus-visible:border-brand"
            />
          </Field>
          <Field label="Description" htmlFor="fdesc" className="col-span-2">
            <Input id="fdesc" value={form.description} placeholder="Optional" onChange={(e) => set({ description: e.target.value })} />
          </Field>
          {problem ? (
            <div className="col-span-2">
              <Alert variant="danger">{problem}</Alert>
            </div>
          ) : null}
        </div>
      </ScreenBody>
      <StepFoot
        primary="Create facility"
        onPrimary={() => void create()}
        busy={busy}
        onBack={() => goTo("district")}
        note="The facility is created in CARE as soon as you press the button."
      />
    </Screen>
  );
}
