package dbuf

import (
	"flag"
	"google.golang.org/grpc"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"
)

var (
	externalDbufUrl = flag.String("external_dbuf_url", "localhost:10000", "URL for server to listen to for external calls from SDN controller, etc")
	dataPlaneUrls   = flag.String("data_plane_urls", "localhost:2152", "Comma-separated list of URLs for server to listen for data plane packets")
)

type Dbuf struct {
	dl         *dataPlaneListener
	bq         *BufferQueue
	grpcServer *grpc.Server
	signals    chan os.Signal
}

func NewDbuf() *Dbuf {
	d := &Dbuf{}
	d.dl = NewDataPlaneListener()
	d.bq = NewBufferQueue(d.dl)
	d.signals = make(chan os.Signal, 1)
	return d
}

func (dbuf *Dbuf) Run() (err error) {
	// Setup signal handler.
	signal.Notify(dbuf.signals, syscall.SIGINT)

	// Start dataplane listener.
	if err = dbuf.dl.Start(*dataPlaneUrls); err != nil {
		return
	}

	// Start buffer queue.
	if err = dbuf.bq.Start(dbuf.dl); err != nil {
		return
	}

	// Create gRPC service.
	lis, err := net.Listen("tcp", *externalDbufUrl)
	if err != nil {
		return
	}
	dbuf.grpcServer = grpc.NewServer()
	RegisterDbufServiceServer(dbuf.grpcServer, newDbufService(dbuf.bq))

	// Install signal handler
	go dbuf.HandleSignals()

	// Blocking
	err = dbuf.grpcServer.Serve(lis)
	if err != nil {
		return
	}

	return
}

func (dbuf *Dbuf) Stop() (err error) {
	dbuf.dl.Stop()
	dbuf.grpcServer.Stop()
	close(dbuf.signals)
	return
}

func (dbuf *Dbuf) HandleSignals() {
	for {
		sig, ok := <-dbuf.signals
		if !ok {
			return
		}
		log.Printf("Got signal %v", sig)
		dbuf.Stop()
	}
}
