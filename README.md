# Network Redundancy Fuzzer (NRF)
NRF is a simple utility that creates and removes routes from the routing table of a Linux server to simulate randomized link failure.

It is meant to be used in conjunction with IP SLA on Cisco devices to detect when the connection to the server running NRF is lost and interpret this as a signal to simulate device/link failure. This creates a central point for managing redundancy tests throughout a network.

It can also be used with some non-Cisco hardware using [nrf-client-emulator](https://github.com/rwinkhart/nrf-client-emulator).

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

On Cisco devices, set up IP SLA to send icmp-echo requests to the server running NRF. The network devices should be configured to disable links if the server is unreachable, thus simulating failure.

On Arista devices, configure [nrf-client-emulator](https://github.com/rwinkhart/nrf-client-emulator).

### Cisco IP SLA Template
Note that the below example checks logs to know when interface status changes.
This is because some Cisco devices do not support the `event track` command.
If your device does support this command, the two lines beginning with `event syslog pattern`
may be replaced with `event track 99 state down` and `event track 99 state up`, respectively.
```
ip sla 99
icmp-echo 10.0.0.1 source-interface <interface facing NRF server>
frequency <interval>

ip sla schedule 99 life forever start-time now
track 99 ip sla 99 reachability

event manager applet DEVICE_FAIL
event syslog pattern "TRACK-6-STATE: 99 ip sla 99 reachability Up -> Down"
action 1 cli command enable
action 2 cli command "conf t"
action 3 cli command "int range <interfaces to shut down>"
action 4 cli command shutdown
action 5 cli command exit
event manager applet DEVICE_RECOVER
event syslog pattern "TRACK-6-STATE: 99 ip sla 99 reachability Down -> Up"
action 1 cli command enable
action 2 cli command "conf t"
action 3 cli command "int range <interfaces to restore>"
action 4 cli command "no shutdown"
action 5 cli command exit
```
