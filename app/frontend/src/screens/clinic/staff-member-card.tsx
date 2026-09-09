import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { GENDERS, STAFF_ROLES, type Option } from "@/options";
import type { StaffMemberForm } from "@/state/forms";

const TEXT_FIELDS: { key: keyof StaffMemberForm; label: string; placeholder?: string }[] = [
  { key: "first_name", label: "First name" },
  { key: "last_name", label: "Last name" },
  { key: "username", label: "Username", placeholder: "asha" },
  { key: "email", label: "Email", placeholder: "asha@clinic.local" },
  { key: "phone_number", label: "Phone number", placeholder: "+919999999999" },
];

function PickerField({
  id,
  label,
  options,
  value,
  onChange,
}: {
  id: string;
  label: string;
  options: Option[];
  value: string;
  onChange: (value: string) => void;
}) {
  return (
    <div className="flex min-w-0 flex-col gap-[7px]">
      <Label htmlFor={id}>{label}</Label>
      <Select value={value} onValueChange={onChange}>
        <SelectTrigger id={id} aria-label={label}>
          <SelectValue placeholder="Select…" />
        </SelectTrigger>
        <SelectContent>
          {options.map((o) => (
            <SelectItem key={o.value} value={o.value}>
              {o.label}
            </SelectItem>
          ))}
        </SelectContent>
      </Select>
    </div>
  );
}

export function StaffMemberCard({
  index,
  member,
  bad,
  onChange,
  onRemove,
}: {
  index: number;
  member: StaffMemberForm;
  bad: ReadonlySet<string>;
  onChange: (values: Partial<StaffMemberForm>) => void;
  onRemove: () => void;
}) {
  return (
    <div className="mb-3 rounded-lg border border-line p-3.5">
      <div className="mb-3 flex items-center justify-between text-[13px] font-semibold text-ink2">
        <span>Staff member {index + 1}</span>
        <button
          type="button"
          onClick={onRemove}
          className="cursor-pointer px-1 py-0.5 text-[12.5px] font-semibold text-muted-foreground hover:text-danger-ink"
        >
          Remove
        </button>
      </div>
      <div className="grid grid-cols-2 gap-3.5">
        {TEXT_FIELDS.map((field) => {
          const id = `member-${member.id}-${field.key}`;
          return (
            <div key={field.key} className="flex min-w-0 flex-col gap-[7px]">
              <Label htmlFor={id}>{field.label}</Label>
              <Input
                id={id}
                autoComplete="off"
                placeholder={field.placeholder}
                aria-invalid={bad.has(`${member.id}.${field.key}`)}
                value={member[field.key] as string}
                onChange={(e) => onChange({ [field.key]: e.target.value })}
              />
            </div>
          );
        })}
        <PickerField
          id={`member-${member.id}-gender`}
          label="Gender"
          options={GENDERS}
          value={member.gender}
          onChange={(gender) => onChange({ gender })}
        />
        <PickerField
          id={`member-${member.id}-role`}
          label="Role"
          options={STAFF_ROLES}
          value={member.role}
          onChange={(role) => onChange({ role })}
        />
      </div>
    </div>
  );
}
