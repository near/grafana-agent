package wal

import (
	"github.com/prometheus/prometheus/model/labels"
	"go.uber.org/atomic"
)

type TraditionalCache struct {
	refid *atomic.Uint64
}

func NewTraditionalCache() *TraditionalCache {
	return &TraditionalCache{refid: atomic.NewUint64(0)}
}

func (t *TraditionalCache) Load() uint64 {
	return t.refid.Load()
}

func (t *TraditionalCache) Store(ref uint64) {
	t.refid.Store(ref)
}

func (t *TraditionalCache) Inc() uint64 {
	return t.refid.Inc()
}

func (t *TraditionalCache) GetOrCreateRef(_ labels.Labels) (ref uint64, created bool) {
	// Traditional cache always increments the value
	return t.refid.Inc(), true
}

func (t *TraditionalCache) ReleaseRef(_ uint64) {}
