import { env } from "./env";

// Pro community perk: invite buyers to the private Pro repo, remove them on
// refund. No-ops (with a log) when GITHUB_TOKEN / GITHUB_PRO_REPO are unset,
// mirroring the email dev fallback. Callers treat failures as non-fatal —
// the license itself is the entitlement, the invite is a perk.

const USERNAME_RE = /^[a-zA-Z0-9](?:[a-zA-Z0-9-]{0,37}[a-zA-Z0-9])?$/;

export function isValidGithubUsername(username: string): boolean {
  return USERNAME_RE.test(username);
}

function configured(): boolean {
  return Boolean(env.githubToken && env.githubProRepo);
}

function headers(): Record<string, string> {
  return {
    Authorization: `Bearer ${env.githubToken}`,
    Accept: "application/vnd.github+json",
    "X-GitHub-Api-Version": "2022-11-28",
    "User-Agent": "deployshuttle-license-server",
  };
}

export async function inviteToProRepo(username: string): Promise<void> {
  if (!isValidGithubUsername(username)) {
    console.warn(`[github] ignoring invalid username ${JSON.stringify(username)}`);
    return;
  }
  if (!configured()) {
    console.log(`[github:dev] would invite ${username} to the Pro repo`);
    return;
  }
  const res = await fetch(
    `${env.githubApiUrl}/repos/${env.githubProRepo}/collaborators/${username}`,
    { method: "PUT", headers: headers(), body: JSON.stringify({ permission: "pull" }) },
  );
  // 201 = invitation sent, 204 = already a collaborator.
  if (!res.ok) {
    const detail = await res.text().catch(() => "");
    throw new Error(`GitHub invite failed (${res.status}): ${detail}`);
  }
}

export async function removeFromProRepo(username: string): Promise<void> {
  if (!isValidGithubUsername(username)) return;
  if (!configured()) {
    console.log(`[github:dev] would remove ${username} from the Pro repo`);
    return;
  }
  const res = await fetch(
    `${env.githubApiUrl}/repos/${env.githubProRepo}/collaborators/${username}`,
    { method: "DELETE", headers: headers() },
  );
  // 204 = removed; removing a non-collaborator also returns 204.
  if (!res.ok) {
    const detail = await res.text().catch(() => "");
    throw new Error(`GitHub remove failed (${res.status}): ${detail}`);
  }
}
