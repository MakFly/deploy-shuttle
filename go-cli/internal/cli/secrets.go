package cli

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/MakFly/deploy-shuttle/go-cli/internal/runtime"
	"github.com/MakFly/deploy-shuttle/go-cli/internal/secrets"
	"github.com/MakFly/deploy-shuttle/go-cli/internal/shell"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

func newSecretsCommand() *cobra.Command {
	var path string
	var flags configFlags
	root := &cobra.Command{Use: "secrets", Short: "Manage local encrypted secrets"}
	store := func() (secrets.Store, error) {
		passphrase := os.Getenv("SHUTTLE_SECRETS_PASSPHRASE")
		if passphrase == "" && term.IsTerminal(int(os.Stdin.Fd())) {
			fmt.Fprint(os.Stderr, "Secrets passphrase: ")
			raw, err := term.ReadPassword(int(os.Stdin.Fd()))
			fmt.Fprintln(os.Stderr)
			if err != nil {
				return secrets.Store{}, err
			}
			passphrase = string(raw)
		}
		return secrets.Store{Path: path, Passphrase: passphrase}, nil
	}
	root.PersistentFlags().StringVar(&path, "file", ".shuttle/secrets.enc", "encrypted secrets file")
	root.AddCommand(&cobra.Command{
		Use:   "set <key> <value>",
		Short: "Set a secret",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			store, err := store()
			if err != nil {
				return err
			}
			return store.Set(args[0], args[1])
		},
	})
	root.AddCommand(&cobra.Command{
		Use:   "get <key>",
		Short: "Get a secret",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			store, err := store()
			if err != nil {
				return err
			}
			value, ok, err := store.Get(args[0])
			if err != nil {
				return err
			}
			if !ok {
				return fmt.Errorf("secret %q not found", args[0])
			}
			fmt.Println(value)
			return nil
		},
	})
	root.AddCommand(&cobra.Command{
		Use:   "list",
		Short: "List secret keys",
		RunE: func(cmd *cobra.Command, args []string) error {
			store, err := store()
			if err != nil {
				return err
			}
			keys, err := store.List()
			if err != nil {
				return err
			}
			sort.Strings(keys)
			for _, key := range keys {
				fmt.Println(key)
			}
			return nil
		},
	})
	root.AddCommand(&cobra.Command{
		Use:   "remove <key>",
		Short: "Remove a secret",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			store, err := store()
			if err != nil {
				return err
			}
			return store.Remove(args[0])
		},
	})
	pushCmd := &cobra.Command{
		Use:   "push",
		Short: "Push secrets to a remote host",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadWithFlags(flags)
			if err != nil {
				return err
			}
			store, err := store()
			if err != nil {
				return err
			}
			values, err := store.LoadAll()
			if err != nil {
				return err
			}
			if len(values) == 0 {
				fmt.Println("No secrets to push.")
				return nil
			}
			content := formatEnv(values)
			for _, group := range cfg.Servers {
				for _, host := range group.Hosts {
					client, err := connectSSH(group, host)
					if err != nil {
						return err
					}
					remoteDir := runtime.AppDir(cfg.App)
					mkdir := client.Run("mkdir -p " + shell.Escape(remoteDir))
					if mkdir.Code != 0 {
						return fmt.Errorf("failed to create remote secrets directory on %s: %s", host, mkdir.Stderr)
					}
					res := client.UploadContent(content, remoteDir+"/.env", 0o600)
					if res.Code != 0 {
						return fmt.Errorf("failed to push secrets to %s: %s", host, res.Stderr)
					}
					fmt.Printf("Secrets pushed to %s:%s/.env\n", host, remoteDir)
				}
			}
			return nil
		},
	}
	addConfigFlags(pushCmd, &flags)
	root.AddCommand(pushCmd)
	return root
}

func formatEnv(values map[string]string) string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	lines := make([]string, 0, len(keys))
	for _, key := range keys {
		value := values[key]
		value = strings.ReplaceAll(value, "\\", "\\\\")
		value = strings.ReplaceAll(value, "\"", "\\\"")
		value = strings.ReplaceAll(value, "$", "\\$")
		value = strings.ReplaceAll(value, "`", "\\`")
		value = strings.ReplaceAll(value, "\n", "\\n")
		lines = append(lines, fmt.Sprintf("%s=\"%s\"", key, value))
	}
	return strings.Join(lines, "\n") + "\n"
}
