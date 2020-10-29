// Copyright 2020-present Open Networking Foundation
// SPDX-License-Identifier: LicenseRef-ONF-Member-1.0

package dbuf

import (
	"expvar"
	"flag"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"log"
	"net/http"
)

var (
	metricsUrl = flag.String(
		"metrics_url", "0.0.0.0:8080",
		"URL for metrics server to listen for requests",
	)
	stats    = expvar.NewMap("counters")
	promRxOk = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "dbuf_interface_rx_ok_total",
			Help: "The total number of successfully received packets on the interfaces",
		},
	)
	promTxOk = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "dbuf_interface_tx_ok_total",
			Help: "The total number of successfully sent packets on the interfaces",
		},
	)
	promRxDrop = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "dbuf_interface_rx_drop_total",
			Help: "The total number of dropped packets on the receive side of the interfaces",
		},
	)
	promTxDrop = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "dbuf_interface_tx_drop_total",
			Help: "The total number of dropped packets that were supposed to be send out on the interfaces",
		},
	)
	promQueueFullDrop = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "dbuf_queue_full_discarded_total",
			Help: "The total number of discarded packets because of a full queue",
		},
	)
	promQueueReleased = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "dbuf_queue_released_total",
			Help: "The total number of released packets from queues",
		},
	)
	promQueueDrop = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "dbuf_queue_dropped_total",
			Help: "The total number of deliberately drop-released packets from queues",
		},
	)
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

func startMetricsServer() {
	go func() {
		http.Handle("/metrics", promhttp.Handler())
		log.Println(http.ListenAndServe(*metricsUrl, nil))
	}()
	log.Printf("Listening for metrics requests on %v", *metricsUrl)
}

func incRxOk(delta int64) {
	stats.Add(rxOkStatKey, delta)
	promRxOk.Add(float64(delta))
}

func incTxOk(delta int64) {
	stats.Add(txOkStatKey, delta)
	promTxOk.Add(float64(delta))
}

func incRxDrop(delta int64) {
	stats.Add(rxDropStatKey, delta)
	promRxDrop.Add(float64(delta))
}

func incTxDrop(delta int64) {
	stats.Add(txDropStatKey, delta)
	promTxDrop.Add(float64(delta))
}

func incQueueFullDrop(delta int64) {
	stats.Add(queueFullDropStatKey, delta)
	promQueueFullDrop.Add(float64(delta))
}

func incQueueReleased(delta int64) {
	stats.Add(queueReleasedStatKey, delta)
	promQueueReleased.Add(float64(delta))
}

func incQueueDrop(delta int64) {
	stats.Add(queueDroppedStatKey, delta)
	promQueueDrop.Add(float64(delta))
}
