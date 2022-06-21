package components

import (
	"context"

	"github.com/prometheus/prometheus/model/exemplar"
	"github.com/prometheus/prometheus/model/labels"
	"github.com/prometheus/prometheus/storage"
)

type FlowAppender struct {
	Append func(metrics []FlowMetric)
}

type FlowMetric struct {
	ref storage.SeriesRef
	l   labels.Labels
	t   int64
	v   float64
}

type PromFlowAppendable struct {
	Appenders []*FlowAppender
}

func (p *PromFlowAppendable) Appender(ctx context.Context) storage.Appender {
	return &promFlowAppender{
		buffer:    make([]FlowMetric, 0),
		Appenders: p.Appenders,
	}
}

type promFlowAppender struct {
	buffer    []FlowMetric
	Appenders []*FlowAppender
}

func (p *promFlowAppender) Append(ref storage.SeriesRef, l labels.Labels, t int64, v float64) (storage.SeriesRef, error) {
	p.buffer = append(p.buffer, FlowMetric{
		ref: ref,
		l:   l,
		t:   t,
		v:   v,
	})
	return ref, nil
}

func (p *promFlowAppender) Commit() error {
	for _, a := range p.Appenders {
		a.Append(p.buffer)
	}
	p.buffer = make([]FlowMetric, 0)
	return nil
}

func (p *promFlowAppender) Rollback() error {
	p.buffer = make([]FlowMetric, 0)
	return nil
}

func (p *promFlowAppender) AppendExemplar(ref storage.SeriesRef, l labels.Labels, e exemplar.Exemplar) (storage.SeriesRef, error) {
	//TODO implement me
	panic("implement me")
}
