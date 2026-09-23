package prereq

import (
	"strings"
	"testing"
)

func TestParseHostInterfacesSortsAndDedupes(t *testing.T) {
	raw := []byte(`{"SPNetworkDataType":[{"interface":"en1"},{"_name":"VPN"},{"interface":"bridge0"},{"interface":"en1"}]}`)
	got, err := parseHostInterfaces(raw)
	if err != nil || strings.Join(got, ",") != "bridge0,en1" {
		t.Fatalf("got %v, %v", got, err)
	}
}

func TestRancherSudoersMatchesRancherLayout(t *testing.T) {
	got := rancherSudoers([]string{"en0"})
	want := "%everyone ALL=(root:wheel) NOPASSWD:NOSETENV: /bin/mkdir -m 775 -p /private/var/run\n\n" +
		"# Manage \"host\" network daemons\n\n" +
		"%everyone ALL=(root:wheel) NOPASSWD:NOSETENV: \\\n" +
		"    /opt/rancher-desktop/bin/socket_vmnet --pidfile=/private/var/run/host_socket_vmnet.pid --socket-group=everyone --vmnet-mode=host --vmnet-gateway=192.168.206.1 --vmnet-dhcp-end=192.168.206.254 --vmnet-mask=255.255.255.0 /private/var/run/socket_vmnet.host, \\\n" +
		"    /usr/bin/pkill -F /private/var/run/host_socket_vmnet.pid\n\n" +
		"# Manage \"rancher-desktop-bridged_en0\" network daemons\n\n" +
		"%everyone ALL=(root:wheel) NOPASSWD:NOSETENV: \\\n" +
		"    /opt/rancher-desktop/bin/socket_vmnet --pidfile=/private/var/run/rancher-desktop-bridged_en0_socket_vmnet.pid --socket-group=everyone --vmnet-mode=bridged --vmnet-interface=en0 /private/var/run/socket_vmnet.rancher-desktop-bridged_en0, \\\n" +
		"    /usr/bin/pkill -F /private/var/run/rancher-desktop-bridged_en0_socket_vmnet.pid\n\n" +
		"# Manage \"rancher-desktop-shared\" network daemons\n\n" +
		"%everyone ALL=(root:wheel) NOPASSWD:NOSETENV: \\\n" +
		"    /opt/rancher-desktop/bin/socket_vmnet --pidfile=/private/var/run/rancher-desktop-shared_socket_vmnet.pid --socket-group=everyone --vmnet-mode=shared --vmnet-gateway=192.168.205.1 --vmnet-dhcp-end=192.168.205.254 --vmnet-mask=255.255.255.0 /private/var/run/socket_vmnet.rancher-desktop-shared, \\\n" +
		"    /usr/bin/pkill -F /private/var/run/rancher-desktop-shared_socket_vmnet.pid\n"
	if got != want {
		t.Fatalf("sudoers drifted from Rancher Desktop's layout:\n%s", got)
	}
}
