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

Run with `-help` for a full list of available flags and descriptions.

### DBUF client simulator

DBUF comes with a simulated client that connects to the API and sends data plane
packets:

```bash
go run cmd/dbuf-client/main.go --demo
```

### Controlplane API

The northbound API is defined in [dbuf.proto](api/dbuf.proto).

## Design

DBUF is:

- Golang microservice with gRPC northbound interface

- Per UE, in memory packet buffer
  - No persistence, inspection, filtering or reordering

- Layer 4 service on Linux networking stack (UDP sockets)
  - GTP/UDP/IP encoding for data plane packets from and to Tofino switches

- Exposing telemetry about system and queue state (e.g. load) via prometheus

### Terminology

#### Packet

A sequence of bytes received on the dataplane interfaces. 

#### Queue

A sequential container storing packets for a single UE. Identified by its ID or the UE's IP address.

#### QueueManager

Manages queues and notifies clients about queue events.   

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
