package main

import (
	"fmt"
	"github.com/vishvananda/netlink"
	"math/rand/v2"
	"net"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"time"
)

// This is a simple program that allows for redundancy testing on virtualized networks
// There is nothing stopping it from working on physical networks, however it would certainly create a mess of cabling
// It must always be run as root (to be able to bounce interfaces)

const (
	ansiInterface = "\033[38;5;4m"
	ansiSubnet    = "\033[38;5;3m"
	ansiReset     = "\033[0m"
)

func main() {
	// 1. check environment variable for custom bounce time
	var bounceSeconds time.Duration
	if bounceSecondsString, present := os.LookupEnv("NRT_BOUNCE_SEC"); present {
		bounceSecondsInt, _ := strconv.Atoi(bounceSecondsString)
		bounceSeconds = time.Duration(bounceSecondsInt) * time.Second
	} else {
		bounceSeconds = 20 * time.Second // default to 20-second bounce time
	}

	// 2. get all interfaces and their IPs
	subnetMap := getSubnetsInterfaces()

	// 3. bring all interfaces back up when the program exits
	// create a channel for handling interrupt signals
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt)

	// create a slice of all interfaces that may need brought back up
	var totalIfaceSlice []string
	for _, ifaceSlice := range subnetMap {
		for _, iface := range ifaceSlice {
			totalIfaceSlice = append(totalIfaceSlice, iface)
		}
	}
	// use a goroutine to listen for interrupt signals and reset interfaces
	go func() {
		<-signals // listen for interrupt signal
		fmt.Println("\nExiting and resetting interfaces...")
		bounceInterfaces(totalIfaceSlice, 0) // bring all interfaces back up
		os.Exit(0)                           // exit the program
	}()

	// 4. loop indefinitely, selecting random interfaces on the same subnet to bounce
	for {
		// 4a. iterate over subnetMap (taking advantage of the unordered nature of maps for randomness) to determine which interfaces to bounce
		for subnet, ifaceSlice := range subnetMap {

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

			// 4b. bounce each target interface to cause IP SLA reachability failure
			fmt.Println("Bouncing", ansiInterface+strings.Join(ifaceSlice, ", ")+ansiReset, "in subnet", ansiSubnet+subnet+ansiReset, "with", bounceSeconds, "of downtime...")
			bounceInterfaces(targetIfaceSlice, bounceSeconds)

			// determine whether to reset the progress on subnets (25% chance)
			if rand.IntN(4) == 0 {
				break
			}
		}
	}
}

// getInterfacesIPs returns a map of all subnets to slices of their associated interfaces
func getSubnetsInterfaces() map[string][]string {
	// get all interfaces
	interfaces, _ := net.Interfaces()

	var subnetInterfacesMap = make(map[string][]string) // track which interfaces belong to each subnet
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
					// add interface to its subnet in the map
					subnetInterfacesMap[subnet.String()] = append(subnetInterfacesMap[subnet.String()], iface.Name)
				}
			}
		}
	}
	return subnetInterfacesMap
}

// bounceInterfaceGO bounces the given interfaces and leaves them down for a specified amount of time
func bounceInterfaces(ifaceSlice []string, bounceSeconds time.Duration) {
	// bring each interface down (if bouncing and not just resetting)
	if bounceSeconds > 0 {
		for _, ifaceName := range ifaceSlice {
			iface, _ := netlink.LinkByName(ifaceName)
			err := netlink.LinkSetDown(iface)
			if err != nil {
				panic(err) // an error here likely indicates a need for privilege escalation
			}
		}

		// wait for the specified amount of time
		time.Sleep(bounceSeconds)
	}

	// bring each interface back up
	for _, ifaceName := range ifaceSlice {
		iface, _ := netlink.LinkByName(ifaceName)
		netlink.LinkSetUp(iface)
	}
}
