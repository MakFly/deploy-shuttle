package readiness

import "testing"

func TestDatabaseListenersPublicButFirewallRestricted(t *testing.T) {
	output := `LISTEN 0 200 0.0.0.0:5432 0.0.0.0:* users:(("postgres",pid=2701933,fd=7))
LISTEN 0 200 [::]:5432 [::]:* users:(("postgres",pid=2701933,fd=8))`
	listeners := publicDatabaseListeners(output)
	if len(listeners) != 2 {
		t.Fatalf("expected 2 listeners, got %d", len(listeners))
	}
	ports := uniqueListenerPorts(listeners)
	if len(ports) != 1 || ports[0] != "5432" {
		t.Fatalf("expected only 5432, got %#v", ports)
	}
	firewall := `Status: active
Default: deny (incoming), allow (outgoing), deny (routed)

To                         Action      From
--                         ------      ----
5432/tcp                   ALLOW IN    127.0.0.1
5432/tcp                   ALLOW IN    172.20.0.0/16`
	if !databasePortsFirewallRestricted(firewall, ports) {
		t.Fatal("expected UFW rules to restrict public database access")
	}
}

func TestDatabaseFirewallDetectsPublicAllow(t *testing.T) {
	firewall := `Status: active
Default: deny (incoming), allow (outgoing), deny (routed)

To                         Action      From
--                         ------      ----
5432/tcp                   ALLOW IN    Anywhere`
	if databasePortsFirewallRestricted(firewall, []string{"5432"}) {
		t.Fatal("expected public allow to be treated as unrestricted")
	}
}
