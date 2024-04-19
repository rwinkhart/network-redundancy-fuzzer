//go:build gateway

package main

import (
	"github.com/vishvananda/netlink"
	"net"
)

// setGateway sets the default gateway to the given IP address
// this is probably not needed, since only the directly connected device on the other side of each interface will be pinged
func setGateway(gatewayIP string) {
	// create a new default route
	gatewayRoute := netlink.Route{
		Dst: nil, // default route
		Gw:  net.ParseIP(gatewayIP),
	}

	// check if a default route already exists
	// if it does, remove it
	existingRoutes, _ := netlink.RouteList(nil, netlink.FAMILY_V4)
	for _, route := range existingRoutes {
		if route.Dst == nil {
			netlink.RouteDel(&route)
			break // for efficiency, assume only one default route exists
		}
	}

	// add the new default route
	err := netlink.RouteAdd(&gatewayRoute)
	if err != nil {
		panic(err) // will error if not run as root or if gatewayIP is not reachable
	}
}
