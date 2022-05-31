package wal

import "github.com/prometheus/prometheus/model/labels"

type RefCache interface {
	Load() uint64
	Store(ref uint64)
	Inc() uint64
	GetOrCreateRef(l labels.Labels) (ref uint64, created bool)
	ReleaseRef(ref uint64)
}
