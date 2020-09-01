package dbuf

import (
	"errors"
	"flag"
	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"log"
	"sync"
)

var (
	maxQueueIds         = flag.Int("max_queue_ids", 4, "Maximum number of queue IDs")
	maxPacketQueueSlots = flag.Uint64("max_packet_slots_per_queue", 2, "Maximum number of packet slots in each queue")
	rxQueueDepth        = flag.Int("rx_queue_depth", 100, "Depth of the rx queue for data plane packets")
)

type State struct {
	maximumBufferIds int
	freeBufferIds    int
	maximumMemory    int
	freeMemory       int
	state            GetQueueStateResponse_QueuesState
}

type bufferPacket struct {
	udpPacket
	id uint32
}

type queue struct {
	packets      []bufferPacket
	maximumSlots uint64
	state        GetQueueStateResponse_QueuesState
	lock         sync.Mutex
}

type BufferQueue struct {
	queues         []queue
	ch             chan udpPacket
	dl             *dataPlaneListener
	subscribers    []chan Notification
	subscriberLock sync.RWMutex
}

func NewBufferQueue(dl *dataPlaneListener) *BufferQueue {
	b := &BufferQueue{}
	b.dl = dl
	b.ch = make(chan udpPacket, *rxQueueDepth)
	b.queues = make([]queue, *maxQueueIds)
	for i, _ := range b.queues {
		b.queues[i].maximumSlots = *maxPacketQueueSlots
		b.queues[i].state = GetQueueStateResponse_QUEUE_STATE_EMPTY
	}

	log.Printf("%+v", b)
	return b
}

func (b *BufferQueue) Start(d *dataPlaneListener) (err error) {
	d.SetOutputChannel(b.ch)
	go b.rxFn()
	return
}

func (b *BufferQueue) Stop() (err error) {
	close(b.ch)
	return
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

	return errors.New("channel not found")
}

func (b BufferQueue) GetState() State {
	var s State
	s.maximumBufferIds = len(b.queues)
	s.freeBufferIds = 0
	s.maximumMemory = 0
	s.freeMemory = 0
	return s
}

func (b BufferQueue) GetQueueState(queueId uint64) (s GetQueueStateResponse, err error) {
	if queueId > uint64(len(b.queues)) {
		err = errors.New("out of range")
		return
	}
	q := &b.queues[queueId]
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
// Should this function be blocking?
// Do we need an explicit DRAIN state?
func (b *BufferQueue) ReleasePackets(queueId uint32) error {
	q := &b.queues[queueId]
	q.lock.Lock()
	defer q.lock.Unlock()
	q.state = GetQueueStateResponse_QUEUE_STATE_DRAINING
	for i, _ := range q.packets {
		if err := b.dl.Send(q.packets[i].udpPacket); err != nil {
			return err
		}
	}
	q.packets = q.packets[:0]

	return nil
}

func (b *BufferQueue) rxFn() {
	for packet := range b.ch {
		parsedPacket := gopacket.NewPacket(packet.payload, layers.LayerTypeGTPv1U, gopacket.Default)
		if gtpLayer := parsedPacket.Layer(layers.LayerTypeGTPv1U); gtpLayer != nil {
			log.Println("This is a GTP packet!")
			gtp, _ := gtpLayer.(*layers.GTPv1U)
			queueId := gtp.TEID
			log.Printf("Got buffer with id %d\n", queueId)

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
	if pkt.id >= uint32(len(b.queues)) {
		log.Printf("Droppend packet. Queue Id %v out of range.", pkt.id)
		b.notifyDrop(pkt.id)
		return
	}
	q := &b.queues[pkt.id]
	q.lock.Lock()
	defer q.lock.Unlock()
	if uint64(len(q.packets)) < q.maximumSlots {
		if len(q.packets) == 0 {
			b.notifyFirst(pkt.id)
		}
		q.packets = append(q.packets, *pkt)
	} else {
		log.Printf("Dropped packet. Queue with id %v is full.", pkt.id)
		b.notifyDrop(pkt.id)
	}
}
