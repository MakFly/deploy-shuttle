package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

// runProWizard fills unset --with-* answers interactively (Spin-Pro-style
// onboarding). Explicit flags act as non-interactive answers and skip their
// prompt. On non-TTY/EOF stdin every prompt returns its default, which
// matches the previous --pro auto-enable set exactly (postgres + redis +
// queue + scheduler + mailpit + ci).
func runProWizard(cmd *cobra.Command, withDB *string, withRedis, withQueue, withScheduler, withMailpit, withCI *bool) {
	fmt.Println("Pro setup — pick your stack services (Enter = default):")
	if !cmd.Flags().Changed("with-db") {
		choice := promptChoice("Database", []string{"postgres (recommended)", "mysql", "none"}, 0)
		switch {
		case strings.HasPrefix(choice, "postgres"):
			*withDB = "postgres"
		case choice == "mysql":
			*withDB = "mysql"
		default:
			*withDB = ""
		}
	}
	if !cmd.Flags().Changed("with-redis") {
		*withRedis = promptConfirm("Redis (cache / queue broker)?", true)
	}
	if !cmd.Flags().Changed("with-queue") {
		*withQueue = promptConfirm("Queue worker?", *withRedis)
	}
	if !cmd.Flags().Changed("with-scheduler") {
		*withScheduler = promptConfirm("Scheduler (cron)?", true)
	}
	if !cmd.Flags().Changed("with-mailpit") {
		*withMailpit = promptConfirm("Mailpit (dev email)?", true)
	}
	if !cmd.Flags().Changed("ci") {
		*withCI = promptConfirm("GitHub Actions workflow?", true)
	}

	services := make([]string, 0, 5)
	if *withDB != "" {
		services = append(services, *withDB)
	}
	if *withRedis {
		services = append(services, "redis")
	}
	if *withQueue {
		services = append(services, "queue")
	}
	if *withScheduler {
		services = append(services, "scheduler")
	}
	if *withMailpit {
		services = append(services, "mailpit")
	}
	summary := strings.Join(services, ", ")
	if summary == "" {
		summary = "none"
	}
	if *withCI {
		summary += " + CI"
	}
	fmt.Printf("  → Pro services: %s\n\n", summary)
}
