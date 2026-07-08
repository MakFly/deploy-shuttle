package runtime

import (
	"fmt"
	"path"
	"strings"
)

func AppDir(app string, deployPath ...string) string {
	if base := cleanDeployPath(deployPath...); base != "" {
		return base
	}
	return fmt.Sprintf("/opt/shuttle/%s", app)
}

func WorkDir(app string, deployPath ...string) string {
	return path.Join(AppDir(app, deployPath...), app)
}

func StatePath(app string, deployPath ...string) string {
	return path.Join(AppDir(app, deployPath...), "state.json")
}

func LockDir(app string, deployPath ...string) string {
	return path.Join(AppDir(app, deployPath...), ".deploy.lock")
}

func BlueGreenDir(app, slot string, deployPath ...string) string {
	return path.Join(AppDir(app, deployPath...), slot) + "/"
}

func cleanDeployPath(deployPath ...string) string {
	if len(deployPath) == 0 {
		return ""
	}
	cleaned := path.Clean(strings.TrimSpace(deployPath[0]))
	if cleaned == "." || cleaned == "/" {
		return ""
	}
	return cleaned
}
