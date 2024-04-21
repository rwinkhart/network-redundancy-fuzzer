# Network Redundancy Fuzzer (NRF)
NRF is a simple utility that creates and removes routes from the routing table to simulate randomized link failure.

It is meant to be used in conjunction with IP SLA on network devices to detect when the connection to the server running NRF is lost and interpret this as a signal to simulate device/link failure. This creates a central point for managing redundancy tests throughout a network.

It will only bring down multiple interfaces at once if they exist in the same subnet. This is to allow the user to specify groups of devices to fail at the same time.

Subnet groups will not always fail in their entirety: when bringing down interfaces for a subnet, each interface belonging to the specified subnet has a 50% chance of being included in the downed group.

# Usage
## Server (running NRF)
For each interface leading to a participating device, assign a static IPv4 address using the first available IP address on the subnet. 

**IMPORTANT:** If putting multiple interfaces on the same subnet, assign the **SAME** IP address to each interface (again, the **first available**). Subsequent addresses are reserved for participating devices.
>Please note that only the first IP address present on each interface will be used.

Once your interfaces are set up, simply run `nrf` with root privileges.
>NRF must always be privileged as it must have access to alter link states and the routing table.

Optionally, a custom downtime (in seconds) on interface bounces can be set using the `NRF_BOUNCE_SEC` environment variable.
>For example: `NRF_BOUNCE_SEC=10 nrf` will keep all bounced links down for 10 seconds before restoring them (default 20 seconds).

If you find yourself needing to disable NRF and want to ensure all your interfaces are up with working routes (so IP SLA doesn't fail), run `nrf --routes`.

## Network devices
Each device participating in redundancy fuzzing must have a link back to the server running NRF. View the server's routing table (`ip route show`) after running `nrf --routes` to see the IP addresses you must use.

Set up IP SLA to send icmp-echo requests to the server running NRF. The network devices should be configured to disable links if the server is unreachable, thus simulating failure.