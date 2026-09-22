import { api, listAll } from "@/lib/api";

export type Facility = {
  id: string;
  name: string;
  facility_type: string;
  address: string;
  pincode: number;
  phone_number: string;
  is_public: boolean;
  invoice_number_expression?: string | null;
  geo_organization?: { id: string; name: string } | null;
};

export const FACILITY_TYPES = [
  "Primary Health Centres",
  "Family Health Centres",
  "Community Health Centres",
  "Taluk Hospitals",
  "Women and Child Health Centres",
  "District Hospitals",
  "Govt Medical College Hospitals",
  "Private Hospital",
  "Co-operative hospitals",
  "Autonomous healthcare facility",
  "Govt Labs",
  "Private Labs",
  "TeleMedicine",
  "Educational Inst",
  "Clinical Non Governmental Organization",
  "Other",
];

export type FacilityInput = {
  name: string;
  facility_type: string;
  phone_number: string;
  pincode: number;
  address: string;
  description: string;
  geo_organization: string;
};

export function listFacilities(): Promise<Facility[]> {
  return listAll<Facility>("/facility/");
}

export function createFacility(input: FacilityInput): Promise<Facility> {
  return api.post<Facility>("/facility/", {
    ...input,
    is_public: true,
    features: [],
    latitude: null,
    longitude: null,
    middleware_address: null,
  });
}

export function setInvoiceExpression(facilityId: string, expression: string): Promise<unknown> {
  return api.post(`/facility/${facilityId}/set_invoice_expression/`, {
    invoice_number_expression: expression,
  });
}
