// Generate a stable, human-readable license key.
// Format: DS-XXXXXX-XXXXXX-XXXXXX (Crockford base32, 18 chars without dashes).

const ALPHABET = "ABCDEFGHJKMNPQRSTVWXYZ0123456789"; // Crockford-ish, no I/L/O/U

export function generateLicenseKey(): string {
  const buf = crypto.getRandomValues(new Uint8Array(18));
  let out = "";
  for (let i = 0; i < buf.length; i++) {
    const idx = (buf[i] ?? 0) % ALPHABET.length;
    out += ALPHABET[idx];
  }
  return `DS-${out.slice(0, 6)}-${out.slice(6, 12)}-${out.slice(12, 18)}`;
}
