# dbuf
Downlink buffering agent for UP4 architecture

## Running DBUF

```bash
go run cmd/dbuf/main.go
```

TODOs:
 - Publish docker images somewhere
 - Decide if ReleasePackets should be blocking until drained and if it should lock the queue the whole time (blocking new packets) 
 - Export monitoring statistics
    - Linux interface drop counter
    - Used / free buffers, queues, memory
 - Preallocate memory for packets
