package dbuf

import (
	"errors"
	"log"
	"net"
	"os"
	"strings"
	"time"
)

type udpPacket struct {
	payload       []byte
	remoteAddress net.UDPAddr
}

type dataPlaneListener struct {
	closeSignal   chan struct{}
	outputChannel chan udpPacket
	udpConn       *net.UDPConn
}

func NewDataPlaneListener() *dataPlaneListener {
	d := &dataPlaneListener{}
	d.closeSignal = make(chan struct{})
	return d
}

func (d *dataPlaneListener) Start(listenUrls string) error {
	urls := strings.Split(listenUrls, ",")
	// TODO: support multiple interfaces/urls
	//for _, url := range urls {
	//}
	url := urls[0]

	//ifs, err := net.Interfaces()
	//if err != nil {
	//	return err
	//}
	//for _, i := range ifs {
	//	log.Printf("%+v", i)
	//	addrs, err := i.Addrs()
	//	if err != nil{
	//		return err
	//	}
	//	for _, addr := range addrs {
	//		log.Println("\t", addr)
	//	}
	//}

	laddr, err := net.ResolveUDPAddr("udp", url)
	if err != nil {
		return err
	}
	d.udpConn, err = net.ListenUDP("udp", laddr)
	if err != nil {
		return err
	}

	go d.ReceiveFn()

	return nil
}

func (d *dataPlaneListener) Stop() {
	log.Println("DataplaneListener stopping")
	select {
	case d.closeSignal <- struct{}{}:
	default:
	}
	log.Println("1")
	close(d.closeSignal)
	log.Println("2")
	d.udpConn.Close()

	log.Println("DataplaneListener stopped")
}

func (d *dataPlaneListener) Send(packet udpPacket) (err error) {
	if err = d.udpConn.SetWriteDeadline(time.Now().Add(time.Second * 1)); err != nil {
		return
	}
	_, err = d.udpConn.WriteToUDP(packet.payload, &packet.remoteAddress)
	if err != nil {
		return err
	}

	return
}

func (d *dataPlaneListener) SetOutputChannel(ch chan udpPacket) {
	d.outputChannel = ch
}

func (d *dataPlaneListener) ReceiveFn() {
	for true {
		buf := make([]byte, 2048)
		n, raddr, err := d.udpConn.ReadFromUDP(buf)
		if errors.Is(err, os.ErrDeadlineExceeded) {
			continue
		} else if err != nil && strings.Contains(err.Error(), "use of closed network connection") {
			log.Println("Listen conn closed")
			break
		} else if err != nil {
			log.Fatalf("%v", err)
		}
		buf = buf[:n]
		log.Printf("Recv %v bytes from %v: %v", n, raddr, buf)
		p := udpPacket{
			payload: buf, remoteAddress: *raddr,
		}
		select {
		case d.outputChannel <- p:
		default:
			log.Println("Dropped packet because channel is full")
		}
	}
}
