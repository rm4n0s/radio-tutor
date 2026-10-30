package phone

import (
	"context"
	"io"
	"net"
	"reflect"
	"strings"
	"testing"
	"testing/iotest"
)

// Sample /proc/net/route files (tab separated, as the kernel writes them).
const (
	routeHeader = "Iface\tDestination\tGateway \tFlags\tRefCnt\tUse\tMetric\tMask\t\tMTU\tWindow\tIRTT\n"

	// Wi-Fi is the default route; Docker's bridge has a connected route only.
	routeWifi = routeHeader +
		"wlp3s0\t00000000\t0100A8C0\t0003\t0\t0\t600\t00000000\t0\t0\t0\n" +
		"wlp3s0\t0000A8C0\t00000000\t0001\t0\t0\t600\t00FFFFFF\t0\t0\t0\n" +
		"docker0\t000011AC\t00000000\t0001\t0\t0\t0\t0000FFFF\t0\t0\t0\n"

	// Ethernet (metric 100) beats Wi-Fi (metric 600), whatever the order.
	routeTwoDefaults = routeHeader +
		"wlp3s0\t00000000\t0100A8C0\t0003\t0\t0\t600\t00000000\t0\t0\t0\n" +
		"enp2s0\t00000000\t0100000A\t0003\t0\t0\t100\t00000000\t0\t0\t0\n" +
		"enp2s0\t00000000\t0200000A\t0003\t0\t0\t100\t00000000\t0\t0\t0\n"

	// A default route that is not up, lines that are damaged, and a route
	// that is not the default one.
	routeJunk = routeHeader +
		"eth9\t00000000\t0100A8C0\t0002\t0\t0\t10\t00000000\t0\t0\t0\n" +
		"short line\n" +
		"eth8\t00000000\t0100A8C0\tZZZZ\t0\t0\t10\t00000000\t0\t0\t0\n" +
		"eth7\t00000000\t0100A8C0\t0003\t0\t0\tabc\t00000000\t0\t0\t0\n" +
		"eth6\t0000A8C0\t0100A8C0\t0003\t0\t0\t10\t00FFFFFF\t0\t0\t0\n" +
		"\n"
)

func ips(addrs ...string) []net.IP {
	out := make([]net.IP, len(addrs))
	for i, a := range addrs {
		out[i] = net.ParseIP(a)
	}
	return out
}

func TestParseDefaultRoutes(t *testing.T) {
	tests := []struct {
		name  string
		table string
		want  []string
	}{
		{"wifi only", routeWifi, []string{"wlp3s0"}},
		{"lowest metric first, no repeats", routeTwoDefaults, []string{"enp2s0", "wlp3s0"}},
		{"junk is skipped", routeJunk, nil},
		{"empty", "", nil},
		{"header only", routeHeader, nil},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ParseDefaultRoutes(strings.NewReader(tc.table))
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("got %v, want %v", got, tc.want)
			}
		})
	}
}

func TestParseDefaultRoutesReadError(t *testing.T) {
	_, err := ParseDefaultRoutes(iotest.ErrReader(io.ErrUnexpectedEOF))
	wantRoute(t, err, "ParseDefaultRoutes.PhoneRouteTableUnreadable")
}

func TestChooseLAN(t *testing.T) {
	wifi := Interface{Name: "wlp3s0", Up: true, Addrs: ips("fe80::1", "192.168.1.20")}
	eth := Interface{Name: "enp2s0", Up: true, Addrs: ips("10.1.2.3")}
	docker := Interface{Name: "docker0", Up: true, Addrs: ips("172.17.0.1")}
	loopback := Interface{Name: "lo", Up: true, Loopback: true, Addrs: ips("127.0.0.1", "10.99.99.99")}
	downEth := Interface{Name: "enp2s0", Up: false, Addrs: ips("10.1.2.3")}
	vpnPublic := Interface{Name: "tun0", Up: true, Addrs: ips("203.0.113.5", "169.254.3.4")}
	v6only := Interface{Name: "wlp3s0", Up: true, Addrs: ips("fd00::5", "2001:db8::1")}
	bridge0 := Interface{Name: "br0", Up: true, Addrs: ips("192.168.50.1")}
	libvirt := Interface{Name: "virbr0", Up: true, Addrs: ips("192.168.122.1")}
	veth := Interface{Name: "veth1a2b", Up: true, Addrs: ips("10.5.5.5")}
	wg := Interface{Name: "wg0", Up: true, Addrs: ips("10.8.0.2")}
	range172 := Interface{Name: "br0", Up: true, Addrs: ips("172.31.255.1")}
	not172 := Interface{Name: "br1", Up: true, Addrs: ips("172.32.0.1", "172.15.0.1")}

	tests := []struct {
		name      string
		route     string
		ifaces    []Interface
		preferred string
		wantIface string
		wantIP    string
		noNetwork bool
	}{
		{"default route interface beats the order", routeWifi, []Interface{docker, loopback, wifi}, "", "wlp3s0", "192.168.1.20", false},
		{"lowest metric default route", routeTwoDefaults, []Interface{wifi, eth}, "", "enp2s0", "10.1.2.3", false},
		{"explicit interface beats the route", routeWifi, []Interface{wifi, eth}, "enp2s0", "enp2s0", "10.1.2.3", false},
		{"explicit interface without an address falls back", routeWifi, []Interface{wifi, eth}, "nosuch0", "wlp3s0", "192.168.1.20", false},
		{"explicit interface that is down falls back", routeWifi, []Interface{wifi, downEth}, "enp2s0", "wlp3s0", "192.168.1.20", false},
		{"default route interface is down", routeTwoDefaults, []Interface{wifi, downEth}, "", "wlp3s0", "192.168.1.20", false},
		{"no route table: the first usable interface that is not virtual", "", []Interface{loopback, vpnPublic, docker, wifi}, "", "wlp3s0", "192.168.1.20", false},
		{"no route table: a bridge named br0 is not virtual", "", []Interface{docker, bridge0, wifi}, "", "br0", "192.168.50.1", false},
		{"no route table: only virtual interfaces", "", []Interface{docker, libvirt, veth, wg}, "", "", "", true},
		{"the default route interface wins even when it looks virtual", "docker0\t00000000\t0101A8C0\t0003\t0\t0\t0\t00000000\t0\t0\t0\n", []Interface{wifi, docker}, "", "docker0", "172.17.0.1", false},
		{"an explicit virtual interface is used", "", []Interface{wifi, docker}, "docker0", "docker0", "172.17.0.1", false},
		{"a route to an interface without a private address", routeTwoDefaults, []Interface{vpnPublic, wifi}, "", "wlp3s0", "192.168.1.20", false},
		{"172.16/12 edges", "", []Interface{not172, range172}, "", "br0", "172.31.255.1", false},
		{"only loopback", routeWifi, []Interface{loopback}, "", "", "", true},
		{"only public and link-local", "", []Interface{vpnPublic}, "", "", "", true},
		{"only IPv6", routeWifi, []Interface{v6only}, "", "", "", true},
		{"everything down", routeWifi, []Interface{{Name: "wlp3s0", Up: false, Addrs: ips("192.168.1.20")}}, "", "", "", true},
		{"no interfaces", routeWifi, nil, "", "", "", true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var route io.Reader
			if tc.route != "" {
				route = strings.NewReader(tc.route)
			}
			got, err := ChooseLAN(route, tc.ifaces, tc.preferred)
			if tc.noNetwork {
				wantRoute(t, err, "chooseLAN.PhoneNoNetwork")
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if got.Interface != tc.wantIface || got.IP.String() != tc.wantIP {
				t.Errorf("got %s on %s, want %s on %s", got.IP, got.Interface, tc.wantIP, tc.wantIface)
			}
		})
	}
}

func TestChooseLANIgnoresAnUnreadableRouteTable(t *testing.T) {
	ifaces := []Interface{{Name: "wlp3s0", Up: true, Addrs: ips("192.168.1.20")}}
	got, err := ChooseLAN(iotest.ErrReader(io.ErrClosedPipe), ifaces, "")
	if err != nil || got.Interface != "wlp3s0" {
		t.Errorf("got %v, %v", got, err)
	}
}

func TestChooseLANDuplicateNames(t *testing.T) {
	ifaces := []Interface{
		{Name: "eth0", Up: true, Addrs: ips("10.0.0.1")},
		{Name: "eth0", Up: true, Addrs: ips("10.0.0.2")},
	}
	got, err := ChooseLAN(nil, ifaces, "")
	if err != nil || got.IP.String() != "10.0.0.1" {
		t.Errorf("got %v, %v", got, err)
	}
}

func TestListInterfacesFailure(t *testing.T) {
	_, err := listInterfaces(func() ([]net.Interface, error) { return nil, io.ErrNoProgress })
	wantRoute(t, err, "listInterfaces.PhoneInterfacesFailed")
}

func TestListInterfacesReal(t *testing.T) {
	ifaces, err := ListInterfaces()
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, ifc := range ifaces {
		if ifc.Loopback {
			found = true
		}
	}
	if !found {
		t.Skip("no loopback interface in this environment")
	}
}

func TestDetectLAN(t *testing.T) {
	got, err := DetectLAN(context.Background(), "")
	if err != nil {
		// A machine without a network (a sandbox) has none to offer.
		wantRoute(t, err, "chooseLAN.PhoneNoNetwork")
		return
	}
	if !got.IP.To4().IsPrivate() {
		t.Errorf("address %s is not private", got.IP)
	}
}

func TestDetectLANCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := DetectLAN(ctx, "")
	wantRoute(t, err, "DetectLAN.PhoneDetectCanceled")
}

func TestVirtualInterface(t *testing.T) {
	tests := []struct {
		name string
		want bool
	}{
		{"docker0", true}, {"br-1a2b3c", true}, {"veth9f", true}, {"virbr0", true}, {"lo", true},
		{"tun0", true}, {"tap1", true}, {"wg0", true},
		{"vEthernet (Default Switch)", true}, {"Loopback Pseudo-Interface 1", true}, {"VMware Network Adapter VMnet1", true},
		{"VirtualBox Host-Only Network", true}, {"Bluetooth Network Connection", true},
		{"Ethernet", false}, {"Wi-Fi", false}, {"Ethernet 2", false},
		{"wlp3s0", false}, {"enp2s0", false}, {"wlan0", false}, {"eth0", false}, {"br0", false}, {"", false},
	}
	for _, tc := range tests {
		if got := VirtualInterface(tc.name); got != tc.want {
			t.Errorf("VirtualInterface(%q) = %v, want %v", tc.name, got, tc.want)
		}
	}
}
