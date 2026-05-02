package readiness

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/MakFly/deploy-shuttle/go-cli/internal/execx"
)

// composeSearchCommand emits `__compose__=<path>` followed by the file body
// for the first readable compose file found in standard locations.
const composeSearchCommand = `for f in docker-compose.prod.yml compose.prod.yml docker-compose.yml compose.yml /opt/shuttle/*/docker-compose.prod.yml /opt/shuttle/*/docker-compose.yml; do if [ -r "$f" ]; then echo __compose__="$f"; cat "$f"; exit 0; fi; done; exit 1`

// composeSnapshot is the result of a single compose lookup.
type composeSnapshot struct {
	Found bool
	Path  string
	Body  string
}

func loadCompose(adapter execx.Adapter) composeSnapshot {
	res := adapter.Run(composeSearchCommand, 3*time.Second)
	if res.ExitCode != 0 {
		return composeSnapshot{}
	}
	path := capture(res.Stdout, `__compose__=([^\n]+)`)
	body := strings.TrimPrefix(res.Stdout, "__compose__="+path+"\n")
	return composeSnapshot{Found: true, Path: strings.TrimSpace(path), Body: body}
}

// checkComposeMissingProdFile fails when no compose file is found at all.
func checkComposeMissingProdFile(snap composeSnapshot) CheckResult {
	if snap.Found {
		return CheckResult{
			ID: "compose.missing_prod_file", Title: "Production compose file is present",
			Category: "compose", Severity: Medium, Status: Passed,
			Summary:  "Detected compose file: " + snap.Path + ".",
			Evidence: map[string]any{"composeFile": snap.Path},
		}
	}
	return CheckResult{
		ID: "compose.missing_prod_file", Title: "Production compose file is present",
		Category: "compose", Severity: Medium, Status: Failed,
		Summary:     "No docker-compose.prod.yml or docker-compose.yml found in standard locations.",
		Remediation: "Add a docker-compose.prod.yml at the project root (or under /opt/shuttle/<app>/) so doctor can audit production services.",
	}
}

// composeEnvFileRegex captures `env_file: <path>` and `env_file: [<paths>]` declarations.
var composeEnvFileRegex = regexp.MustCompile(`(?m)^\s*env_file:\s*(?:\[([^\]]+)\]|([^\n#]+))`)

// checkComposeEnvFileMissing fails when an env_file reference points to a file
// that does not exist on the target.
func checkComposeEnvFileMissing(adapter execx.Adapter, snap composeSnapshot) CheckResult {
	if !snap.Found {
		return CheckResult{
			ID: "compose.env_file_missing", Title: "compose env_file references resolve",
			Category: "compose", Severity: Medium, Status: Skipped,
			Summary: "No compose file detected; cannot verify env_file references.",
		}
	}
	references := []string{}
	for _, match := range composeEnvFileRegex.FindAllStringSubmatch(snap.Body, -1) {
		raw := strings.TrimSpace(match[1] + match[2])
		raw = strings.Trim(raw, `"' `)
		for _, candidate := range strings.Split(raw, ",") {
			candidate = strings.Trim(strings.TrimSpace(candidate), `"' `)
			if candidate == "" || strings.HasPrefix(candidate, "#") {
				continue
			}
			references = append(references, candidate)
		}
	}
	if len(references) == 0 {
		return CheckResult{
			ID: "compose.env_file_missing", Title: "compose env_file references resolve",
			Category: "compose", Severity: Medium, Status: Passed,
			Summary:  "No env_file references in " + snap.Path + ".",
			Evidence: map[string]any{"composeFile": snap.Path},
		}
	}
	missing := []string{}
	composeDir := dirOf(snap.Path)
	for _, ref := range references {
		probe := ref
		if !strings.HasPrefix(probe, "/") {
			probe = composeDir + "/" + ref
		}
		if adapter.Run("test -f "+shellQuote(probe), time.Second).ExitCode != 0 {
			missing = append(missing, ref)
		}
	}
	status := Passed
	summary := "All env_file references resolve."
	if len(missing) > 0 {
		status = Failed
		summary = fmt.Sprintf("%d env_file reference(s) point to missing files: %s.", len(missing), strings.Join(missing, ", "))
	}
	return CheckResult{
		ID: "compose.env_file_missing", Title: "compose env_file references resolve",
		Category: "compose", Severity: Medium, Status: status,
		Summary:     summary,
		Remediation: "Create the referenced env file(s) or update the compose file to point at the correct path.",
		Evidence:    map[string]any{"composeFile": snap.Path, "references": references, "missing": missing},
	}
}

// composeImageRegex captures `image: foo:tag` lines.
var composeImageRegex = regexp.MustCompile(`(?m)^\s*image:\s*([^\s#]+)`)

// checkComposeLatestTag flags images using ':latest' or no explicit tag.
func checkComposeLatestTag(snap composeSnapshot) CheckResult {
	if !snap.Found {
		return CheckResult{
			ID: "compose.latest_tag_used", Title: "Production compose pins image tags",
			Category: "compose", Severity: Medium, Status: Skipped,
			Summary: "No compose file detected.",
		}
	}
	offenders := []string{}
	for _, match := range composeImageRegex.FindAllStringSubmatch(snap.Body, -1) {
		image := strings.Trim(match[1], `"'`)
		// Strip any registry prefix so the colon in 'registry:5000/foo' is not
		// mistaken for a tag separator.
		nameAndTag := image
		if slash := strings.LastIndex(image, "/"); slash >= 0 {
			nameAndTag = image[slash+1:]
		}
		tag := ""
		if idx := strings.LastIndex(nameAndTag, ":"); idx > 0 {
			tag = nameAndTag[idx+1:]
		}
		if tag == "" || tag == "latest" {
			offenders = append(offenders, image)
		}
	}
	if len(offenders) == 0 {
		return CheckResult{
			ID: "compose.latest_tag_used", Title: "Production compose pins image tags",
			Category: "compose", Severity: Medium, Status: Passed,
			Summary:  "All image references in " + snap.Path + " are pinned.",
			Evidence: map[string]any{"composeFile": snap.Path},
		}
	}
	return CheckResult{
		ID: "compose.latest_tag_used", Title: "Production compose pins image tags",
		Category: "compose", Severity: Medium, Status: Failed,
		Summary:     fmt.Sprintf("%d compose image(s) use ':latest' or no tag: %s.", len(offenders), strings.Join(offenders, ", ")),
		Remediation: "Pin every production image to an explicit immutable tag or digest so deploys are reproducible.",
		Evidence:    map[string]any{"composeFile": snap.Path, "offenders": offenders},
	}
}

// composeServiceRegex matches a service block start (top-level service key).
var composeServiceRegex = regexp.MustCompile(`(?m)^\s{2}([a-zA-Z0-9._-]+):\s*$`)

// checkComposeNoResourceLimits warns when no service declares cpu/memory limits.
func checkComposeNoResourceLimits(snap composeSnapshot) CheckResult {
	if !snap.Found {
		return CheckResult{
			ID: "compose.no_resource_limits", Title: "compose services declare resource limits",
			Category: "compose", Severity: Low, Status: Skipped,
			Summary: "No compose file detected.",
		}
	}
	services := composeServiceRegex.FindAllStringSubmatch(snap.Body, -1)
	hasLimits := regexp.MustCompile(`(?m)mem_limit:|cpus:|cpu_count:|^\s+limits:`).MatchString(snap.Body)
	if hasLimits {
		return CheckResult{
			ID: "compose.no_resource_limits", Title: "compose services declare resource limits",
			Category: "compose", Severity: Low, Status: Passed,
			Summary:  "compose declares cpu/memory limits.",
			Evidence: map[string]any{"composeFile": snap.Path, "serviceCount": len(services)},
		}
	}
	return CheckResult{
		ID: "compose.no_resource_limits", Title: "compose services declare resource limits",
		Category: "compose", Severity: Low, Status: Failed,
		Summary:     fmt.Sprintf("No mem_limit / cpus / deploy.resources.limits found across %d compose service(s).", len(services)),
		Remediation: "Add 'deploy.resources.limits' (Swarm) or 'mem_limit' / 'cpus' (classic) so a runaway container cannot starve the host.",
		Evidence:    map[string]any{"composeFile": snap.Path, "serviceCount": len(services)},
	}
}

// sensitiveBindMounts lists host paths that should never be bind-mounted into containers.
var sensitiveBindMounts = []string{
	"/var/run/docker.sock",
	"/etc:",
	"/etc/ssh",
	"/etc/shadow",
	"/root",
	"/proc:",
	"/sys:",
	"/:",
}

// checkComposeBindMountSensitivePaths flags risky host bind mounts.
func checkComposeBindMountSensitivePaths(snap composeSnapshot) CheckResult {
	if !snap.Found {
		return CheckResult{
			ID: "compose.bind_mount_sensitive_paths", Title: "compose does not bind-mount sensitive host paths",
			Category: "compose", Severity: High, Status: Skipped,
			Summary: "No compose file detected.",
		}
	}
	hits := []string{}
	for _, line := range nonEmptyLines(snap.Body) {
		for _, candidate := range sensitiveBindMounts {
			if strings.Contains(line, candidate) {
				hits = append(hits, strings.TrimSpace(line))
				break
			}
		}
	}
	if len(hits) == 0 {
		return CheckResult{
			ID: "compose.bind_mount_sensitive_paths", Title: "compose does not bind-mount sensitive host paths",
			Category: "compose", Severity: High, Status: Passed,
			Summary:  "No sensitive host bind mounts in " + snap.Path + ".",
			Evidence: map[string]any{"composeFile": snap.Path},
		}
	}
	return CheckResult{
		ID: "compose.bind_mount_sensitive_paths", Title: "compose does not bind-mount sensitive host paths",
		Category: "compose", Severity: High, Status: Failed,
		Summary:     fmt.Sprintf("%d compose mount(s) reference sensitive host paths.", len(hits)),
		Remediation: "Remove host bind mounts of /var/run/docker.sock (without explicit allow-list), /etc, /root, /proc, /sys, or root '/'. Use named volumes instead.",
		Evidence:    map[string]any{"composeFile": snap.Path, "lines": hits},
	}
}

func dirOf(path string) string {
	if idx := strings.LastIndex(path, "/"); idx > 0 {
		return path[:idx]
	}
	return "."
}

// shellQuote mirrors the harden-package helper but kept local to avoid an import cycle.
func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'"
}
