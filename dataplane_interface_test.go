package dbuf

import (
	"bytes"
	"net"
	"testing"
	"time"
)

func recvUdpPacketOrDie(t *testing.T, serverConn *net.UDPConn) udpPacket {
	buf := make([]byte, 2048)
	if err := serverConn.SetReadDeadline(time.Now().Add(time.Millisecond * 500)); err != nil {
		t.Fatal("SetReadDeadline: ", err)
	}
	n, raddr, err := serverConn.ReadFromUDP(buf)
	if err != nil {
		t.Fatalf("ReadFromUdp failed: %v", err)
	}
	buf = buf[:n]
	revcPkt := udpPacket{
		payload: buf, remoteAddress: *raddr,
	}

	return revcPkt
}

func sendUdpPacketOrDie(t *testing.T, serverConn *net.UDPConn, pkt udpPacket) {
	if err := serverConn.SetWriteDeadline(time.Now().Add(time.Millisecond * 500)); err != nil {
		t.Fatal("SetWriteDeadline: ", err)
	}
	if _, err := serverConn.WriteToUDP(pkt.payload, &pkt.remoteAddress); err != nil {
		t.Fatal("WriteToUDP", err)
	}
}

// Basic remote UDP test server setup on a random localhost port.
func setupTestServerOrDie(t *testing.T) (*net.UDPAddr, *net.UDPConn) {
	testServerUrl := "localhost:0"
	serverAddr, err := net.ResolveUDPAddr("udp", testServerUrl)
	if err != nil {
		t.Fatal("ResolveUDPAddr", err)
	}
	serverConn, err := net.ListenUDP("udp", serverAddr)
	if err != nil {
		t.Fatal("ListenUDP", err)
	}
	serverAddr, ok := serverConn.LocalAddr().(*net.UDPAddr)
	if !ok {
		t.Fatal("Type assertion failed")
	}

	return serverAddr, serverConn
}

func TestDataPlaneInterface(t *testing.T) {
	testServerAddr, testServerConn := setupTestServerOrDie(t)

	di := NewDataPlaneInterface()
	if err := di.Start("localhost:0"); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	diInterfaceAddr, ok := di.udpConn.LocalAddr().(*net.UDPAddr)
	if !ok {
		t.Fatal("Type assertion failed")
	}

	t.Run("Send", func(t *testing.T) {
		pkt := udpPacket{
			payload:       []byte{1, 2, 3},
			remoteAddress: *testServerAddr,
		}
		if err := di.Send(pkt); err != nil {
			t.Fatalf("Send failed: %v", err)
		}
		recvPkt := recvUdpPacketOrDie(t, testServerConn)
		if bytes.Compare(pkt.payload, recvPkt.payload) != 0 {
			t.Fatalf("Packet payload does not match: "+
				"expected: %v"+
				"got: %v", pkt.payload, recvPkt.payload)
		}
		if recvPkt.remoteAddress.String() != diInterfaceAddr.String() {
			t.Fatalf("Packet remote address does not match: "+
				"expected: %v"+
				"got: %v", pkt.remoteAddress, recvPkt.remoteAddress)
		}
	})

	t.Run("Receive", func(t *testing.T) {
		ch := make(chan udpPacket)
		di.SetOutputChannel(ch)
		pkt := udpPacket{
			payload:       []byte{1, 2, 3},
			remoteAddress: *diInterfaceAddr,
		}
		sendUdpPacketOrDie(t, testServerConn, pkt)
		var recvPkt udpPacket
		select {
		case recvPkt = <-ch:
		case <-time.After(time.Millisecond * 500):
			t.Fatal("Packet not received")
		}
		if bytes.Compare(pkt.payload, recvPkt.payload) != 0 {
			t.Fatalf("Packet payload does not match:\n"+
				"expected: %v\n"+
				"got: %v", pkt.payload, recvPkt.payload)
		}
		if recvPkt.remoteAddress.String() != testServerAddr.String() {
			t.Fatalf("Packet remote address does not match:\n"+
				"expected: %v\n"+
				"got: %v", pkt.remoteAddress, recvPkt.remoteAddress)
		}
	})

	di.Stop()
}
