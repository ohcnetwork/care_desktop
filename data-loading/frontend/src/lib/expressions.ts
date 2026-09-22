export function invoiceExpression(initials: string): string {
  return `f'${initials}-INV-{invoice_count + 1}'`;
}

export function patientIdExpression(initials: string): string {
  return `f'${initials}-{patient_count + 1:04d}'`;
}

export function previewInvoice(initials: string, count = 0): string {
  return `${initials}-INV-${count + 1}`;
}

export function previewPatientId(initials: string, count = 0): string {
  return `${initials}-${String(count + 1).padStart(4, "0")}`;
}

export function cleanInitials(raw: string): string {
  return raw.toUpperCase().replace(/[^A-Z0-9]/g, "").slice(0, 6);
}
