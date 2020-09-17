package dbuf

import (
	"flag"
	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"log"
	"sync"
	"time"
)

var (
	queueIdLow          = flag.Uint("queue_id_low", 0, "Lowest queue ID to buffer")
	queueIdHigh         = flag.Uint("queue_id_high", 4, "Highest queue ID to buffer")
	maxPacketQueueSlots = flag.Uint64(
		"max_packet_slots_per_queue", 1024, "Maximum number of packet slots in each queue",
	)
	rxQueueDepth = flag.Int(
		"rx_queue_depth", 100, "Depth of the rx queue for data plane packets",
	)
	dropTimeout = flag.Duration(
		"queue_drop_timeout", time.Second*5,
		"Packets of a queue are dropped when no new packets are received within the timeout.",
	)
	rxWorkers = flag.Uint("rx_workers", 1, "Number of rx worker goroutines.")
)

type bufferPacket struct {
	udpPacket
	id uint32
}

type queue struct {
	packets      []bufferPacket
	maximumSlots uint64
	state        GetQueueStateResponse_QueuesState
	lock         sync.Mutex
	dropTimer    *time.Timer
}

func (q *queue) empty() bool {
	q.lock.Lock()
	defer q.lock.Unlock()
	return len(q.packets) == 0
}

func (q *queue) unsafeEmpty() bool {
	return len(q.packets) == 0
}

func (q *queue) unsafeFull() bool {
	return uint64(len(q.packets)) == q.maximumSlots
}

type BufferQueue struct {
	queues         []queue
	ch             chan udpPacket
	di             *dataPlaneInterface
	subscribers    []chan Notification
	subscriberLock sync.RWMutex
}

func NewBufferQueue(di *dataPlaneInterface) *BufferQueue {
	b := &BufferQueue{}
	b.di = di
	b.ch = make(chan udpPacket, *rxQueueDepth)
	b.queues = make([]queue, *queueIdHigh-*queueIdLow)
	for i := range b.queues {
		queueId, err := GetQueueIdFromIndex(i) // Prevent capturing i by reference
		if err != nil {
			log.Fatal(err)
		}
		q := &b.queues[i]
		q.lock.Lock()
		q.packets = make([]bufferPacket, 0, *maxPacketQueueSlots)
		q.maximumSlots = *maxPacketQueueSlots
		q.state = GetQueueStateResponse_QUEUE_STATE_BUFFERING
		q.dropTimer = time.AfterFunc(
			*dropTimeout, func() {
				if err := b.ReleasePackets(queueId, true, false); err != nil {
					log.Printf("Error droppping packets from queue %v: %v", queueId, err)
				} else {
					log.Printf("Dropped queue %v due to timeout.", queueId)
				}
			},
		)
		q.lock.Unlock()
	}

	return b
}

func (b *BufferQueue) Start() (err error) {
	b.di.SetOutputChannel(b.ch)
	for i := uint(0); i < *rxWorkers; i++ {
		go b.rxFn()
	}
	return
}

func (b *BufferQueue) Stop() (err error) {
	close(b.ch)
	return
}

func (b *BufferQueue) GetQueue(queueId uint32) (q *queue, err error) {
	if uint(queueId) < *queueIdLow || uint(queueId) >= *queueIdHigh {
		return nil, status.Errorf(codes.OutOfRange, "queue id %v is out of range", queueId)
	}
	index := int(queueId) - int(*queueIdLow)

	return &b.queues[index], nil
}

func (b *BufferQueue) RegisterSubscriber(ch chan Notification) (err error) {
	b.subscriberLock.Lock()
	defer b.subscriberLock.Unlock()
	b.subscribers = append(b.subscribers, ch)
	log.Printf("Registered subscriber %v", ch)
	return
}

func (b *BufferQueue) UnregisterSubscriber(ch chan Notification) (err error) {
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

func (b *BufferQueue) GetState() (s GetDbufStateResponse) {
	s.QueueIdLow = uint64(*queueIdLow)
	s.QueueIdHigh = uint64(*queueIdHigh)
	s.MaximumMemory = 0
	s.FreeMemory = 0
	for i := range b.queues {
		q := &b.queues[i]
		if q.empty() {
			s.FreeQueues += 1
		}
	}

	return s
}

func GetQueueIdFromIndex(idx int) (uint32, error) {
	queueId := uint(idx) + *queueIdLow
	if queueId < *queueIdLow || queueId >= *queueIdHigh {
		return 0, status.Errorf(codes.InvalidArgument, "queue index %v is out of range", idx)
	}
	return uint32(queueId), nil
}

func (b *BufferQueue) GetQueueState(queueId uint64) (s GetQueueStateResponse, err error) {
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
func (b *BufferQueue) ReleasePackets(queueId uint32, drop bool, passthrough bool) error {
	q, err := b.GetQueue(queueId)
	if err != nil {
		return err
	}
	q.lock.Lock()
	defer q.lock.Unlock()
	q.state = GetQueueStateResponse_QUEUE_STATE_DRAINING
	for i := range q.packets {
		if !drop {
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
	q.packets = q.packets[:0]
	//q.packets = make([]bufferPacket, 0, *maxPacketQueueSlots)
	if passthrough {
		q.state = GetQueueStateResponse_QUEUE_STATE_PASSTHROUGH
	} else {
		q.state = GetQueueStateResponse_QUEUE_STATE_BUFFERING
	}
	q.dropTimer.Stop()
	q.dropTimer.Reset(*dropTimeout)

	return nil
}

func (b *BufferQueue) rxFn() {
	for packet := range b.ch {
		parsedPacket := gopacket.NewPacket(packet.payload, layers.LayerTypeGTPv1U, gopacket.Default)
		if gtpLayer := parsedPacket.Layer(layers.LayerTypeGTPv1U); gtpLayer != nil {
			gtp, _ := gtpLayer.(*layers.GTPv1U)
			queueId := gtp.TEID
			//log.Printf("Got buffer with id %d\n", queueId)

			buf := &bufferPacket{packet, queueId}
			b.enqueueBuffer(buf)
		} else {
			log.Printf("Could not parse packet as GTP: %v", packet)
		}
	}
}

func (b *BufferQueue) notifyDrop(queueId uint32) {
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

func (b *BufferQueue) notifyFirst(queueId uint32) {
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

func (b *BufferQueue) enqueueBuffer(pkt *bufferPacket) {
	q, err := b.GetQueue(pkt.id)
	if err != nil {
		log.Printf("Dropped packet. Queue Id %v out of range.", pkt.id)
		b.notifyDrop(pkt.id)
		return
	}
	q.lock.Lock()
	defer q.lock.Unlock()
	if !q.unsafeFull() {
		if q.state == GetQueueStateResponse_QUEUE_STATE_PASSTHROUGH {
			_ = b.di.Send(pkt.udpPacket)
			return
		}
		if q.unsafeEmpty() {
			b.notifyFirst(pkt.id)
		}
		q.packets = append(q.packets, *pkt)
		q.dropTimer.Stop()
		q.dropTimer.Reset(*dropTimeout)
	} else {
		b.notifyDrop(pkt.id)
		incQueueFullDrop(1)
		log.Printf("Dropped packet. Queue with id %v is full.", pkt.id)
	}
}
