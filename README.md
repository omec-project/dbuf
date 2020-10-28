# dbuf
Downlink buffering agent for UP4 architecture

## Running DBUF

```bash
go run cmd/dbuf/main.go
```

Options:

```
-data_plane_urls string
    Comma-separated list of URLs for server to listen for data plane packets (default "0.0.0.0:2152")
-external_dbuf_url string
    URL for server to listen to for external calls from SDN controller, etc (default "0.0.0.0:10000")
-max_packet_slots_per_queue uint
    Maximum number of packet slots in each queue (default 1024)
-max_queues uint
    Maximum number of queues to allocate (default 4)
-metrics_url string
    URL for metrics server to listen for requests (default "0.0.0.0:8080")
-queue_drop_timeout duration
    Packets of a queue are dropped when no new packets are received within the timeout (default 5s)
-rx_queue_depth int
    Depth of the rx queue for data plane packets (default 100)
-rx_workers uint
    Number of rx worker goroutines (default 1)
```

TODOs:
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
