package scraperbuffer

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/grafana/agent/pkg/flow/components"

	"github.com/alecthomas/units"
	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
	"github.com/grafana/agent/component"
	"github.com/grafana/agent/pkg/build"
	"github.com/hashicorp/hcl/v2"
	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/config"
	"github.com/prometheus/prometheus/discovery/targetgroup"
	"github.com/prometheus/prometheus/model/exemplar"
	"github.com/prometheus/prometheus/model/labels"
	"github.com/prometheus/prometheus/scrape"
	"github.com/prometheus/prometheus/storage"
	"github.com/rfratto/gohcl"
)

func init() {
	scrape.UserAgent = fmt.Sprintf("GrafanaAgent/%s", build.Version)

	component.Register(component.Registration{
		Name: "metrics_scraper_buffer",
		Args: Config{},
		Build: func(o component.Options, c component.Arguments) (component.Component, error) {
			return NewComponent(o, c.(Config))
		},
	})
}

// Config represents the input state of the metrics_scraper component.
type Config struct {
	Targets []TargetGroup `hcl:"targets"`

	HonorLabels           bool                `hcl:"honor_labels,optional"`
	HonorTimestamps       bool                `hcl:"honor_timestamps,optional"`
	Params                map[string][]string `hcl:"params,optional"`
	ScrapeInterval        time.Duration       `hcl:"scrape_interval,optional"`
	ScrapeTimeout         time.Duration       `hcl:"scrape_timeout,optional"`
	MetricsPath           string              `hcl:"metrics_path,optional"`
	Scheme                string              `hcl:"scheme,optional"`
	BodySizeLimit         units.Base2Bytes    `hcl:"body_size_limit,optional"`
	SampleLimit           uint                `hcl:"sample_limit,optional"`
	TargetLimit           uint                `hcl:"target_limit,optional"`
	LabelLimit            uint                `hcl:"label_limit,optional"`
	LabelNameLengthLimit  uint                `hcl:"label_name_length_limit,optional"`
	LabelValueLengthLimit uint                `hcl:"label_value_length_limit,optional"`

	// TODO(rfratto): http client config
	Receiver []*components.MetricsBuffer `hcl:"receiver"`
}

var DefaultConfig = Config{
	MetricsPath:     "/metrics",
	Scheme:          "http",
	HonorLabels:     false,
	HonorTimestamps: true,
	ScrapeInterval:  time.Duration(60 * time.Second),
	ScrapeTimeout:   time.Duration(10 * time.Second),
}

var _ gohcl.Decoder = (*Config)(nil)

func (c *Config) DecodeHCL(body hcl.Body, ctx *hcl.EvalContext) error {
	*c = DefaultConfig

	type config Config
	return gohcl.DecodeBody(body, ctx, (*config)(c))
}

// TargetGroup is a set of targets that share a common set of labels.
type TargetGroup struct {
	Targets []LabelSet `hcl:"targets"`
	Labels  LabelSet   `hcl:"labels,optional"`
}

// LabelSet is a map of label names to values.
type LabelSet map[string]string

// Component is the metrics_scraper component.
type Component struct {
	log log.Logger
	id  string

	mut sync.RWMutex
	cfg Config

	newTargets chan struct{}
	scraper    *scrape.Manager
	app        *lazyAppendable
}

type scrapeAppendable struct {
	// Not sure if this actually necessary
	mut      sync.Mutex
	buffer   map[int64]*components.MetricsBuffer
	receiver []*components.MetricsBuffer
}

func newScrapeAppendable(receiver []*components.MetricsBuffer) *scrapeAppendable {
	return &scrapeAppendable{
		buffer:   make(map[int64]*components.MetricsBuffer),
		receiver: receiver,
	}
}

func (s *scrapeAppendable) Append(ref storage.SeriesRef, l labels.Labels, t int64, v float64) (storage.SeriesRef, error) {
	s.mut.Lock()
	defer s.mut.Unlock()
	set, found := s.buffer[t]
	if !found {
		set = &components.MetricsBuffer{
			Timestamp: t,
			Metrics:   make([]components.MetricRef, 0),
		}
		s.buffer[t] = set
	}
	set.Metrics = append(set.Metrics, components.MetricRef{
		RefId: 0,
		Value: v,
	})
}

func (s *scrapeAppendable) Commit() error {
	s.mut.Lock()
	defer s.mut.Unlock()
}

func (s *scrapeAppendable) Rollback() error {
	s.mut.Lock()
	defer s.mut.Unlock()
}

func (s *scrapeAppendable) AppendExemplar(ref storage.SeriesRef, l labels.Labels, e exemplar.Exemplar) (storage.SeriesRef, error) {
	//TODO implement me
	panic("implement me")
}

func (s *scrapeAppendable) Appender(ctx context.Context) storage.Appender {
	return s
}

// NewComponent creates a new metrics_scraper component.
func NewComponent(o component.Options, c Config) (*Component, error) {
	app := &lazyAppendable{id: o.ID}
	if c.Receiver != nil {
		app.Set(c.Receiver)
	}

	scrapeLogger := log.With(o.Logger, "subcomponent", "scrape")
	scraper := scrape.NewManager(&scrape.Options{}, scrapeLogger, app)

	res := &Component{
		log: o.Logger,
		id:  o.ID,

		app:        app,
		scraper:    scraper,
		newTargets: make(chan struct{}, 1),
	}
	if err := res.Update(c); err != nil {
		return nil, err
	}
	return res, nil
}

var _ component.Component = (*Component)(nil)

// Run implements Component.
func (c *Component) Run(ctx context.Context) error {
	defer c.scraper.Stop()

	targetChan := make(chan map[string][]*targetgroup.Group)

	go func() {
		err := c.scraper.Run(targetChan)
		if err != nil {
			level.Error(c.log).Log("msg", "scraper failed", "err", err)
		}
	}()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-c.newTargets:
			c.mut.RLock()
			targets := c.cfg.Targets
			c.mut.RUnlock()

			// Try to send the targets
			promTargets := c.convertTargets(targets)
			select {
			case <-ctx.Done():
			case targetChan <- promTargets:
				level.Debug(c.log).Log("msg", "passed targets to scrape manager", "count", len(targets))
			}
		}
	}
}

func (c *Component) convertTargets(groups []TargetGroup) map[string][]*targetgroup.Group {
	var promGroups []*targetgroup.Group

	for _, g := range groups {
		var promGroup targetgroup.Group
		for _, target := range g.Targets {
			promGroup.Targets = append(promGroup.Targets, convertLabelSet(target))
		}
		promGroup.Labels = convertLabelSet(g.Labels)
		promGroup.Source = c.id
		promGroups = append(promGroups, &promGroup)
	}

	return map[string][]*targetgroup.Group{c.id: promGroups}
}

func convertLabelSet(in LabelSet) model.LabelSet {
	out := make(model.LabelSet, len(in))
	for k, v := range in {
		out[model.LabelName(k)] = model.LabelValue(v)
	}
	return out
}

// Update implements UpdatableComponent.
func (c *Component) Update(newConfig component.Arguments) error {
	cfg := newConfig.(Config)

	c.mut.Lock()
	defer c.mut.Unlock()

	sc := config.DefaultScrapeConfig
	sc.JobName = c.id
	sc.HonorLabels = cfg.HonorLabels
	sc.HonorTimestamps = cfg.HonorTimestamps
	sc.Params = cfg.Params
	sc.ScrapeInterval = model.Duration(cfg.ScrapeInterval)
	sc.ScrapeTimeout = model.Duration(cfg.ScrapeTimeout)
	sc.MetricsPath = cfg.MetricsPath
	sc.Scheme = cfg.Scheme
	sc.BodySizeLimit = cfg.BodySizeLimit
	sc.SampleLimit = cfg.SampleLimit
	sc.TargetLimit = cfg.TargetLimit
	sc.LabelLimit = cfg.LabelLimit
	sc.LabelNameLengthLimit = cfg.LabelNameLengthLimit
	sc.LabelValueLengthLimit = cfg.LabelValueLengthLimit

	err := c.scraper.ApplyConfig(&config.Config{
		ScrapeConfigs: []*config.ScrapeConfig{&sc},
	})
	if err != nil {
		return fmt.Errorf("error applying targets: %w", err)
	}

	if cfg.Receiver != nil {
		c.app.Set(cfg.Receiver)
	}

	c.cfg = cfg

	select {
	case c.newTargets <- struct{}{}:
	default:
	}
	return nil
}

// CurrentState implements Component.
func (c *Component) CurrentState() interface{} {
	return nil
}

// Config implements Component.
func (c *Component) Config() Config {
	c.mut.RLock()
	defer c.mut.RUnlock()
	return c.cfg
}

type lazyAppendable struct {
	id    string
	mut   sync.RWMutex
	inner []storage.Appender
}

func (la *lazyAppendable) Append(ref storage.SeriesRef, l labels.Labels, t int64, v float64) (storage.SeriesRef, error) {
	if len(la.inner) == 0 {
		return 0, nil
	}
	newRef := storage.SeriesRef(0)
	for _, in := range la.inner {
		nr, err := in.Append(ref, l, t, v)
		newRef = nr
		if err != nil {
			return 0, err
		}
	}
	return newRef, nil
}

func (la *lazyAppendable) Commit() error {
	if len(la.inner) == 0 {
		return nil
	}
	for _, in := range la.inner {
		err := in.Commit()
		if err != nil {
			return err
		}
	}
	return nil
}

func (la *lazyAppendable) Rollback() error {
	if len(la.inner) == 0 {
		return nil
	}
	for _, in := range la.inner {
		err := in.Rollback()
		if err != nil {
			return err
		}
	}
	return nil
}

func (la *lazyAppendable) AppendExemplar(ref storage.SeriesRef, l labels.Labels, e exemplar.Exemplar) (storage.SeriesRef, error) {
	return 0, nil
}

var _ storage.Appendable = (*lazyAppendable)(nil)

func (la *lazyAppendable) Appender(ctx context.Context) storage.Appender {
	la.mut.RLock()
	defer la.mut.RUnlock()
	return la
}

func (la *lazyAppendable) Set(app []*components.MetricsReceiver) {
	la.mut.Lock()
	defer la.mut.Unlock()
	la.inner = make([]storage.Appender, 0)
	if len(app) == 0 {
		return
	}
	// TODO there is totally an issue with locking here but not above
	la.inner = make([]storage.Appender, 0)
	for _, a := range app {
		if a == nil {
			continue
		}
		la.inner = append(la.inner, a.Appender(context.Background()))
	}
}
