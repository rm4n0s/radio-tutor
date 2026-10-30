//go:build windows

package phone

import "net"

// probeAddr is an address of the documentation range (RFC 5737): nothing
// answers there, and connecting a UDP socket to it sends no packet. It only asks
// the system which local address it would use to reach the internet.
const probeAddr = "192.0.2.1:9"

// defaultRouteInterfaces returns the name of the interface that carries the
// default route: Windows has no /proc/net/route, so a UDP socket is "connected"
// to a public address (no packet is sent) and the local address the system
// picked is looked up among the interfaces. No default route (no network) gives
// none.
func defaultRouteInterfaces(ifaces []Interface) []string {
	return defaultRouteInterfacesVia(ifaces, func() (net.IP, error) {
		c, err := net.Dial("udp4", probeAddr)
		if err != nil {
			return nil, err
		}
		defer func() { _ = c.Close() }()
		if a, ok := c.LocalAddr().(*net.UDPAddr); ok {
			return a.IP, nil
		}
		return nil, nil
	})
}

// defaultRouteInterfacesVia is defaultRouteInterfaces with the lookup of the
// source address given, so that tests can replace the network.
func defaultRouteInterfacesVia(ifaces []Interface, source func() (net.IP, error)) []string {
	ip, err := source()
	if err != nil || ip == nil {
		return nil
	}
	var names []string
	for _, ifc := range ifaces {
		for _, a := range ifc.Addrs {
			if a.Equal(ip) {
				names = append(names, ifc.Name)
				break
			}
		}
	}
	return names
}
