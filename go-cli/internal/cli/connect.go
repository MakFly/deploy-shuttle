package cli

import (
	"github.com/MakFly/deploy-shuttle/go-cli/internal/config"
	"github.com/MakFly/deploy-shuttle/go-cli/internal/ssh"
)

func connectSSH(group config.ServerGroup, host string) (*ssh.Client, error) {
	return ssh.NewClient(host, group.User, group.Port)
}
