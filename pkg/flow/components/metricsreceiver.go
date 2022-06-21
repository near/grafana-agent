package components

import (
	"github.com/prometheus/prometheus/storage"
)

// MetricsReceiver is the type used by the metrics_forwarder component to
// receive metrics to write to a WAL.
type MetricsReceiver struct {
	storage.Appendable `hcl:"appender"`
}

type MetricsBufferReceiver struct {
	Write func(buffer *MetricsBuffer)
}

// MetricsBuffer contains a time and the buffer necessary, this should NOT be mutated.
type MetricsBuffer struct {
	Timestamp int64       `hcl:"ts"`
	Metrics   []MetricRef `hcl:"metrics"`
}

// MetricRef contains the reference to a label set and the value of that metric
type MetricRef struct {
	RefId uint64  `hcl:"ref"`
	Value float64 `hcl:"value"`
}
