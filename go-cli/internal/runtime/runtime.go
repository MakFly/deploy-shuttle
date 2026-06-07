package runtime

import "fmt"

func AppDir(app string) string {
	return fmt.Sprintf("/opt/shuttle/%s", app)
}

func WorkDir(app string) string {
	return fmt.Sprintf("/opt/shuttle/%s/%s", app, app)
}

func StatePath(app string) string {
	return fmt.Sprintf("/opt/shuttle/%s/state.json", app)
}

func LockDir(app string) string {
	return fmt.Sprintf("/opt/shuttle/%s/.deploy.lock", app)
}

func BlueGreenDir(app, slot string) string {
	return fmt.Sprintf("/opt/shuttle/%s/%s/", app, slot)
}
