// Copyright 2020-present Open Networking Foundation
// SPDX-License-Identifier: LicenseRef-ONF-Member-1.0

package dbuf

import (
	"encoding/binary"
	"flag"
	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	. "github.com/omec-project/dbuf/api"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"log"
	"net"
	"sync"
	"time"
)

var (
	maxQueues           = flag.Uint64("max_queues", 4, "Maximum number of queues to allocate")
	maxPacketQueueSlots = flag.Uint64(
		"max_packet_slots_per_queue", 1024, "Maximum number of packet slots in each queue",
	)
	rxQueueDepth = flag.Int(
		"rx_queue_depth", 100, "Depth of the rx queue for data plane packets "+
			"between the dataplane interface and the queue manager",
	)
	dropTimeout = flag.Duration(
		"queue_drop_timeout", time.Second*5,
		"Packets of a queue are dropped when no new packets are received within the timeout",
	)
	rxWorkers     = flag.Uint("rx_workers", 1, "Number of rx worker goroutines handling dataplane packets")
	notifyTimeout = flag.Duration("notify_timeout", time.Second*3, "Minimum amount of time between consecutive notifications for the same queue")
)

type bufferPacket struct {
	udpPacket
	id uint32
}

type queue struct {
	packets               []bufferPacket
	maximumSlots          uint64
	state                 GetQueueStateResponse_QueuesState
	lock                  sync.Mutex
	dropTimer             *time.Timer
	dropTimeout           time.Duration
	lastFirstNotification int64
	lastDropNotification  int64
}

func NewQueue(maxSlots uint64, timer *time.Timer, dropTimeout time.Duration) *queue {
	q := &queue{}
	q.lock.Lock()
	q.packets = make([]bufferPacket, 0, maxSlots)
	q.maximumSlots = maxSlots
	q.state = GetQueueStateResponse_QUEUE_STATE_BUFFERING
	q.dropTimer = timer
	q.dropTimeout = dropTimeout
	q.lock.Unlock()

	return q
}

func (q *queue) empty() bool {
	q.lock.Lock()
	defer q.lock.Unlock()
	return q.unsafeEmpty()
}

func (q *queue) unsafeEmpty() bool {
	return len(q.packets) == 0
}

func (q *queue) unsafeFull() bool {
	return uint64(len(q.packets)) == q.maximumSlots
}

func (q *queue) unsafeEnsureInvariants() {
	if uint64(len(q.packets)) > q.maximumSlots {
		log.Fatalf(
			"queue invariant violated: len(q.packets) > q.maximumSlots, %v > %v\n",
			len(q.packets), q.maximumSlots,
		)
	}
	if len(q.packets) > 0 && q.state == GetQueueStateResponse_QUEUE_STATE_PASSTHROUGH {
		log.Fatal("queue invariant violated: has packets but is in PASSTHROUGH mode")
	}
}

func (q *queue) enqueuePacket(pkt *bufferPacket) error {
	q.lock.Lock()
	defer q.lock.Unlock()

	if q.unsafeFull() {
		return status.Errorf(codes.ResourceExhausted, "queue is full")
	}
	if q.state == GetQueueStateResponse_QUEUE_STATE_PASSTHROUGH {
		return status.Errorf(codes.FailedPrecondition, "queue is in PASSTHROUGH mode")
	}

	q.packets = append(q.packets, *pkt)
	q.dropTimer.Stop()
	q.dropTimer.Reset(q.dropTimeout)

	return nil
}

func (q *queue) clear() {
	q.lock.Lock()
	defer q.lock.Unlock()
	q.state = GetQueueStateResponse_QUEUE_STATE_BUFFERING
	q.packets = q.packets[:0]
	q.dropTimer.Stop()
	q.dropTimer.Reset(q.dropTimeout)
}

type QueueManagerInterface interface {
	Start() error
	Stop() error
	RegisterSubscriber(chan Notification) error
	UnregisterSubscriber(chan Notification) error
	GetState() GetDbufStateResponse
	GetQueueState(uint64) (GetQueueStateResponse, error)
	ReleasePackets(
		queueId uint32, dst *net.UDPAddr, drop bool, passthrough bool,
	) error
}

type QueueManager struct {
	maxQueues            uint64
	queues               map[uint32]*queue
	ch                   chan udpPacket
	di                   DataPlaneInterfaceInterface
	queueLock            sync.RWMutex
	subscribers          []chan Notification
	subscriberLock       sync.RWMutex
	lastDropNotification int64
}

func NewQueueManager(di DataPlaneInterfaceInterface, numMaxQueues uint64) *QueueManager {
	b := &QueueManager{}
	b.maxQueues = numMaxQueues
	b.di = di
	b.ch = make(chan udpPacket, *rxQueueDepth)
	b.queueLock.Lock()
	b.queues = make(map[uint32]*queue, numMaxQueues)
	b.queueLock.Unlock()

	return b
}

func (b *QueueManager) Start() (err error) {
	b.di.SetOutputChannel(b.ch)
	for i := uint(0); i < *rxWorkers; i++ {
		go b.rxFn()
	}
	return
}

func (b *QueueManager) Stop() (err error) {
	close(b.ch)
	return
}

func (b *QueueManager) GetQueue(queueId uint32) (q *queue, err error) {
	b.queueLock.RLock()
	q, ok := b.queues[queueId]
	b.queueLock.RUnlock()
	if !ok {
		return nil, status.Errorf(codes.NotFound, "queue with id %v does not exist", queueId)
	}
	return q, nil
}

func (b *QueueManager) allocateQueue(queueId uint32) (q *queue, err error) {
	q, err = b.GetQueue(queueId)
	if status.Code(err) != codes.NotFound {
		return nil, status.Errorf(codes.AlreadyExists, "queue with id %v already exists", queueId)
	}
	err = nil
	b.queueLock.RLock()
	numQueues := uint64(len(b.queues))
	b.queueLock.RUnlock()
	if numQueues >= b.maxQueues {
		return nil, status.Errorf(
			codes.ResourceExhausted, "maximum number of queues already allocated",
		)
	}
	q = NewQueue(*maxPacketQueueSlots, time.AfterFunc(
		*dropTimeout, func() {
			if err := b.ReleasePackets(queueId, nil, true, false); err != nil {
				log.Printf("Error droppping packets from queue %v: %v", queueId, err)
			} else {
				log.Printf("Dropped queue %v due to timeout.", queueId)
			}
		}), *dropTimeout)
	b.queueLock.Lock()
	b.queues[queueId] = q
	b.queueLock.Unlock()

	return
}

// TODO(max): Trigger from a timer or similar
func (b *QueueManager) freeQueue(queueId uint32) (err error) {
	_, err = b.GetQueue(queueId)
	if err != nil {
		return
	}

	b.queueLock.Lock()
	delete(b.queues, queueId)
	b.queueLock.Unlock()

	return
}

func (b *QueueManager) getOrAllocateQueue(queueId uint32) (q *queue, err error) {
	q, err = b.GetQueue(queueId)
	if status.Code(err) == codes.NotFound {
		return b.allocateQueue(queueId)
	}
	return
}

func (b *QueueManager) RegisterSubscriber(ch chan Notification) (err error) {
	b.subscriberLock.Lock()
	defer b.subscriberLock.Unlock()
	b.subscribers = append(b.subscribers, ch)
	log.Printf("Registered subscriber %v", ch)
	return
}

func (b *QueueManager) UnregisterSubscriber(ch chan Notification) (err error) {
	b.subscriberLock.Lock()
	defer b.subscriberLock.Unlock()
	for i := range b.subscribers {
		subs := &b.subscribers[i]
		if *subs == ch {
			b.subscribers = append(b.subscribers[:i], b.subscribers[i+1:]...)
			log.Printf("Unregistered subscriber %v", ch)
			return
		}
	}

	return status.Error(codes.NotFound, "channel not found")
}

func (b *QueueManager) GetState() (s GetDbufStateResponse) {
	b.queueLock.RLock()
	defer b.queueLock.RUnlock()
	s.MaximumQueues = uint64(b.maxQueues)
	s.AllocatedQueues = uint64(len(b.queues))
	for _, q := range b.queues {
		if q.empty() {
			s.EmptyQueues += 1
		}
	}
	s.MaximumMemory = 0 // FIXME
	s.FreeMemory = 0    // FIXME

	return s
}

func (b *QueueManager) GetQueueState(queueId uint64) (s GetQueueStateResponse, err error) {
	q, err := b.GetQueue(uint32(queueId))
	if err != nil {
		return
	}
	q.lock.Lock()
	defer q.lock.Unlock()
	s.MaximumBuffers = q.maximumSlots
	s.FreeBuffers = q.maximumSlots - uint64(len(q.packets))
	s.MaximumMemory = 0
	s.FreeMemory = 0
	s.State = q.state
	return
}

// TODO: Handle continuous drain state where we keep forwarding new packets
// Should this function be non-blocking?
// Do we need an explicit DRAIN state?
func (b *QueueManager) ReleasePackets(
	queueId uint32, dst *net.UDPAddr, drop bool, passthrough bool,
) error {
	q, err := b.GetQueue(queueId)
	if err != nil {
		return err
	}
	q.lock.Lock()
	defer q.lock.Unlock()
	q.state = GetQueueStateResponse_QUEUE_STATE_DRAINING
	for i := range q.packets {
		if !drop {
			q.packets[i].udpPacket.remoteAddress = *dst
			if err := b.di.Send(q.packets[i].udpPacket); err != nil {
				return err
			}
		}
		q.packets[i] = bufferPacket{}
	}
	if drop {
		incQueueDrop(int64(len(q.packets)))
	} else {
		incQueueReleased(int64(len(q.packets)))
	}
	// q.clear() ?
	q.packets = q.packets[:0]
	//q.packets = make([]bufferPacket, 0, *maxPacketQueueSlots)
	if passthrough {
		q.state = GetQueueStateResponse_QUEUE_STATE_PASSTHROUGH
	} else {
		q.state = GetQueueStateResponse_QUEUE_STATE_BUFFERING
	}
	q.dropTimer.Stop()
	q.dropTimer.Reset(q.dropTimeout)

	return nil
}

func (b *QueueManager) rxFn() {
	for packet := range b.ch {
		parsedPacket := gopacket.NewPacket(packet.payload, layers.LayerTypeGTPv1U, gopacket.Default)
		if gtpLayer := parsedPacket.Layer(layers.LayerTypeGTPv1U); gtpLayer != nil {
			if ipv4Layer := parsedPacket.Layer(layers.LayerTypeIPv4); ipv4Layer != nil {
				ipv4, _ := ipv4Layer.(*layers.IPv4)
				dst := ipv4.DstIP.To4()
				queueId := binary.BigEndian.Uint32(dst)
				buf := &bufferPacket{packet, queueId}
				b.enqueueBuffer(buf)
			} else {
				log.Printf("Could not parse packet as IPv4: %v", packet)
			}
		} else {
			log.Printf("Could not parse packet as GTP: %v", packet)
			// TODO(max): add counter here and above
		}
	}
	log.Println("receive loop stopped")
}

func (b *QueueManager) notifyDrop(queueId uint32) {
	b.subscriberLock.RLock()
	defer b.subscriberLock.RUnlock()
	for _, ch := range b.subscribers {
		n := Notification{
			MessageType: &Notification_DroppedPacket_{&Notification_DroppedPacket{QueueId: queueId}},
		}
		select {
		case ch <- n:
			log.Print("notifyDrop", n)
		default:
			log.Print("notifyDrop failed, channel full")
		}
	}
}

func (b *QueueManager) notifyFirst(queueId uint32) {
	b.subscriberLock.RLock()
	defer b.subscriberLock.RUnlock()
	for _, ch := range b.subscribers {
		n := Notification{
			MessageType: &Notification_FirstBuffer_{&Notification_FirstBuffer{NewBufferId: queueId}},
		}
		select {
		case ch <- n:
			log.Print("notifyFirst", n)
		default:
			log.Print("notifyFirst failed, channel full")
		}
	}
}

func (b *QueueManager) enqueueBuffer(pkt *bufferPacket) {
	now := time.Now().Unix()
	q, err := b.getOrAllocateQueue(pkt.id)
	if err != nil {
		log.Printf("Dropped packet. No resources for queue %v.", pkt.id)
		if now-b.lastDropNotification >= int64(notifyTimeout.Seconds()) {
			b.notifyDrop(pkt.id)
			b.lastDropNotification = now
		}
		return
	}
	q.lock.Lock()
	defer q.lock.Unlock()
	if !q.unsafeFull() {
		if q.state == GetQueueStateResponse_QUEUE_STATE_PASSTHROUGH {
			_ = b.di.Send(pkt.udpPacket)
			return
		}
		if now-q.lastFirstNotification >= int64(notifyTimeout.Seconds()) {
			b.notifyFirst(pkt.id)
			q.lastFirstNotification = now
		}
		q.packets = append(q.packets, *pkt)
		q.dropTimer.Stop()
		q.dropTimer.Reset(q.dropTimeout)
	} else {
		if now-q.lastDropNotification >= int64(notifyTimeout.Seconds()) {
			b.notifyDrop(pkt.id)
			q.lastDropNotification = now
		}
		incQueueFullDrop(1)
		log.Printf("Dropped packet. Queue with id %v is full.", pkt.id)
	}
}
