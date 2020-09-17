package dbuf

import (
	"expvar"
)

var (
	stats = expvar.NewMap("counters")
)

func init() {
	stats.Add(rxOkStatKey, 0)
	stats.Add(txOkStatKey, 0)
	stats.Add(rxDropStatKey, 0)
	stats.Add(txDropStatKey, 0)
	stats.Add(queueFullDropStatKey, 0)
	stats.Add(queueReleasedStatKey, 0)
	stats.Add(queueDroppedStatKey, 0)
}

const rxOkStatKey string = "rx_ok"
const txOkStatKey string = "tx_ok"
const rxDropStatKey string = "rx_drop"
const txDropStatKey string = "tx_drop"
const queueFullDropStatKey string = "queue_full_discarded_packets"
const queueReleasedStatKey string = "queue_released_packets"
const queueDroppedStatKey string = "queue_dropped_packets"

func incRxOk(delta int64) {
	stats.Add(rxOkStatKey, delta)
}

func incTxOk(delta int64) {
	stats.Add(txOkStatKey, delta)
}

func incRxDrop(delta int64) {
	stats.Add(rxDropStatKey, delta)
}

func incTxDrop(delta int64) {
	stats.Add(txDropStatKey, delta)
}

func incQueueFullDrop(delta int64) {
	stats.Add(queueFullDropStatKey, delta)
}

func incQueueReleased(delta int64) {
	stats.Add(queueReleasedStatKey, delta)
}

func incQueueDrop(delta int64) {
	stats.Add(queueDroppedStatKey, delta)
}
