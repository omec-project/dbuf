package dbuf

import (
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"testing"
	"time"
)

func TestFoo(*testing.T) {
	var di *dataPlaneInterface
	di = nil
	bq := NewBufferQueue(di)

	bq.Stop()
}

func TestQueue(t *testing.T) {
	queueSize := uint64(10)
	clearTimeout := time.Millisecond * 10
	q := NewQueue(queueSize, time.AfterFunc(time.Second*0, func() {}), clearTimeout)
	pkt := &bufferPacket{}

	if !q.empty() {
		t.Errorf("new queue is not empty")
	}

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
		q2 = nil
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
