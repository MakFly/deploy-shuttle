import { env } from "./env";

// Minimal Resend client. No-ops when RESEND_API_KEY is missing (dev/test).
export async function sendLicenseKeyEmail(to: string, key: string): Promise<void> {
  if (!env.resendApiKey) {
    console.log(`[email:dev] would send license key ${key} to ${to}`);
    return;
  }
  const res = await fetch("https://api.resend.com/emails", {
    method: "POST",
    headers: {
      Authorization: `Bearer ${env.resendApiKey}`,
      "Content-Type": "application/json",
    },
    body: JSON.stringify({
      from: env.resendFrom,
      to,
      subject: "Your DeployShuttle Pro license key",
      html: licenseEmailHtml(key),
      text: licenseEmailText(key),
    }),
  });
  if (!res.ok) {
    const detail = await res.text().catch(() => "");
    throw new Error(`Resend send failed (${res.status}): ${detail}`);
  }
}

function licenseEmailHtml(key: string): string {
  return `
    <p>Welcome to DeployShuttle Pro.</p>
    <p>Your license key is:</p>
    <p style="font-family: monospace; font-size: 16px; padding: 12px; background: #f4f4f5; border-radius: 6px; display: inline-block;"><strong>${key}</strong></p>
    <p>Activate it on a machine:</p>
    <pre style="background: #111; color: #eee; padding: 12px; border-radius: 6px;">shuttle license activate ${key}</pre>
    <p>The CLI verifies offline for 14 days at a time and refreshes silently when you're online.</p>
    <p>Thanks for supporting the project.</p>
  `;
}

function licenseEmailText(key: string): string {
  return `Welcome to DeployShuttle Pro.

Your license key is:
  ${key}

Activate it on a machine:
  shuttle license activate ${key}

The CLI verifies offline for 14 days at a time and refreshes silently when you're online.
`;
}
