package cli

import (
	"github.com/MakFly/deploy-shuttle/go-cli/internal/output"
	"github.com/spf13/cobra"
)

func NewRootCommand() *cobra.Command {
	root := &cobra.Command{
		Use:     "deploy-shuttle",
		Short:   "Audit, harden and deploy Docker apps on VPS",
		Version: "0.1.0",
	}
	root.PersistentFlags().BoolVarP(&output.Verbose, "verbose", "v", false, "enable verbose output")
	root.AddCommand(
		newDoctorCommand(),
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
	)
	return root
}
