# Technical Architecture and Security

## 1. Technical Architecture

### CLI runtime

Recommended:

- Node.js/Bun for fastest iteration if current project already uses TS;
- Go later if single binary distribution becomes critical.

### Modules

```txt
src/
  cli/
    commands/
      doctor.ts
      report.ts
      harden.ts
      init.ts
      deploy.ts
  core/
    checks/
      system/
      ssh/
      firewall/
      docker/
      compose/
      caddy/
      cloudflare/
      database/
      backups/
      secrets/
      monitoring/
    scoring/
    report/
    remediation/
  adapters/
    local-shell.ts
    ssh.ts
    docker.ts
    cloudflare.ts
  config/
    schema.ts
```

## 2. Check Interface

```ts
export type CheckSeverity = 'critical' | 'high' | 'medium' | 'low' | 'info';

export type CheckStatus = 'passed' | 'failed' | 'skipped' | 'unknown';

export type CheckResult = {
  id: string;
  title: string;
  category: string;
  severity: CheckSeverity;
  status: CheckStatus;
  summary: string;
  details?: string;
  remediation?: string;
  autoFixAvailable: boolean;
  evidence?: Record<string, unknown>;
};

export type CheckContext = {
  target: Target;
  profile: string[];
  exec: ExecAdapter;
  config: DeployShuttleConfig;
};

export type Check = {
  id: string;
  run: (context: CheckContext) => Promise<CheckResult>;
};
```

## 3. Exec Adapter

```ts
export type ExecResult = {
  exitCode: number;
  stdout: string;
  stderr: string;
};

export type ExecAdapter = {
  run: (command: string, options?: { timeoutMs?: number }) => Promise<ExecResult>;
};
```

## 4. Security Requirements

- Never print secrets.
- Redact env values by default.
- Do not upload reports to cloud without explicit login/sync.
- SSH commands must be visible in verbose mode.
- `harden` must support `--dry-run`.
- Dangerous fixes require confirmation.
- Report must not include private key material.
- Cloudflare tokens must be stored locally in OS keychain if possible.
- Any future cloud sync must encrypt sensitive metadata.
