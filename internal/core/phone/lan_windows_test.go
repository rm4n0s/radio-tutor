//go:build windows

package phone

import (
	"net"
	"reflect"
	"testing"

	"github.com/rm4n0s/errors"
)

func TestDefaultRouteInterfacesVia(t *testing.T) {
	ifaces := []Interface{
		{Name: "vEthernet (WSL)", Up: true, Addrs: []net.IP{net.ParseIP("172.20.0.1")}},
		{Name: "Wi-Fi", Up: true, Addrs: []net.IP{net.ParseIP("fe80::1"), net.ParseIP("192.168.1.20")}},
	}
	tests := []struct {
		name   string
		source func() (net.IP, error)
		want   []string
	}{
		{"source on the Wi-Fi", func() (net.IP, error) { return net.ParseIP("192.168.1.20"), nil }, []string{"Wi-Fi"}},
		{"source on no interface", func() (net.IP, error) { return net.ParseIP("10.9.9.9"), nil }, nil},
		{"no route", func() (net.IP, error) { return nil, errors.New("Fake", "unreachable") }, nil},
		{"no address", func() (net.IP, error) { return nil, nil }, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := defaultRouteInterfacesVia(ifaces, tt.source); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}

// With the Wi-Fi as the default route and Hyper-V's switch listed first, the
// Wi-Fi wins.
func TestChooseLANWindowsNames(t *testing.T) {
	ifaces := []Interface{
		{Name: "vEthernet (Default Switch)", Up: true, Addrs: []net.IP{net.ParseIP("172.28.0.1")}},
		{Name: "Wi-Fi", Up: true, Addrs: []net.IP{net.ParseIP("192.168.1.20")}},
	}
	got, err := chooseLAN(nil, ifaces, "")
	if err != nil || got.Interface != "Wi-Fi" {
		t.Errorf("without a route: %+v, %v", got, err)
	}
	got, err = chooseLAN([]string{"vEthernet (Default Switch)"}, ifaces, "")
	if err != nil || got.Interface != "vEthernet (Default Switch)" {
		t.Errorf("a virtual interface with the default route is used: %+v, %v", got, err)
	}
}
