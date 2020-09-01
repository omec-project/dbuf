package main

import (
	"context"
	"encoding/binary"
	"flag"
	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"github.com/omec-project/dbuf"
	"google.golang.org/grpc"
	"io"
	"log"
	"net"
	"time"
)

var (
	url                     = flag.String("url", "localhost:10000", "URL of DBUF server to connect to.")
	dataplaneUrl            = flag.String("dataplane_url", "localhost:2152", "Dataplane URL of DBUF server to connect to.")
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

//func newDbufClient(dbufServiceClient dbuf.DbufServiceClient) *dbufClient {
//	return &dbufClient{DbufServiceClient: dbufServiceClient}
//}

func newDbufClient() *dbufClient {
	return &dbufClient{}
}

func (c *dbufClient) Start() (err error) {
	c.conn, err = grpc.Dial(*url, grpc.WithInsecure())
	if err != nil {
		return
	}
	c.DbufServiceClient = dbuf.NewDbufServiceClient(c.conn)

	// Dataplane setup
	raddr, err := net.ResolveUDPAddr("udp", *dataplaneUrl)
	if err != nil {
		return
	}
	laddr, err := net.ResolveUDPAddr("udp", "127.0.0.2:2152")
	if err != nil {
		return
	}
	c.dataplaneConn, err = net.DialUDP("udp", laddr, raddr)
	if err != nil {
		return
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
		log.Fatalln("Subscribe error: ", err)
	}
	n, err := stream.Recv()
	if err != nil {
		log.Fatalln("Recv error: ", err)
	}
	if ready := n.GetReady(); ready == nil {
		log.Fatal("Server did not respond with ready")
	}
	log.Print("Subscribed to notifications")
	go readNotifications(stream)

	return
}

func (c *dbufClient) Demo() (err error) {
	var queueId uint32 = 1

	for i := 0; i < 10; i++ {
		doSendPacket(c.dataplaneConn, queueId)
		time.Sleep(time.Millisecond * 50)
	}
	c.doReleasePackets(queueId)

	go func() {
		for true {
			doSendPacket(c.dataplaneConn, queueId)
			time.Sleep(time.Millisecond * 1)
		}
	}()

	for i := 0; i < 10; i++ {
		c.doReleasePackets(queueId)
		time.Sleep(time.Millisecond * 1)
	}

	return
}

func (c *dbufClient) doReleasePackets(queueId uint32) (err error) {
	_, err = c.ReleasePackets(context.Background(), &dbuf.ReleasePacketsRequest{BufferId: uint64(queueId)})
	if err != nil {
		return
	}
	log.Print("Released packets of queue ", *releasePackets)
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
		log.Printf("Notification %v", notification)
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
		log.Printf("Recv %v bytes from %v: %v", n, raddr, buf)
	}
}

func doSendPacket(conn *net.UDPConn, queueId uint32) {
	payload := make([]byte, 8)
	binary.BigEndian.PutUint64(payload, dataplanePayloadCounter)
	dataplanePayloadCounter++
	buf := gopacket.NewSerializeBuffer()
	opts := gopacket.SerializeOptions{}
	err := gopacket.SerializeLayers(buf, opts,
		&layers.GTPv1U{
			Version:       1,
			ProtocolType:  1,
			MessageType:   255,
			MessageLength: uint16(len(payload)),
			TEID:          queueId,
		},
		gopacket.Payload(payload))
	if err != nil {
		log.Printf("SerializeLayers error %v", err)
		return
	}
	packetData := buf.Bytes()

	n, err := conn.Write(packetData)
	if err != nil {
		log.Fatalf("Write error: %v", err)
	}
	log.Printf("Sent %v bytes: %v", n, packetData)
}

func main() {
	log.SetFlags(log.Ldate | log.Ltime | log.Lmicroseconds | log.Lshortfile)
	flag.Parse()

	client := newDbufClient()
	if err := client.Start(); err != nil {
		log.Fatal(err)
	}
	defer client.Shutdown()

	if *subscribe {
		if err := client.SetupSubscription(); err != nil {
			log.Fatal(err)
		}
	}

	if *getCurrentState {
		state, err := client.GetCurrentState(context.Background(), &dbuf.GetCurrentStateRequest{})
		if err != nil {
			log.Fatalln("GetCurrentState error: ", err)
		}
		log.Print(state)
	} else if *getQueueState > 0 {
		state, err := client.GetQueueState(context.Background(), &dbuf.GetQueueStateRequest{QueueId: *getQueueState})
		if err != nil {
			log.Fatalln("GetQueueState error: ", err)
		}
		log.Print(state)
	} else if *releasePackets > 0 {
		_, err := client.ReleasePackets(context.Background(), &dbuf.ReleasePacketsRequest{BufferId: *releasePackets})
		if err != nil {
			log.Fatalln("ReleasePackets error: ", err)
		}
		log.Print("Released packets of queue ", *releasePackets)
	} else if *sendPacket > 0 {
		doSendPacket(client.dataplaneConn, uint32(*sendPacket))
	} else if *demo {
		client.Demo()
	} else {
		log.Fatal("No command given.")
	}

	if *subscribe {
		time.Sleep(time.Second)
	}
}
