package components

import "github.com/prometheus/prometheus/storage"

// MetricsReceiver is the type used by the metrics_forwarder component to
// receive metrics to write to a WAL.
type MetricsReceiver struct {
	storage.Appendable `hcl:"appender"`
}
