<!--
Copyright 2020-present Open Networking Foundation
SPDX-License-Identifier: LicenseRef-ONF-Member-1.0
-->

# DBUF
A downlink buffering agent for the UP4 architecture.

## Running DBUF

```bash
go run cmd/dbuf/main.go
```

#### Options:

Run with `-help` for a full list of available flags and descriptions.

### Controlplane API

The northbound API is defined in [dbuf.proto](api/dbuf.proto).

## Design

- Golang microservice with gRPC northbound interface

- Per UE, in memory packet buffer
  - No persistence, inspection or reordering

- L4 service on Linux networking stack (UDP sockets)
  - GTP/UDP/IP encoding for dataplane packets from and to Tofino

- Monitoring of system and queue state (e.g. load) via prometheus

### Terminology

#### Queue

A sequential container storing packets for a single UE. Identified by its ID or the UE's IP address.

#### Packet

A sequence of bytes received on the dataplane interfaces. 

#### QueueManager

#### Buffer

## TODOs
 - Delete unused queues after a timeout
 - Maybe introduce a release delay to prevent overloading downstream
 - Use strings for IP addresses when possible, e.g. ModifyQueue message, notifications
 - Expose dataplane addresses over gRPC
 - Expose queue stats in json and prometheus
 - Decide if ReleasePackets should be blocking until drained and if it should lock the queue the whole time (blocking new packets)
 - Export monitoring statistics
    - Linux interface drop counter
    - Used / free buffers, queues, memory
 - Preallocate memory for packets
