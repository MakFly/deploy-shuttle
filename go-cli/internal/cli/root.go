package cli

import (
	"github.com/MakFly/deploy-shuttle/go-cli/internal/output"
	"github.com/MakFly/deploy-shuttle/go-cli/internal/version"
	"github.com/spf13/cobra"
)

func NewRootCommand() *cobra.Command {
	root := &cobra.Command{
		Use:     "shuttle",
		Short:   "Audit, harden and deploy Docker apps on VPS",
		Version: version.Version,
	}
	root.PersistentFlags().BoolVarP(&output.Verbose, "verbose", "v", false, "enable verbose output")
	root.AddCommand(
		newDoctorCommand(),
		newReportCommand(),
		newHardenCommand(),
		newValidateCommand(),
		newInitCommand(),
		newNewCommand(),
		newSecretsCommand(),
		newProvisionCommand(),
		newDeployCommand(),
		newStatusCommand(),
		newLogsCommand(),
		newSSHCommand(),
		newExecCommand(),
		newRollbackCommand(),
		newDestroyCommand(),
		newLockCommand(),
		newDevCommand(),
		newMonitorCommand(),
		newCICommand(),
		newLicenseCommand(),
		newUpdateCommand(),
		newUninstallCommand(),
	)
	return root
}
