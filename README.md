# Network Redundancy Fuzzer (NRF)
NRF is a simple utility that randomly brings down interfaces/links on the host device.

It is meant to be used in conjunction with IP SLA on network devices to detect when the connection to the server running NRF is lost and interpret this as a signal to simulate device/link failure. This creates a central point for managing redundancy tests throughout a network.

It will only bring down multiple interfaces at once if they exist in the same subnet. This is to allow the user to specify groups of devices to fail at the same time.

Subnet groups will not always fail in their entirety: when bringing down interfaces for a subnet, each interface belonging to the specified subnet has a 50% chance of being included in the downed group.

# Usage
## Network devices
Each device participating in redundancy fuzzing must have a link back to the server running NRF. Assign IPv4 addresses to these links. Use the same subnet for links that are expected to occasionally fail in unison.

Set up IP SLA to send icmp-echo requests to the server running NRF. The network devices should be configured to disable links if the server is unreachable, thus simulating failure.

## Server (running NRF)
Set a static IPv4 address (on the correct subnet) for each interface leading to a network device.

>Please note that only the first IP address present on each interface will be used.

Once your interfaces are set up, simply run `nrf` with root privileges.

NRF must always be privileged as it must have access to alter link states.

>Optionally, a custom downtime (in seconds) on interface bounces can be set using the `NRF_BOUNCE_SEC` environment variable.
>>For example: `NRF_BOUNCE_SEC=10 nrf` will keep all bounced links down for 10 seconds before restoring them (default 20 seconds).
