// Picker lists for the clinic details screen. The screen runs *before* the
// install, so there is no backend to ask — these are the values CARE ships with.
// `care options` prints the authoritative lists to check them against; a backend
// that adds a role or facility type won't offer it here until this list is
// updated. The seed is validated for real at the end of the install, and a
// mismatch is reported without failing the install.

export type Option = { value: string; label: string };

const asOptions = (list: readonly (string | readonly [string, string])[]): Option[] =>
  list.map((o) => (Array.isArray(o) ? { value: o[0], label: o[1] } : { value: o as string, label: o as string }));

export const FACILITY_TYPES = asOptions([
  "Private Hospital", "Clinical Non Governmental Organization", "Primary Health Centres",
  "Family Health Centres", "Community Health Centres", "Taluk Hospitals",
  "District Hospitals", "Govt Medical College Hospitals", "Co-operative hospitals",
  "Autonomous healthcare facility", "Women and Child Health Centres",
  "Non Clinical Non Governmental Organization", "Community Based Organization",
  "Educational Inst", "Private Labs", "Govt Labs", "TeleMedicine", "Other",
]);

export const STAFF_ROLES = asOptions([
  "Doctor", "Nurse", "Staff", "Administrator", "Facility Admin", "Pharmacist", "Volunteer",
]);

export const GENDERS = asOptions([
  ["female", "Female"], ["male", "Male"], ["non_binary", "Non-binary"], ["transgender", "Transgender"],
]);

export const DEFAULT_FACILITY_TYPE = "Private Hospital";
export const DEFAULT_ROLE = "Nurse";
