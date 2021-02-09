// Copyright 2020-present Open Networking Foundation
// SPDX-License-Identifier: LicenseRef-ONF-Member-1.0

package dbuf

import (
	"encoding/binary"
	"github.com/golang/mock/gomock"
	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	. "github.com/omec-project/dbuf/api"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"net"
	"testing"
	"time"
)

func TestQueue(t *testing.T) {
	queueSize := uint64(10)
	clearTimeout := time.Millisecond * 10
	q := NewQueue(queueSize, time.AfterFunc(time.Second*0, func() {}), clearTimeout)
	pkt := &bufferPacket{}

	if !q.empty() {
		t.Errorf("new queue is not empty")
	}

	t.Run("ClearSuccess", func(t *testing.T) {
		q.packets = append(q.packets, bufferPacket{})
		q.clear()
		if len(q.packets) != 0 {
			t.Error("queue contains packets after clear")
		}
	})

	t.Run("SimpleEnqueueSuccess", func(t *testing.T) {
		q.clear()
		if err := q.enqueuePacket(pkt); err != nil {
			t.Errorf("enqueuePacket failed: %v", err)
		}
		if len(q.packets) != 1 {
			t.Errorf("queue len did not increase after enqueue")
		}
	})

	t.Run("QueueFullFail", func(t *testing.T) {
		q.clear()
		for i := uint64(0); i < queueSize; i++ {
			if err := q.enqueuePacket(pkt); err != nil {
				t.Errorf("enqueuePacket failed: %v", err)
			}
		}
		if !q.unsafeFull() {
			t.Errorf("queue not full after inserting %v packets", queueSize)
		}
		fullErr := q.enqueuePacket(pkt)
		if status.Code(fullErr) != codes.ResourceExhausted {
			t.Errorf("could enqueue more packets than queue capacity")
		}
	})

	t.Run("PassthroughModeFail", func(t *testing.T) {
		q.clear()
		q.state = GetQueueStateResponse_QUEUE_STATE_PASSTHROUGH
		precondErr := q.enqueuePacket(pkt)
		if status.Code(precondErr) != codes.FailedPrecondition {
			t.Errorf("could enqueue into queue in PASSTHROUGH mode")
		}
	})

	t.Run("TimeoutClearTest", func(t *testing.T) {
		var q2 *queue
		q2 = NewQueue(queueSize, time.AfterFunc(clearTimeout, func() {
			q2.clear()
		}), clearTimeout)

		if err := q2.enqueuePacket(pkt); err != nil {
			t.Errorf("enqueuePacket failed: %v", err)
		}
		time.Sleep(clearTimeout + time.Millisecond*5)
		if !q2.empty() {
			t.Error("queue not empty after timeout")
		}
	})
}

func TestBufferQueue(t *testing.T) {
	const queueId1 uint32 = 1
	const numMaxQueues uint64 = 5

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	var udpOutputChannel chan udpPacket
	mockDi := NewMockDataPlaneInterfaceInterface(ctrl)
	mockDi.EXPECT().SetOutputChannel(gomock.Any()).DoAndReturn(func(ch chan udpPacket) error {
		udpOutputChannel = ch
		return nil
	})
	bq := NewQueueManager(mockDi, numMaxQueues)
	if err := bq.Start(); err != nil {
		t.Fatal("failed to start", err)
	}
	if _, err := bq.allocateQueue(queueId1); err != nil {
		t.Fatal("allocateQueue failed", err)
	}
	notifications := make(chan Notification)
	if err := bq.RegisterSubscriber(notifications); err != nil {
		t.Fatal("RegisterSubscriber failed", err)
	}

	t.Run("UnregisterSubscriberNotExistFail", func(t *testing.T) {
		ch := make(chan Notification)
		s := status.Convert(bq.UnregisterSubscriber(ch))
		assertStatusErrorCode(t, s, codes.NotFound)
		assertStatusErrorMessage(t, s, "not found")
	})

	t.Run("GetStateSuccess", func(t *testing.T) {
		s := bq.GetState()
		t.Logf("%+v", s)
		if s.MaximumQueues != numMaxQueues {
			t.Fatal("incorrect MaximumQueues:", s.MaximumQueues)
		}
		s = bq.GetState()
		if s.AllocatedQueues != 1 {
			t.Fatal("incorrect AllocatedQueues:", s.MaximumQueues)
		}
	})

	t.Run("GetQueueStateSuccess", func(t *testing.T) {
		s, err := bq.GetQueueState(uint64(queueId1))
		if err != nil {
			t.Fatal("GetQueueState failed:", err)
		}
		if s.MaximumBuffers != *maxPacketQueueSlots {
			t.Fatal("MaximumBuffers incorrect:", s.MaximumBuffers)
		}
	})

	t.Run("GetOrAllocateQueueSuccess", func(t *testing.T) {
		bq := NewQueueManager(mockDi, numMaxQueues)
		if _, err := bq.getOrAllocateQueue(queueId1); err != nil {
			t.Fatal("getOrAllocateQueue failed for existing queue:", err)
		}
		if _, err := bq.getOrAllocateQueue(queueId1); err != nil {
			t.Fatal("getOrAllocateQueue failed for new queue:", err)
		}
	})

	t.Run("AllocateQueueExistsFail", func(t *testing.T) {
		_, err := bq.allocateQueue(queueId1)
		s := status.Convert(err)
		assertStatusErrorCode(t, s, codes.AlreadyExists)
		assertStatusErrorMessage(t, s, "already exists")
	})

	t.Run("AllocateQueueMaxLimitFail", func(t *testing.T) {
		bq := NewQueueManager(mockDi, numMaxQueues)
		for i := uint32(0); uint64(i) < numMaxQueues; i++ {
			if _, err := bq.allocateQueue(i); err != nil {
				t.Fatal("allocateQueue failed:", err)
			}
		}
		_, err := bq.allocateQueue(uint32(numMaxQueues) + 1)
		s := status.Convert(err)
		assertStatusErrorCode(t, s, codes.ResourceExhausted)
		assertStatusErrorMessage(t, s, "already allocated")
	})

	t.Run("FreeQueueNotExistsFail", func(t *testing.T) {
		s := status.Convert(bq.freeQueue(queueId1 + 1))
		assertStatusErrorCode(t, s, codes.NotFound)
		assertStatusErrorMessage(t, s, "not exist")
	})

	t.Run("rxFn", func(t *testing.T) {
		payload := make([]byte, 8)
		binary.BigEndian.PutUint64(payload, 1234)
		dstIp := make(net.IP, net.IPv4len)
		binary.BigEndian.PutUint32(dstIp, queueId1)
		buf := gopacket.NewSerializeBuffer()
		opts := gopacket.SerializeOptions{}
		err := gopacket.SerializeLayers(
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
			t.Fatalf("SerializeLayers error %v", err)
			return
		}
		packetData := buf.Bytes()
		raddr := net.UDPAddr{
			IP:   net.IPv4(10, 0, 0, 1),
			Port: 2152,
		}
		pkt := udpPacket{
			payload:       packetData,
			remoteAddress: raddr,
		}
		udpOutputChannel <- pkt
		var n Notification
		select {
		case n = <-notifications:
			break
		case <-time.After(time.Millisecond * 100):
			t.Fatal("Did not receive notification")
		}
		first := n.GetFirstBuffer()
		if first == nil {
			t.Fatalf("Wrong type of notification %T", n)
		}
		if first.NewBufferId != queueId1 {
			t.Fatal("FirstBuffer notification contained wrong queue id", first)
		}

		// TODO(max): extend
		mockDi.EXPECT().Send(gomock.Any()).Return(nil)
		if err := bq.ReleasePackets(queueId1, &raddr, false, false); err != nil {
			t.Fatal("ReleasePackets failed:", err)
		}
	})

	// Cleanup
	if err := bq.freeQueue(queueId1); err != nil {
		t.Fatal("freeQueue failed", err)
	}
	if err := bq.UnregisterSubscriber(notifications); err != nil {
		t.Fatal("UnregisterSubscriber failed:", err)
	}
	if err := bq.Stop(); err != nil {
		t.Fatal("failed to stop:", err)
	}
}
