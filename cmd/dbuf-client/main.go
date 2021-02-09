// Copyright 2020-present Open Networking Foundation
// SPDX-License-Identifier: LicenseRef-ONF-Member-1.0

package main

import (
	"context"
	"encoding/binary"
	"flag"
	"fmt"
	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	dbuf "github.com/omec-project/dbuf/api"
	"github.com/omec-project/dbuf/utils"
	log "github.com/sirupsen/logrus"
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
	logLevel                = utils.NewLogLevelFlagValue(log.InfoLevel)
	logFormat               = utils.NewLogFormatFlagValue(&log.TextFormatter{})
	dataplanePayloadCounter = uint64(1)
)

func init() {
	flag.Var(logFormat, "log_format", "Format of the logs")
	flag.Var(logLevel, "log_level", "Verbosity of the logs")
}

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
		log.Fatalln("Subscribe error: ", err)
	}
	n, err := stream.Recv()
	if err != nil {
		log.Fatalln("Recv error: ", err)
	}
	if ready := n.GetReady(); ready == nil {
		log.Fatal("Server did not respond with ready")
	}
	log.Info("Subscribed to notifications")
	go readNotifications(stream)

	return
}

func (c *dbufClient) Demo() (err error) {
	if err := c.SetupSubscription(); err != nil {
		log.Fatal(err)
	}

	s, err := c.GetDbufState(context.Background(), &dbuf.GetDbufStateRequest{})
	if err != nil {
		log.Fatal(err)
	}
	log.Info(s)
	minQueueId := uint32(1)
	maxQueueId := uint32(s.MaximumQueues + 1)
	randomId := func() uint32 {
		return (rand.Uint32() % (maxQueueId - minQueueId)) + minQueueId
	}

	for i := minQueueId; i < maxQueueId; i++ {
		if err = doSendPacket(c.dataplaneConn, i); err != nil {
			log.Fatal(err)
		}
		time.Sleep(time.Millisecond * 50)
	}
	for i := minQueueId; i < maxQueueId; i++ {
		if err = c.doModifyQueue(i, dbuf.ModifyQueueRequest_QUEUE_ACTION_RELEASE); err != nil {
			log.Infoln(err)
		}
	}

	// Test passthrough mode
	for i := minQueueId; i < maxQueueId; i++ {
		if err = c.doModifyQueue(
			i, dbuf.ModifyQueueRequest_QUEUE_ACTION_RELEASE_AND_PASSTHROUGH,
		); err != nil {
			log.Fatal(err)
		}
	}
	for i := minQueueId; i < maxQueueId; i++ {
		if err = doSendPacket(c.dataplaneConn, i); err != nil {
			log.Fatal(err)
		}
		time.Sleep(time.Millisecond * 50)
	}
	for i := minQueueId; i < maxQueueId; i++ {
		if err = c.doModifyQueue(i, dbuf.ModifyQueueRequest_QUEUE_ACTION_RELEASE); err != nil {
			log.Fatal(err)
		}
	}
	time.Sleep(time.Millisecond * 500)

	go func() {
		for true {
			if err = doSendPacket(c.dataplaneConn, randomId()); err != nil {
				log.Fatal(err)
			}
			time.Sleep(time.Microsecond * 100)
		}
	}()

	for true {
		if err = c.doModifyQueue(
			randomId(), dbuf.ModifyQueueRequest_QUEUE_ACTION_RELEASE,
		); err != nil {
			log.Fatal(err)
		}
		time.Sleep(time.Millisecond * 200)
	}

	return
}

func (c *dbufClient) doModifyQueue(
	queueId uint32, action dbuf.ModifyQueueRequest_QueueAction,
) (err error) {
	_, err = c.ModifyQueue(
		context.Background(),
		&dbuf.ModifyQueueRequest{
			Action: action, QueueId: uint64(queueId),
			DestinationAddress: c.dataplaneConn.LocalAddr().String(),
		},
	)
	if err != nil {
		return
	}
	log.Infof("Modified queue %v to action %v", queueId, action)
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
		log.Infof("Notification %v", notification)
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
		log.Infof("Recv %v bytes from %v: %v", n, raddr, buf)
	}
}

func doSendPacket(conn *net.UDPConn, queueId uint32) (err error) {
	payload := make([]byte, 8)
	binary.BigEndian.PutUint64(payload, dataplanePayloadCounter)
	dataplanePayloadCounter++
	dstIp := make(net.IP, net.IPv4len)
	binary.BigEndian.PutUint32(dstIp, queueId)
	buf := gopacket.NewSerializeBuffer()
	opts := gopacket.SerializeOptions{}
	err = gopacket.SerializeLayers(
		buf, opts,
		&layers.GTPv1U{
			Version:       1,
			ProtocolType:  1,
			MessageType:   255,
			MessageLength: 20 + 8 + uint16(len(payload)),
		},
		&layers.IPv4{
			Version:  4,
			IHL:      5,
			Protocol: layers.IPProtocolUDP,
			Length:   20 + 8 + uint16(len(payload)),
			TTL:      64,
			SrcIP:    net.IP{10, 0, 0, 1},
			DstIP:    dstIp,
		},
		&layers.UDP{
			SrcPort:  0,
			DstPort:  0,
			Length:   8 + uint16(len(payload)),
			Checksum: 0,
		},
		gopacket.Payload(payload),
	)
	if err != nil {
		log.Infof("SerializeLayers error %v", err)
		return
	}
	packetData := buf.Bytes()

	n, err := conn.Write(packetData)
	if err != nil {
		log.Fatalf("Write error: %v", err)
	}
	log.Infof("Sent %v bytes: %v", n, packetData)

	return
}

func main() {
	flag.Parse()
	log.SetReportCaller(true)
	log.SetFormatter(logFormat.GetFormatter())
	log.SetLevel(logLevel.GetLevel())

	client := newDbufClient()
	if err := client.Start(); err != nil {
		log.Fatalf("Could not start client: %v", err)
	}
	defer client.Shutdown()

	if *subscribe {
		if err := client.SetupSubscription(); err != nil {
			log.Fatal(err)
		}
	}

	if *getCurrentState {
		state, err := client.GetDbufState(context.Background(), &dbuf.GetDbufStateRequest{})
		if err != nil {
			log.Fatalln("GetCurrentState error: ", err)
		}
		log.Info(state)
	} else if *getQueueState > 0 {
		state, err := client.GetQueueState(
			context.Background(), &dbuf.GetQueueStateRequest{QueueId: *getQueueState},
		)
		if err != nil {
			log.Fatalln("GetQueueState error: ", err)
		}
		log.Info(state)
	} else if *releasePackets > 0 {
		_, err := client.ModifyQueue(
			context.Background(), &dbuf.ModifyQueueRequest{QueueId: *releasePackets},
		)
		if err != nil {
			log.Fatalln("ReleasePackets error: ", err)
		}
		log.Info("Released packets of queue ", *releasePackets)
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
