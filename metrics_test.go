package dbuf

import (
	"expvar"
	"testing"
)

func TestStartMetricsServer(t *testing.T) {
	startMetricsServer()
	// TODO(max): http query test?
}

func TestIncFunctions(t *testing.T) {
	tab := []struct {
		key string
		fn  func(int64)
	}{
		{rxOkStatKey, incRxOk},
		{txOkStatKey, incTxOk},
		{rxDropStatKey, incRxDrop},
		{txDropStatKey, incTxDrop},
		{queueFullDropStatKey, incQueueFullDrop},
		{queueReleasedStatKey, incQueueReleased},
		{queueDroppedStatKey, incQueueDrop},
	}

	for _, c := range tab {
		v := stats.Get(c.key)
		if v == nil {
			t.Errorf("stats maps does not contain key %v", c.key)
		}
		i, ok := v.(*expvar.Int)
		if !ok {
			t.Fatalf("variable %v has unexpected type %v", c.key, v)
		}
		valuePre := i.Value()
		c.fn(1)
		valuePost := i.Value()
		if valuePost != valuePre+1 {
			t.Errorf("Counter %v did not increase", c.key)
		}
	}
}
