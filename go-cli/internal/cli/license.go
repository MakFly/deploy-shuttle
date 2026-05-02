package cli

import (
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/MakFly/deploy-shuttle/go-cli/internal/license"
	"github.com/MakFly/deploy-shuttle/go-cli/internal/version"
	"github.com/spf13/cobra"
)

func newLicenseCommand() *cobra.Command {
	root := &cobra.Command{Use: "license", Short: "Manage your DeployShuttle Pro license"}
	root.AddCommand(newLicenseActivateCommand())
	root.AddCommand(newLicenseStatusCommand())
	root.AddCommand(newLicenseRefreshCommand())
	root.AddCommand(newLicenseDeactivateCommand())
	return root
}

func newLicenseActivateCommand() *cobra.Command {
	var server string
	cmd := &cobra.Command{
		Use:   "activate <key>",
		Short: "Activate a Pro license on this machine",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			serverURL := resolveServer(server)
			client := license.NewClient(serverURL)
			fp := license.MachineFingerprint()
			ctx, cancel := context.WithTimeout(cmd.Context(), 15*time.Second)
			defer cancel()
			resp, err := client.Activate(ctx, args[0], fp, version.Version)
			if err != nil {
				return err
			}
			state := license.State{
				Token:       resp.Token,
				ServerURL:   serverURL,
				Tier:        resp.Tier,
				ExpiresAt:   resp.ExpiresAt.UTC(),
				RefreshAt:   time.Now().UTC().Add(refreshLeadFor(resp.ExpiresAt)),
				ActivatedAt: time.Now().UTC(),
			}
			if err := license.Save("", state); err != nil {
				return err
			}
			fmt.Printf("DeployShuttle Pro activated. Tier: %s. Offline grace until %s.\n", state.Tier, state.ExpiresAt.Format(time.RFC3339))
			return nil
		},
	}
	cmd.Flags().StringVar(&server, "server", "", "license server URL (default: env DEPLOY_SHUTTLE_LICENSE_SERVER or built-in)")
	return cmd
}

func newLicenseStatusCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show the current license tier and offline grace window",
		RunE: func(cmd *cobra.Command, args []string) error {
			state, err := license.Load("")
			if err != nil {
				if errors.Is(err, license.ErrNoLicense) {
					fmt.Println("No license activated. DeployShuttle is running on the Free tier.")
					fmt.Printf("Buy Pro: %s\n", license.CheckoutURL)
					return nil
				}
				return err
			}
			fmt.Printf("Tier        : %s\n", state.Tier)
			fmt.Printf("Server      : %s\n", state.ServerURL)
			fmt.Printf("Activated   : %s\n", state.ActivatedAt.Format(time.RFC3339))
			fmt.Printf("Expires     : %s (in %s)\n", state.ExpiresAt.Format(time.RFC3339), humanDuration(time.Until(state.ExpiresAt)))
			fmt.Printf("Refresh due : %s\n", state.RefreshAt.Format(time.RFC3339))
			if version.LicensePubKeyB64 == "" {
				fmt.Println("Build       : dev (license verification is disabled)")
				return nil
			}
			pub, decodeErr := decodePubKeyB64(version.LicensePubKeyB64)
			if decodeErr != nil {
				fmt.Printf("Verify      : ERROR (embedded public key is invalid: %s)\n", decodeErr)
				return nil
			}
			if _, vErr := license.VerifyToken(state.Token, pub, time.Now().UTC(), license.MachineFingerprint()); vErr != nil {
				fmt.Printf("Verify      : %s\n", vErr)
				return nil
			}
			fmt.Println("Verify      : ok")
			return nil
		},
	}
}

func newLicenseRefreshCommand() *cobra.Command {
	var server string
	cmd := &cobra.Command{
		Use:   "refresh",
		Short: "Force-refresh the cached license token",
		RunE: func(cmd *cobra.Command, args []string) error {
			state, err := license.Load("")
			if err != nil {
				return err
			}
			serverURL := state.ServerURL
			if server != "" {
				serverURL = server
			}
			client := license.NewClient(serverURL)
			ctx, cancel := context.WithTimeout(cmd.Context(), 15*time.Second)
			defer cancel()
			resp, err := client.Refresh(ctx, state.Token)
			if err != nil {
				return err
			}
			state.Token = resp.Token
			state.Tier = resp.Tier
			state.ExpiresAt = resp.ExpiresAt.UTC()
			state.RefreshAt = time.Now().UTC().Add(refreshLeadFor(resp.ExpiresAt))
			if err := license.Save("", state); err != nil {
				return err
			}
			fmt.Printf("License refreshed. New offline grace until %s.\n", state.ExpiresAt.Format(time.RFC3339))
			return nil
		},
	}
	cmd.Flags().StringVar(&server, "server", "", "override the stored license server URL")
	return cmd
}

func newLicenseDeactivateCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "deactivate",
		Short: "Remove the cached license token from this machine",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := license.Clear(""); err != nil {
				return err
			}
			fmt.Println("License removed from this machine.")
			return nil
		},
	}
}

func resolveServer(flag string) string {
	if flag != "" {
		return flag
	}
	if env := os.Getenv("DEPLOY_SHUTTLE_LICENSE_SERVER"); env != "" {
		return env
	}
	return version.LicenseServer
}

func refreshLeadFor(exp time.Time) time.Duration {
	d := time.Until(exp)
	if d < 48*time.Hour {
		return 0
	}
	return d - 48*time.Hour
}

func humanDuration(d time.Duration) string {
	if d <= 0 {
		return "expired"
	}
	if d > 24*time.Hour {
		return fmt.Sprintf("%d days", int(d.Hours())/24)
	}
	if d > time.Hour {
		return fmt.Sprintf("%d hours", int(d.Hours()))
	}
	return fmt.Sprintf("%d minutes", int(d.Minutes()))
}

func decodePubKeyB64(b64 string) (ed25519.PublicKey, error) {
	raw, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		raw, err = base64.RawURLEncoding.DecodeString(b64)
		if err != nil {
			return nil, err
		}
	}
	if len(raw) != ed25519.PublicKeySize {
		return nil, fmt.Errorf("expected %d bytes, got %d", ed25519.PublicKeySize, len(raw))
	}
	return ed25519.PublicKey(raw), nil
}
