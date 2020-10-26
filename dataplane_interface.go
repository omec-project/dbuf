package dbuf

import (
	"errors"
	"log"
	"net"
	"os"
	"strings"
	"sync"
	"time"
)

type udpPacket struct {
	payload       []byte
	remoteAddress net.UDPAddr
}

type DataPlaneInterfaceInterface interface {
	Start(string) error
	Stop()
	SetOutputChannel(chan udpPacket)
	Send(udpPacket) error
}

type dataPlaneInterface struct {
	outputChannel chan udpPacket
	udpConn       *net.UDPConn
	channelLock   sync.RWMutex // TODO(max): embed directly?
}

func NewDataPlaneInterface() *dataPlaneInterface {
	d := &dataPlaneInterface{}
	return d
}

func (d *dataPlaneInterface) Start(listenUrls string) error {
	urls := strings.Split(listenUrls, ",")
	// TODO: support multiple interfaces/urls
	//for _, url := range urls {
	//}
	url := urls[0]

	laddr, err := net.ResolveUDPAddr("udp", url)
	if err != nil {
		return err
	}
	d.udpConn, err = net.ListenUDP("udp", laddr)
	if err != nil {
		return err
	}

	go d.receiveFn()

	return nil
}

func (d *dataPlaneInterface) Stop() {
	log.Println("DataplaneListener stopping")
	d.udpConn.Close()

	log.Println("DataplaneListener stopped")
}

func (d *dataPlaneInterface) Send(packet udpPacket) (err error) {
	if err = d.udpConn.SetWriteDeadline(time.Now().Add(time.Second * 1)); err != nil {
		return
	}
	_, err = d.udpConn.WriteToUDP(packet.payload, &packet.remoteAddress)
	if err != nil {
		incTxDrop(1)
		return err
	}
	incTxOk(1)
	return
}

func (d *dataPlaneInterface) SetOutputChannel(ch chan udpPacket) {
	d.channelLock.Lock()
	defer d.channelLock.Unlock()
	d.outputChannel = ch
}

func (d *dataPlaneInterface) receiveFn() {
	for true {
		buf := make([]byte, 2048)
		if err := d.udpConn.SetReadDeadline(time.Now().Add(time.Second * 1)); err != nil {
			return
		}
		n, raddr, err := d.udpConn.ReadFromUDP(buf)
		if errors.Is(err, os.ErrDeadlineExceeded) {
			continue
		} else if err != nil && strings.Contains(err.Error(), "use of closed network connection") {
			log.Println("Listen conn closed")
			break
		} else if err != nil {
			log.Fatalf("%v", err)
		}
		incRxOk(1)
		buf = buf[:n]
		//log.Printf("Recv %v bytes from %v: %v", n, raddr, buf)
		//log.Printf("Recv %v bytes from %v", n, raddr)
		p := udpPacket{
			payload: buf, remoteAddress: *raddr,
		}
		d.channelLock.RLock()
		select {
		case d.outputChannel <- p:
		default:
			incRxDrop(1)
			log.Println("Dropped packet because channel is full")
		}
		d.channelLock.RUnlock()
	}
}
