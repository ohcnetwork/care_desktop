// The two long-lived forms. They live above the screens so that stepping
// forward to the clinic details and back — or bouncing off a failed install —
// never loses what the operator already typed.
import { DEFAULT_FACILITY_TYPE, DEFAULT_ROLE, GENDERS } from "@/options";

export type SetupForm = {
  /** Without the ".local" suffix, which the field shows as a fixed adornment. */
  hostInput: string;
  adminPassword: string;
  adminConfirm: string;
  backupPassword: string;
  backupConfirm: string;
  backupDir: string;
};

export const EMPTY_SETUP_FORM: SetupForm = {
  hostInput: "care",
  adminPassword: "",
  adminConfirm: "",
  backupPassword: "",
  backupConfirm: "",
  backupDir: "",
};

export type StaffMemberForm = {
  id: number;
  first_name: string;
  last_name: string;
  username: string;
  email: string;
  phone_number: string;
  gender: string;
  role: string;
};

export type ClinicForm = {
  name: string;
  facilityType: string;
  region: string;
  address: string;
  pincode: string;
  phone: string;
  staffPassword: string;
  members: StaffMemberForm[];
  nextMemberId: number;
};

export const EMPTY_CLINIC_FORM: ClinicForm = {
  name: "",
  facilityType: DEFAULT_FACILITY_TYPE,
  region: "",
  address: "",
  pincode: "",
  phone: "",
  staffPassword: "",
  members: [],
  nextMemberId: 0,
};

export function newStaffMember(id: number): StaffMemberForm {
  return {
    id,
    first_name: "",
    last_name: "",
    username: "",
    email: "",
    phone_number: "",
    // A native <select> always had one option selected; starting empty here
    // would submit a blank gender.
    gender: GENDERS[0].value,
    role: DEFAULT_ROLE,
  };
}
