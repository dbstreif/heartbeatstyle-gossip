# heartbeatstyle-gossip

A simple implementation of the heartbeat-style gossip propagation strategy.

## Usage
1. Ensure that the latest version of go is installed `go version`
2. `go run node.go <BIND PORT> <NODE PORT 1> <NODE PORT 2> [BAD NODE ARG]`
3. Run several instances and connect the ports to establish a node network. The *BAD NODE ARG* indicates that the program will run a test failure case. Simply add the `bad` argument to denote the node as a test failure node.
