package cli

import (
	"fmt"
	"net"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/MakFly/deploy-shuttle/go-cli/internal/config"
	"github.com/MakFly/deploy-shuttle/go-cli/internal/output"
)

const vpnCheckTimeout = 3 * time.Second

func ensureVPNReachable(group config.ServerGroup, host string) error {
	vpn := group.VPN
	if !vpn.Required {
		return nil
	}

	if vpn.Interface != "" {
		if err := checkWireGuardInterface(vpn.Interface); err != nil {
			return err
		}
	}

	checkHost := vpn.CheckHost
	if checkHost == "" {
		checkHost = host
	}
	checkPort := vpn.CheckPort
	if checkPort == 0 {
		checkPort = group.Port
	}
	if checkPort == 0 {
		checkPort = 22
	}

	address := net.JoinHostPort(checkHost, strconv.Itoa(checkPort))
	output.Detail("VPN required; checking %s", address)
	conn, err := net.DialTimeout("tcp", address, vpnCheckTimeout)
	if err != nil {
		return fmt.Errorf("vpn required for %s, but %s is not reachable; start WireGuard%s and retry: %w", host, address, interfaceHint(vpn.Interface), err)
	}
	_ = conn.Close()
	return nil
}

func checkWireGuardInterface(name string) error {
	if _, err := exec.LookPath("wg"); err != nil {
		output.Debug("wg binary not found; relying on TCP VPN reachability check")
		return nil
	}
	out, err := exec.Command("wg", "show", name).CombinedOutput()
	if err == nil {
		return nil
	}
	detail := strings.TrimSpace(string(out))
	if detail == "" {
		detail = err.Error()
	}
	return fmt.Errorf("vpn required but WireGuard interface %q is not active or not readable: %s", name, detail)
}

func interfaceHint(name string) string {
	if name == "" {
		return ""
	}
	return fmt.Sprintf(" interface %q", name)
}
