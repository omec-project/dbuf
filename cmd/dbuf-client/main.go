package main

import (
	"context"
	"encoding/binary"
	"flag"
	"fmt"
	"github.com/golang/glog"
	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"github.com/omec-project/dbuf"
	"google.golang.org/grpc"
	"io"
	"math/rand"
	"net"
	"time"
)

var (
	url = flag.String(
		"url", "localhost:10000", "URL of DBUF server to connect to.",
	)
	remoteDataplaneUrl = flag.String(
		"remote_dataplane_url", "localhost:2152", "Dataplane URL of DBUF server to connect to.",
	)
	localDataplaneUrl = flag.String(
		"local_dataplane_url", "localhost:", "Local dataplane url to send packets from.",
	)
	getCurrentState         = flag.Bool("get_current_state", false, "Get the current DBUF state.")
	releasePackets          = flag.Uint64("release_queue", 0, "Release packets from queue.")
	getQueueState           = flag.Uint64("get_queue_state", 0, "Get the state of a DBUF queue.")
	sendPacket              = flag.Uint64("send_packet", 0, "Send a packet to DBUF.")
	subscribe               = flag.Bool("subscribe", false, "Subscribe to Notifications.")
	demo                    = flag.Bool("demo", false, "Run a demo of most functions.")
	dataplanePayloadCounter = uint64(1)
)

type dbufClient struct {
	dbuf.DbufServiceClient
	conn          *grpc.ClientConn
	client        dbuf.DbufServiceClient
	dataplaneConn *net.UDPConn
}

func newDbufClient() *dbufClient {
	return &dbufClient{}
}

func (c *dbufClient) Start() (err error) {
	c.conn, err = grpc.Dial(
		*url, grpc.WithInsecure(), grpc.WithBlock(), grpc.WithTimeout(time.Second),
	)
	if err != nil {
		return fmt.Errorf("error while dialing \"%v\": %w", *url, err)
	}
	c.DbufServiceClient = dbuf.NewDbufServiceClient(c.conn)

	// Dataplane setup
	raddr, err := net.ResolveUDPAddr("udp", *remoteDataplaneUrl)
	if err != nil {
		return fmt.Errorf("could not resolve remote dataplane url %v: %w", *remoteDataplaneUrl, err)
	}
	laddr, err := net.ResolveUDPAddr("udp", *localDataplaneUrl)
	if err != nil {
		return fmt.Errorf("could not resolve local dataplane url %v: %w", *localDataplaneUrl, err)
	}
	c.dataplaneConn, err = net.DialUDP("udp", laddr, raddr)
	if err != nil {
		return fmt.Errorf("could not dial udp: %w", err)
	}
	go readDataplane(c.dataplaneConn)

	return
}

func (c *dbufClient) Shutdown() (err error) {
	c.conn.Close()
	c.dataplaneConn.Close()

	return
}

func (c *dbufClient) SetupSubscription() (err error) {
	stream, err := c.Subscribe(context.Background(), &dbuf.SubscribeRequest{})
	if err != nil {
		glog.Fatalln("Subscribe error: ", err)
	}
	n, err := stream.Recv()
	if err != nil {
		glog.Fatalln("Recv error: ", err)
	}
	if ready := n.GetReady(); ready == nil {
		glog.Fatal("Server did not respond with ready")
	}
	glog.Info("Subscribed to notifications")
	go readNotifications(stream)

	return
}

func (c *dbufClient) Demo() (err error) {
	if err := c.SetupSubscription(); err != nil {
		glog.Fatal(err)
	}

	s, err := c.GetDbufState(context.Background(), &dbuf.GetDbufStateRequest{})
	if err != nil {
		glog.Fatal(err)
	}
	glog.Info(s)
	maxQueueId := uint32(s.QueueIdHigh)
	minQueueId := uint32(s.QueueIdLow)
	randomId := func() uint32 {
		return (rand.Uint32() % (maxQueueId - minQueueId)) + minQueueId
	}

	for i := uint32(s.QueueIdLow); i < 10; i++ {
		if err = doSendPacket(c.dataplaneConn, i%maxQueueId); err != nil {
			glog.Fatal(err)
		}
		time.Sleep(time.Millisecond * 50)
	}
	for i := uint32(s.QueueIdLow); i < 10; i++ {
		if err = c.doReleasePackets(i % maxQueueId); err != nil {
			glog.Infoln(err)
		}
	}

	go func() {
		for true {
			if err = doSendPacket(c.dataplaneConn, randomId()); err != nil {
				glog.Fatal(err)
			}
			time.Sleep(time.Microsecond * 100)
		}
	}()

	for true {
		if err = c.doReleasePackets(randomId()); err != nil {
			glog.Fatal(err)
		}
		time.Sleep(time.Millisecond * 200)
	}

	return
}

func (c *dbufClient) doReleasePackets(queueId uint32) (err error) {
	_, err = c.ModifyQueue(
		context.Background(), &dbuf.QueueOperationRequest{
			Action: dbuf.QueueOperationRequest_QUEUE_ACTION_RELEASE, BufferId: uint64(queueId),
		},
	)
	if err != nil {
		return
	}
	glog.Info("Released packets of queue ", queueId)
	return
}

func readNotifications(stream dbuf.DbufService_SubscribeClient) error {
	for {
		notification, err := stream.Recv()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		glog.Infof("Notification %v", notification)
	}
}

func readDataplane(conn *net.UDPConn) error {
	for {
		buf := make([]byte, 2048)
		n, raddr, err := conn.ReadFromUDP(buf)
		if err != nil {
			return err
		}
		buf = buf[:n]
		glog.Infof("Recv %v bytes from %v: %v", n, raddr, buf)
	}
}

func doSendPacket(conn *net.UDPConn, queueId uint32) (err error) {
	payload := make([]byte, 8)
	binary.BigEndian.PutUint64(payload, dataplanePayloadCounter)
	dataplanePayloadCounter++
	buf := gopacket.NewSerializeBuffer()
	opts := gopacket.SerializeOptions{}
	err = gopacket.SerializeLayers(
		buf, opts,
		&layers.GTPv1U{
			Version:       1,
			ProtocolType:  1,
			MessageType:   255,
			MessageLength: uint16(len(payload)),
			TEID:          queueId,
		},
		gopacket.Payload(payload),
	)
	if err != nil {
		glog.Infof("SerializeLayers error %v", err)
		return
	}
	packetData := buf.Bytes()

	n, err := conn.Write(packetData)
	if err != nil {
		glog.Fatalf("Write error: %v", err)
	}
	glog.Infof("Sent %v bytes: %v", n, packetData)

	return
}

func main() {
	//log.SetFlags(log.Ldate | log.Ltime | log.Lmicroseconds | log.Lshortfile)
	flag.Parse()
	flag.Lookup("logtostderr").Value.Set("true")
	flag.Lookup("stderrthreshold").Value.Set("INFO")

	client := newDbufClient()
	if err := client.Start(); err != nil {
		glog.Fatalf("Could not start client: %v", err)
	}
	defer client.Shutdown()

	if *subscribe {
		if err := client.SetupSubscription(); err != nil {
			glog.Fatal(err)
		}
	}

	if *getCurrentState {
		state, err := client.GetDbufState(context.Background(), &dbuf.GetDbufStateRequest{})
		if err != nil {
			glog.Fatalln("GetCurrentState error: ", err)
		}
		glog.Info(state)
	} else if *getQueueState > 0 {
		state, err := client.GetQueueState(
			context.Background(), &dbuf.GetQueueStateRequest{QueueId: *getQueueState},
		)
		if err != nil {
			glog.Fatalln("GetQueueState error: ", err)
		}
		glog.Info(state)
	} else if *releasePackets > 0 {
		_, err := client.ModifyQueue(
			context.Background(), &dbuf.QueueOperationRequest{BufferId: *releasePackets},
		)
		if err != nil {
			glog.Fatalln("ReleasePackets error: ", err)
		}
		glog.Info("Released packets of queue ", *releasePackets)
	} else if *sendPacket > 0 {
		doSendPacket(client.dataplaneConn, uint32(*sendPacket))
	} else if *demo {
		client.Demo()
	} else {
		glog.Fatal("No command given.")
	}

	if *subscribe {
		time.Sleep(time.Second)
	}

	glog.Flush()
}
