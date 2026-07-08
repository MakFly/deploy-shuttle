package runtime

import "testing"

func TestDefaultRuntimePaths(t *testing.T) {
	if got := AppDir("myapp"); got != "/opt/shuttle/myapp" {
		t.Fatalf("AppDir default = %q", got)
	}
	if got := StatePath("myapp"); got != "/opt/shuttle/myapp/state.json" {
		t.Fatalf("StatePath default = %q", got)
	}
	if got := LockDir("myapp"); got != "/opt/shuttle/myapp/.deploy.lock" {
		t.Fatalf("LockDir default = %q", got)
	}
	if got := BlueGreenDir("myapp", "blue"); got != "/opt/shuttle/myapp/blue/" {
		t.Fatalf("BlueGreenDir default = %q", got)
	}
}

func TestRuntimePathOverride(t *testing.T) {
	base := "/opt/deploy/myapp"
	if got := AppDir("myapp", base); got != base {
		t.Fatalf("AppDir override = %q", got)
	}
	if got := StatePath("myapp", base); got != "/opt/deploy/myapp/state.json" {
		t.Fatalf("StatePath override = %q", got)
	}
	if got := LockDir("myapp", base); got != "/opt/deploy/myapp/.deploy.lock" {
		t.Fatalf("LockDir override = %q", got)
	}
	if got := BlueGreenDir("myapp", "green", base); got != "/opt/deploy/myapp/green/" {
		t.Fatalf("BlueGreenDir override = %q", got)
	}
}
