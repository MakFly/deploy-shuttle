package execx

import (
	"time"

	"github.com/MakFly/deploy-shuttle/go-cli/internal/ssh"
)

type SSH struct {
	Client *ssh.Client
}

func (s SSH) Run(command string, timeout time.Duration) Result {
	if s.Client == nil {
		return Result{ExitCode: 255, Stderr: "missing SSH client"}
	}
	res := s.Client.Run(command)
	return Result{
		ExitCode: res.Code,
		Stdout:   res.Stdout,
		Stderr:   res.Stderr,
	}
}
