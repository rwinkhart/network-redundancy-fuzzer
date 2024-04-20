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

// This is a simple program that allows for redundancy testing on virtualized networks
// There is nothing stopping it from working on physical networks, however it would certainly create a mess of cabling
// It must always be run as root (to be able to bounce interfaces)

// TODO Current limitations:
// Takes over all non-loopback interfaces and clears all routes for them - in order to allow the server running NRF to be multi-purpose, add an environment variable to specify excluded interfaces

const (
	ansiInterface = "\033[38;5;4m"
	ansiSubnet    = "\033[38;5;3m"
	ansiReset     = "\033[0m"
)

func main() {
	// 1. check environment variable for custom bounce time
	var bounceSeconds time.Duration
	if bounceSecondsString, present := os.LookupEnv("NRF_BOUNCE_SEC"); present {
		bounceSecondsInt, _ := strconv.Atoi(bounceSecondsString)
		bounceSeconds = time.Duration(bounceSecondsInt) * time.Second
	} else {
		bounceSeconds = 20 * time.Second // default to 20-second bounce time
	}

	// 2. get all subnets and their associated interfaces
	subnetMap := getSubnetsInterfaces()

	// 3. bring all interfaces back up when the program exits
	// create a channel for handling interrupt signals
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt)

	// create a slice of all interfaces that may need brought back up
	var totalIfaceSlice []string
	for _, ifIPSlice := range subnetMap {
		for _, ifIPMap := range ifIPSlice {
			for ifaceName := range ifIPMap {
				totalIfaceSlice = append(totalIfaceSlice, ifaceName)
			}
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
		for subnet, ifIPSlice := range subnetMap {

			// track length of ifIPSlice (number of interfaces in the subnet)
			ifIPSliceLength := len(ifIPSlice)

			// track target interfaces
			var targetIfaceSlice []string

			// create a slice of all interfaces in the subnet
			var validInterfaces []string
			for _, ifIPMap := range ifIPSlice {
				for ifaceName := range ifIPMap {
					validInterfaces = append(validInterfaces, ifaceName)
				}
			}

			// select a random set of valid interfaces (within the current subnet) to bounce
			for i := range ifIPSliceLength {
				// determine whether to add current interface to targetIfaceSlice (50% chance)
				if rand.IntN(2) == 0 {
					// add the interface to the targetIfaceSlice
					targetIfaceSlice = append(targetIfaceSlice, validInterfaces[i])
				}
			}

			// ensure at least one interface is selected
			if len(targetIfaceSlice) < 1 {
				targetIfaceSlice = append(targetIfaceSlice, validInterfaces[rand.IntN(ifIPSliceLength)])
			}

			// 4b. bounce each target interface to cause IP SLA reachability failure
			fmt.Println("Bouncing", ansiInterface+strings.Join(targetIfaceSlice, ", ")+ansiReset, "in subnet", ansiSubnet+subnet+ansiReset, "with", bounceSeconds, "of downtime...")
			bounceInterfaces(targetIfaceSlice, bounceSeconds)

			// determine whether to reset the progress on subnets (25% chance)
			if rand.IntN(4) == 0 {
				break
			}
		}
	}
}

// getInterfacesIPs returns a map of all subnets to slices of their associated interfaces paired with designated IPs for the end devices
func getSubnetsInterfaces() map[string][]map[string]string {
	// get all interfaces
	interfaces, _ := net.Interfaces()

	// create a map of subnets to slices of maps of interfaces to end-device IPs
	var subnetInterfacesMap = make(map[string][]map[string]string)

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
				panic(err) // will error without privilege escalation, will be first error of this type encountered and thus is the only one handled
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

// getNextIP returns the next IP in the subnet after the provided IP
func getNextIP(ip net.IP) net.IP {
	ipInt := big.NewInt(0)
	ipInt.SetBytes(ip.To4())
	ipInt.Add(ipInt, big.NewInt(1))
	nextIP := net.IP(ipInt.Bytes())
	// TODO ensure nextIP is not a network number, broadcast address, and that it is part of the specified subnet
	return nextIP
}
