package phone

import (
	"bufio"
	"context"
	"io"
	"net"
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/rm4n0s/errors"
)

// RouteTablePath is the file ParseDefaultRoutes reads on Linux.
const RouteTablePath = "/proc/net/route"

// Interface is a network interface as the LAN address choice needs it. Tests
// build them by hand; ListInterfaces reads the real ones.
type Interface struct {
	// Name is the interface name, such as "wlan0".
	Name string
	// Up reports whether the interface is up.
	Up bool
	// Loopback reports whether it is a loopback interface.
	Loopback bool
	// Addrs are the interface's addresses.
	Addrs []net.IP
}

// LANAddr is the chosen address of this computer on the local network.
type LANAddr struct {
	// IP is the private IPv4 address.
	IP net.IP
	// Interface is the name of its interface.
	Interface string
}

// ParseDefaultRoutes reads the content of /proc/net/route and returns the names
// of the interfaces that have a default route (destination and mask 0.0.0.0,
// route up), the one with the lowest metric first. A line that cannot be
// understood is skipped.
func ParseDefaultRoutes(r io.Reader) ([]string, error) {
	type route struct {
		iface  string
		metric int
	}
	var routes []route
	sc := bufio.NewScanner(r)
	for sc.Scan() {
		f := strings.Fields(sc.Text())
		// Iface Destination Gateway Flags RefCnt Use Metric Mask ...
		if len(f) < 8 || f[0] == "Iface" {
			continue
		}
		if f[1] != "00000000" || f[7] != "00000000" {
			continue
		}
		flags, err := strconv.ParseUint(f[3], 16, 32)
		if err != nil || flags&0x1 == 0 { // RTF_UP
			continue
		}
		metric, err := strconv.Atoi(f[6])
		if err != nil {
			continue
		}
		routes = append(routes, route{f[0], metric})
	}
	if err := sc.Err(); err != nil {
		return nil, errors.NewErr("PhoneRouteTableUnreadable", err)
	}
	sort.SliceStable(routes, func(i, j int) bool { return routes[i].metric < routes[j].metric })
	var names []string
	seen := map[string]bool{}
	for _, rt := range routes {
		if !seen[rt.iface] {
			seen[rt.iface] = true
			names = append(names, rt.iface)
		}
	}
	return names, nil
}

// ChooseLAN picks the address the phone should connect to (SPEC §10.2): the
// first IPv4 address, private (10/8, 172.16/12, 192.168/16), of an interface
// that is up and not loopback. The interface named preferred (the setting
// phone.interface, empty for automatic) is tried first when it has such an
// address, then the interfaces of the default routes in route (the content of
// /proc/net/route, may be nil) from the lowest metric, then the other
// interfaces in the order given. A route table that cannot be read is ignored.
// With no such address the error has the tag PhoneNoNetwork.
func ChooseLAN(route io.Reader, ifaces []Interface, preferred string) (LANAddr, error) {
	var order []string
	if preferred != "" {
		order = append(order, preferred)
	}
	if route != nil {
		if names, err := ParseDefaultRoutes(route); err == nil {
			order = append(order, names...)
		}
	}
	for _, ifc := range ifaces {
		order = append(order, ifc.Name)
	}
	byName := make(map[string]Interface, len(ifaces))
	for _, ifc := range ifaces {
		if _, dup := byName[ifc.Name]; !dup {
			byName[ifc.Name] = ifc
		}
	}
	for _, name := range order {
		ifc, ok := byName[name]
		if !ok || !ifc.Up || ifc.Loopback {
			continue
		}
		for _, ip := range ifc.Addrs {
			if v4 := ip.To4(); v4 != nil && v4.IsPrivate() {
				return LANAddr{IP: v4, Interface: ifc.Name}, nil
			}
		}
	}
	return LANAddr{}, errors.New("PhoneNoNetwork", "this computer has no private IPv4 address on an interface that is up", "preferred", preferred)
}

// ListInterfaces returns the network interfaces of this computer.
func ListInterfaces() ([]Interface, error) { return listInterfaces(net.Interfaces) }

// listInterfaces is ListInterfaces with the system call passed in, so that a
// test can make it fail.
func listInterfaces(system func() ([]net.Interface, error)) ([]Interface, error) {
	nis, err := system()
	if err != nil {
		return nil, errors.NewErr("PhoneInterfacesFailed", err)
	}
	out := make([]Interface, 0, len(nis))
	for _, ni := range nis {
		ifc := Interface{
			Name:     ni.Name,
			Up:       ni.Flags&net.FlagUp != 0,
			Loopback: ni.Flags&net.FlagLoopback != 0,
		}
		// An interface whose addresses cannot be read has none to offer.
		if addrs, err := ni.Addrs(); err == nil {
			for _, a := range addrs {
				switch v := a.(type) {
				case *net.IPNet:
					ifc.Addrs = append(ifc.Addrs, v.IP)
				case *net.IPAddr:
					ifc.Addrs = append(ifc.Addrs, v.IP)
				}
			}
		}
		out = append(out, ifc)
	}
	return out, nil
}

// DetectLAN chooses the LAN address of this computer: ChooseLAN with the real
// route table (/proc/net/route, ignored when it cannot be opened) and the real
// interfaces. preferred is the setting phone.interface.
func DetectLAN(ctx context.Context, preferred string) (LANAddr, error) {
	if err := ctx.Err(); err != nil {
		return LANAddr{}, errors.NewErr("PhoneDetectCanceled", err)
	}
	ifaces, err := ListInterfaces()
	if err != nil {
		return LANAddr{}, err
	}
	var route io.Reader
	if f, err := os.Open(RouteTablePath); err == nil {
		defer func() { _ = f.Close() }()
		route = f
	}
	return ChooseLAN(route, ifaces, preferred)
}
