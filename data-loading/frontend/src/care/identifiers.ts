import { api, listAll } from "@/lib/api";

export type IdentifierConfig = {
  id: string;
  status: "active" | "inactive";
  config: {
    use: string;
    system: string;
    display: string;
    default_value?: string | null;
    required: boolean;
    unique: boolean;
    regex: string;
    description?: string;
    auto_maintained?: boolean;
  };
};

export const PATIENT_ID_SYSTEM = "care.local/patient-id";

export function listIdentifierConfigs(): Promise<IdentifierConfig[]> {
  return listAll<IdentifierConfig>("/patient_identifier_config/");
}

export function createIdentifierConfig(input: { display: string; default_value: string }): Promise<IdentifierConfig> {
  return api.post<IdentifierConfig>("/patient_identifier_config/", {
    facility: null,
    status: "active",
    config: {
      use: "official",
      system: PATIENT_ID_SYSTEM,
      display: input.display,
      description: "Generated when a patient is registered",
      required: true,
      unique: true,
      regex: "",
      retrieve_config: {
        retrieve_with_dob: false,
        retrieve_with_year_of_birth: false,
        retrieve_with_otp: false,
        retrieve_partial_search: true,
      },
      default_value: input.default_value,
      auto_maintained: false,
    },
  });
}
