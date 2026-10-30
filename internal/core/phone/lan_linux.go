//go:build !windows

package phone

import "os"

// RouteTablePath is the file ParseDefaultRoutes reads on Linux.
const RouteTablePath = "/proc/net/route"

// defaultRouteInterfaces returns the names of the interfaces that have a default
// route, the one with the lowest metric first, from the route table. A table
// that cannot be opened or read gives none.
func defaultRouteInterfaces([]Interface) []string {
	f, err := os.Open(RouteTablePath)
	if err != nil {
		return nil
	}
	defer func() { _ = f.Close() }()
	names, err := ParseDefaultRoutes(f)
	if err != nil {
		return nil
	}
	return names
}
