package cli

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"

	"github.com/spf13/cobra"
)

func newSSHCommand() *cobra.Command {
	var flags configFlags
	var serverName string
	cmd := &cobra.Command{
		Use:   "ssh",
		Short: "Open an SSH session to the first configured server",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadWithFlags(flags)
			if err != nil {
				return err
			}
			for name, group := range cfg.Servers {
				if serverName != "" && name != serverName {
					continue
				}
				for _, host := range group.Hosts {
					target := host
					if group.User != "" {
						target = fmt.Sprintf("%s@%s", group.User, host)
					}
					sshArgs := []string{"-p", strconv.Itoa(group.Port), target}
					sshCmd := exec.Command("ssh", sshArgs...)
					sshCmd.Stdin = os.Stdin
					sshCmd.Stdout = os.Stdout
					sshCmd.Stderr = os.Stderr
					return sshCmd.Run()
				}
			}
			if serverName != "" {
				return fmt.Errorf("server group %q not found", serverName)
			}
			return fmt.Errorf("no servers configured")
		},
	}
	addConfigFlags(cmd, &flags)
	cmd.Flags().StringVar(&serverName, "server", "", "server group name")
	return cmd
}
