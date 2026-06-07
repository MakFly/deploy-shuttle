package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"runtime"
	"strings"

	"github.com/MakFly/deploy-shuttle/go-cli/internal/version"
	"github.com/spf13/cobra"
)

func newUpdateCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "update",
		Short: "Update shuttle to the latest version",
		RunE:  runUpdate,
	}
}

func runUpdate(cmd *cobra.Command, args []string) error {
	repo := "MakFly/deploy-shuttle"

	fmt.Print("Checking latest version... ")
	latest, err := fetchLatestTag(repo)
	if err != nil {
		return fmt.Errorf("failed to check latest version: %w", err)
	}
	fmt.Printf("%s\n", latest)

	current := version.Version
	if current == latest || "v"+current == latest || current == "v"+latest {
		fmt.Printf("\n✓ Already up to date (%s)\n", current)
		return nil
	}

	fmt.Printf("Current: %s → Latest: %s\n", current, latest)

	asset := assetName()
	if asset == "" {
		return fmt.Errorf("no prebuilt binary for %s/%s", runtime.GOOS, runtime.GOARCH)
	}

	url := fmt.Sprintf("https://github.com/%s/releases/download/%s/%s", repo, latest, asset)
	fmt.Printf("Downloading %s ...\n", asset)

	bin, err := downloadBinary(url)
	if err != nil {
		return fmt.Errorf("download failed: %w", err)
	}

	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("cannot locate current binary: %w", err)
	}

	if err := replaceBinary(exe, bin); err != nil {
		return fmt.Errorf("failed to replace binary: %w", err)
	}

	fmt.Printf("\n✓ Updated to %s\n", latest)
	return nil
}

func fetchLatestTag(repo string) (string, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/releases/latest", repo)
	resp, err := http.Get(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return "", fmt.Errorf("GitHub API returned %d", resp.StatusCode)
	}
	var release struct {
		TagName string `json:"tag_name"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return "", err
	}
	return release.TagName, nil
}

func assetName() string {
	osTag := ""
	switch runtime.GOOS {
	case "linux":
		osTag = "linux"
	case "darwin":
		osTag = "darwin"
	default:
		return ""
	}
	archTag := ""
	switch runtime.GOARCH {
	case "amd64":
		archTag = "x64"
	case "arm64":
		archTag = "arm64"
	default:
		return ""
	}
	return fmt.Sprintf("shuttle-%s-%s", osTag, archTag)
}

func downloadBinary(url string) ([]byte, error) {
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("HTTP %d from %s", resp.StatusCode, url)
	}
	return io.ReadAll(resp.Body)
}

func replaceBinary(path string, data []byte) error {
	tmp := path + ".new"
	if err := os.WriteFile(tmp, data, 0o755); err != nil {
		return err
	}
	old := path + ".old"
	_ = os.Remove(old)
	if err := os.Rename(path, old); err != nil {
		if !strings.Contains(err.Error(), "no such file") {
			return err
		}
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Rename(old, path)
		return err
	}
	_ = os.Remove(old)
	return nil
}
