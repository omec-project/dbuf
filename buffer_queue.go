package dbuf

import (
	"flag"
	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"log"
)

var (
	maxQueueIds         = flag.Int("max_queue_ids", 4, "Maximum number of queue IDs")
	maxPacketQueueSlots = flag.Int("max_packet_slots_per_queue", 2, "Maximum number of packet slots in each queue")
	rxQueueDepth        = flag.Int("rx_queue_depth", 100, "Depth of the rx queue for data plane packets")
)

type bufferPacket struct {
	udpPacket
	id uint32
}

type queue struct {
	packets      []bufferPacket
	maximumSlots int
}

type State struct {
	maximumBufferIds int
	freeBufferIds    int
	maximumMemory    int
	freeMemory       int
}

type BufferQueue struct {
	queues []queue
	ch     chan udpPacket
	dl     *dataPlaneListener
}

func NewBufferQueue(dl *dataPlaneListener) *BufferQueue {
	b := &BufferQueue{}
	b.dl = dl
	b.ch = make(chan udpPacket, *rxQueueDepth)
	b.queues = make([]queue, *maxQueueIds)
	for i, _ := range b.queues {
		b.queues[i].maximumSlots = *maxPacketQueueSlots
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

func (b BufferQueue) GetState() State {
	var s State
	s.maximumBufferIds = len(b.queues)
	s.freeBufferIds = 0
	s.maximumMemory = -1
	s.freeMemory = -1
	return s
}

func (b *BufferQueue) ReleasePackets(queueId int) error {
	q := &b.queues[queueId]
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
			// Get actual TCP data from this layer
			gtp, _ := gtpLayer.(*layers.GTPv1U)
			queueId := gtp.TEID
			log.Printf("Got buffer with id %d\n", queueId)

			buf := &bufferPacket{packet, queueId}
			b.enqueueBuffer(buf)
		} else {
			log.Printf("Could not parse packet as GTP: %v", packet)
			// TESTING ONLY
			buf := &bufferPacket{packet, 1}
			b.enqueueBuffer(buf)
		}
	}
}

func (b *BufferQueue) enqueueBuffer(pkt *bufferPacket) {
	q := &b.queues[pkt.id]
	if len(q.packets) < q.maximumSlots {
		q.packets = append(q.packets, *pkt)
	} else {
		log.Printf("Droppend packet. Queue with id %v is full: %v", pkt.id, q)
	}
}
