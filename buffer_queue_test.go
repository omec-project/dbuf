package dbuf

import "testing"

func TestFoo(*testing.T) {
	var di *dataPlaneInterface
	di = nil
	bq := NewBufferQueue(di)

	bq.Stop()
}

func TestQueue(t *testing.T) {
	q := NewQueue()
	if !q.empty() {
		t.Errorf("new queue is not empty")
	}
}
