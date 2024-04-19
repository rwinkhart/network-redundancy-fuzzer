package main

import (
	"github.com/vishvananda/netlink"
	"math/rand/v2"
	"net"
	"strings"
	"sync"
	"time"
)

// This is a simple program that allows for redundancy testing on virtualized networks
// There is nothing stopping it from working on physical networks, however it would certainly create a mess of cabling
// It must always be run as root (to be able to bounce interfaces)

// create a wait group for tracking later concurrent goroutines
var wg sync.WaitGroup

func main() {
	// 1. get all interfaces and their IPs
	interfaceAddrs := getInterfacesAddrs()

	// TODO defer a function that will bring all interfaces back up before exiting

	// 2. create a map of subnets to interfaces contained within the subnets
	var subnetMap = make(map[string][]string)
	for iface, ipMask := range interfaceAddrs {
		_, subnet, _ := net.ParseCIDR(ipMask)
		subnetMap[subnet.String()] = append(subnetMap[subnet.String()], iface)
	}

	// 3. loop indefinitely, selecting random interfaces on the same subnet to bounce
	for {
		// 3a. iterate over subnetMap (taking advantage of the unordered nature of maps for randomness) to determine which interfaces to bounce
		for _, ifaceSlice := range subnetMap {

			// track length of ifaceSlice
			ifaceSliceLength := len(ifaceSlice)

			// track target interfaces
			var targetIfaceSlice []string

			// select a random set of valid interfaces (within the current subnet) to bounce
			for i := range ifaceSliceLength {
				// determine whether to add current interface to targetIfaceSlice (50% chance)
				if rand.IntN(2) == 0 {
					// add the interface to the targetIfaceSlice
					targetIfaceSlice = append(targetIfaceSlice, ifaceSlice[i])
				}
			}

			// ensure at least one interface is selected
			if len(targetIfaceSlice) < 1 {
				targetIfaceSlice = append(targetIfaceSlice, ifaceSlice[rand.IntN(ifaceSliceLength)])
			}

			// 3b. bounce each target interface to cause IP SLA failure, use goroutines to bounce interfaces concurrently
			for _, iface := range targetIfaceSlice {
				wg.Add(1)
				go bounceInterfaceGO(iface, 20*time.Second) // TODO make bounce time configurable via environment variable
			}
			// block execution until all goroutines (bounces) have completed
			wg.Wait()

			// determine whether to reset the progress on subnets (25% chance)
			if rand.IntN(4) == 0 {
				break
			}
		}
	}
}

// getInterfacesIPs returns a map of all non-loopback interfaces to their IPv4 addresses in CIDR notation
func getInterfacesAddrs() map[string]string {
	// get all interfaces
	interfaces, _ := net.Interfaces()

	// get all IPs for each interface
	var interfaceAddrs = make(map[string]string)
	for _, iface := range interfaces {
		// ensure the interface is not a loopback
		if !strings.HasPrefix(iface.Name, "lo") {
			// get all IPs for the interface
			addrs, _ := iface.Addrs()
			for _, addr := range addrs {
				// separate the IP from the subnet mask
				ip, _, _ := net.ParseCIDR(addr.String())
				// ensure the IP is an IPv4 address
				if net.ParseIP(ip.String()).To4() != nil {
					// add the interface and IP to the list
					interfaceAddrs[iface.Name] = addr.String()
				}
			}
		}
	}
	return interfaceAddrs
}

// bounceInterfaceGO bounces the given interface and leaves it down for a specified amount of time
// it is meant to be run as a goroutine
func bounceInterfaceGO(ifaceName string, bounceSeconds time.Duration) {
	defer wg.Done()
	iface, _ := netlink.LinkByName(ifaceName)
	err := netlink.LinkSetDown(iface)
	if err != nil {
		panic(err) // an error here likely indicates a need for privilege escalation
	}
	time.Sleep(bounceSeconds)
	netlink.LinkSetUp(iface)
}
