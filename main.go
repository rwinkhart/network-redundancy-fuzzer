package main

import (
	"fmt"
	"github.com/vishvananda/netlink"
	"math/big"
	"math/rand/v2"
	"net"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"time"
)

// TODO Current limitations:
// Takes over all non-loopback interfaces - in order to allow the server running NRF to be multi-purpose, add an environment variable to specify excluded interfaces

const (
	ansiInterface = "\033[38;5;4m"
	ansiSubnet    = "\033[38;5;3m"
	ansiReset     = "\033[0m"
)

var subnetInterfacesMap = make(map[string][]map[string]string)

// getSubnetsInterfaces populates the global map (subnetInterfacesMap) of all subnets to slices of their associated interfaces paired with designated IPs for the end devices
func getSubnetsInterfaces() {
	// get all interfaces
	interfaces, _ := net.Interfaces()

	// iterate over all interfaces
	for _, iface := range interfaces {

		// ensure the interface is not a loopback
		if !strings.HasPrefix(iface.Name, "lo") {

			// get the first IP for the interface
			addrs, _ := iface.Addrs()
			if len(addrs) > 0 {
				firstAddr := addrs[0]

				// separate the IP from the subnet mask
				ip, subnet, _ := net.ParseCIDR(firstAddr.String())

				// ensure the IP is an IPv4 address
				if net.ParseIP(ip.String()).To4() != nil {

					// add interface to its subnet in the map (reserve a space for the end-device IP)
					subnetInterfacesMap[subnet.String()] = append(subnetInterfacesMap[subnet.String()], map[string]string{iface.Name: ""})
				}
			}
		}
	}

	// iterate over the subnets and their associated interfaces
	for subnet, ifIPSlice := range subnetInterfacesMap {
		// track last reserved end-device IP
		networkNumber, _, _ := net.ParseCIDR(subnet)
		var lastReservedIP = getNextIP(networkNumber) // increment the IP to be one greater than the network number

		// iterate over the interface to IP maps in the subnet
		for i, ifIPMap := range ifIPSlice {
			for ifaceName := range ifIPMap {
				// set the end-device IP for the interface
				lastReservedIP = getNextIP(lastReservedIP)
				subnetInterfacesMap[subnet][i][ifaceName] = lastReservedIP.String()
			}
		}
	}
}

// getNextIP returns the next IP in the subnet after the provided IP
func getNextIP(ip net.IP) net.IP {
	ipInt := big.NewInt(0)
	ipInt.SetBytes(ip.To4())
	ipInt.Add(ipInt, big.NewInt(1))
	nextIP := net.IP(ipInt.Bytes())
	// TODO ensure nextIP is not a network number, broadcast address, and that it is part of the specified subnet
	return nextIP
}

// setInterfaceRoutes is a wrapper for bounceStaticRoutes that can set routes for all of or a random selection of interfaces in each subnet
// if ensureUp is true, all interfaces will be brought up
// if random is true, random interfaces in each subnet will be selected, rather than all interfaces
// bounceSeconds is the number of seconds to wait before re-instating the static routes (set to 0 for instant reinstatement)
func setInterfaceRoutes(ensureUp bool, random bool, bounceSeconds time.Duration) {
	for subnet, ifIPSlice := range subnetInterfacesMap {
		// reset map of interfaces to their designated IPs for each subnet
		var ifaceNamesIPsMap = make(map[string]string)

		// populate the map with the interfaces and their designated IPs
		for i, ifIPMap := range ifIPSlice {
			for ifaceName := range ifIPMap {

				if ensureUp {
					upInterface(ifaceName) // ensure all interfaces are up
				}

				// add interface to map (or determine whether to do this if random is true)
				if random {
					if rand.IntN(2) == 0 { // 50% chance
						ifaceNamesIPsMap[ifaceName] = ifIPSlice[i][ifaceName]
					}
				} else {
					ifaceNamesIPsMap[ifaceName] = ifIPSlice[i][ifaceName]
				}
			}
		}

		// ensure at least one interface was selected to have its route bounced
		// if none were, no bouncing will occur and the subnet will be skipped
		if len(ifaceNamesIPsMap) > 0 {
			// bounce static routes for all selected interfaces in the subnet
			bounceStaticRoutes(ifaceNamesIPsMap, subnet, bounceSeconds)
		}

		// if random is true, determine whether to break the loop, effectively resetting progress through subnets (25% chance)
		if random && rand.IntN(4) == 0 {
			break
		}
	}
}

// upInterface ensures the given interface/link is up
func upInterface(ifaceName string) {
	iface, _ := netlink.LinkByName(ifaceName)
	err := netlink.LinkSetUp(iface)
	if err != nil {
		panic(err) // will error without privilege escalation, may be first error of this type encountered and thus must be handled
	}
}

// bounceStaticRoutes removes all routes for the given interfaces and then adds static routes (using the provided IPs) after the designated time
// subnet only needs to be specified for logging purposes (and if bounceSeconds > 0)
// this function is not meant to be used directly, but rather through setInterfaceRoutes
func bounceStaticRoutes(ifaceNamesIPsMap map[string]string, subnet string, bounceSeconds time.Duration) {
	var logsPrinted bool // track whether logs have already been printed

	// create a slice of all interfaces being bounced (to be referenced in log to stdout)
	var targetIfaceSlice []string
	for targetIface := range ifaceNamesIPsMap {
		targetIfaceSlice = append(targetIfaceSlice, targetIface)
	}

	// clear pre-existing routes for each interface
	for ifaceName := range ifaceNamesIPsMap {
		// get the interface by name
		iface, _ := netlink.LinkByName(ifaceName)

		// remove all pre-existing routes for the interface
		existingRoutes, _ := netlink.RouteList(iface, netlink.FAMILY_V4)
		for _, route := range existingRoutes {
			if !logsPrinted && bounceSeconds > 0 { // log bounces to stdout if there will be downtime
				fmt.Println("Bouncing", ansiInterface+strings.Join(targetIfaceSlice, ", ")+ansiReset, "in subnet", ansiSubnet+subnet+ansiReset, "with", bounceSeconds, "of downtime...")
				logsPrinted = true
			}
			netlink.RouteDel(&route)
		}
	}

	// wait for the designated amount of time before re-instating the route
	time.Sleep(bounceSeconds)

	// create a new static route for each interface with their designated IPs
	for ifaceName, ip := range ifaceNamesIPsMap {
		// get the interface by name
		iface, _ := netlink.LinkByName(ifaceName)

		// create a new static route for the interface
		staticRoute := netlink.Route{
			LinkIndex: iface.Attrs().Index,
			Dst: &net.IPNet{
				IP:   net.ParseIP(ip),
				Mask: net.CIDRMask(32, 32),
			},
			Scope: netlink.SCOPE_LINK,
		}

		// add the new static route to the routing table
		netlink.RouteAdd(&staticRoute)
	}
}

func main() {
	// 1. check if any flags were provided
	var flag string
	if len(os.Args) > 1 {
		flag = os.Args[1]
	} else {
		flag = ""
	}

	// 2. get all subnets and their associated interfaces (also generate IPs for the opposite end of each interface)
	getSubnetsInterfaces() // populates global subnetInterfacesMap

	// 3. set up initial routes for all interfaces (and ensure the links are up)
	setInterfaceRoutes(true, false, 0)

	if flag == "--routes" {
		os.Exit(0) // exit after routes have been set
	}

	// 4. check environment variable for custom route bounce time
	var bounceSeconds time.Duration
	if bounceSecondsString, present := os.LookupEnv("NRF_BOUNCE_SEC"); present {
		bounceSecondsInt, _ := strconv.Atoi(bounceSecondsString)
		bounceSeconds = time.Duration(bounceSecondsInt) * time.Second
	} else {
		bounceSeconds = 20 * time.Second // default to 20-second bounce time
	}

	// 5. prepare to bring all interfaces back up when the program exits
	// create a channel for handling interrupt signals
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt)
	// use a goroutine to listen for interrupt signals and restore interfaces
	go func() {
		<-signals // listen for interrupt signal
		fmt.Println("\nExiting and leaving interfaces in a functional state...")
		setInterfaceRoutes(false, false, 0) // links should already be up at this point
		os.Exit(0)                          // exit the program
	}()

	// 6. loop indefinitely, selecting random interfaces on the same subnet to bounce routes for
	for {
		setInterfaceRoutes(false, true, bounceSeconds)
	}
}
