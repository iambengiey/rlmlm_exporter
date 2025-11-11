package prometheus

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
)

type Labels map[string]string

type ValueType string

const (
	CounterValue ValueType = "counter"
	GaugeValue   ValueType = "gauge"
)

type Desc struct {
	fqName         string
	help           string
	variableLabels []string
	constLabels    Labels
}

type Metric interface {
	Desc() *Desc
	Value() float64
	ValueType() ValueType
	LabelValues() []string
}

type constMetric struct {
	desc        *Desc
	value       float64
	valueType   ValueType
	labelValues []string
}

func (m constMetric) Desc() *Desc           { return m.desc }
func (m constMetric) Value() float64        { return m.value }
func (m constMetric) ValueType() ValueType  { return m.valueType }
func (m constMetric) LabelValues() []string { return append([]string{}, m.labelValues...) }

func NewDesc(fqName, help string, variableLabels []string, constLabels Labels) *Desc {
	if constLabels == nil {
		constLabels = Labels{}
	}
	return &Desc{fqName: fqName, help: help, variableLabels: append([]string{}, variableLabels...), constLabels: constLabels}
}

func MustNewConstMetric(desc *Desc, valueType ValueType, value float64, labelValues ...string) Metric {
	if desc == nil {
		panic("nil desc")
	}
	if len(labelValues) != len(desc.variableLabels) {
		panic("incorrect number of label values")
	}
	return constMetric{desc: desc, value: value, valueType: valueType, labelValues: append([]string{}, labelValues...)}
}

func BuildFQName(namespace, subsystem, name string) string {
	parts := []string{}
	if namespace != "" {
		parts = append(parts, namespace)
	}
	if subsystem != "" {
		parts = append(parts, subsystem)
	}
	if name != "" {
		parts = append(parts, name)
	}
	return strings.Join(parts, "_")
}

type Collector interface {
	Describe(chan<- *Desc)
	Collect(chan<- Metric)
}

type Registry struct {
	mu         sync.Mutex
	collectors []Collector
}

func NewRegistry() *Registry {
	return &Registry{}
}

func (r *Registry) Register(c Collector) error {
	if c == nil {
		return errors.New("nil collector")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.collectors = append(r.collectors, c)
	return nil
}

func (r *Registry) Gather() ([]*MetricFamily, error) {
	r.mu.Lock()
	collectors := append([]Collector{}, r.collectors...)
	r.mu.Unlock()
	var families []*MetricFamily
	for _, collector := range collectors {
		descCh := make(chan *Desc)
		metricCh := make(chan Metric)
		go func() {
			collector.Describe(descCh)
			close(descCh)
		}()
		go func() {
			collector.Collect(metricCh)
			close(metricCh)
		}()

		for range descCh {
			// Drain described metrics to satisfy the Collector contract.
		}
		metrics := []Metric{}
		for m := range metricCh {
			metrics = append(metrics, m)
		}
		for _, metric := range metrics {
			desc := metric.Desc()
			family := findOrCreateFamily(&families, desc, metric.ValueType())
			family.Metrics = append(family.Metrics, sampleFromMetric(metric, desc))
		}
	}
	return families, nil
}

type MetricSample struct {
	Labels map[string]string
	Value  float64
}

type MetricFamily struct {
	Name    string
	Help    string
	Type    ValueType
	Metrics []MetricSample
}

func sampleFromMetric(metric Metric, desc *Desc) MetricSample {
	labels := make(map[string]string, len(desc.constLabels)+len(desc.variableLabels))
	for k, v := range desc.constLabels {
		labels[k] = v
	}
	for i, name := range desc.variableLabels {
		labels[name] = metric.LabelValues()[i]
	}
	return MetricSample{Labels: labels, Value: metric.Value()}
}

func findOrCreateFamily(families *[]*MetricFamily, desc *Desc, valueType ValueType) *MetricFamily {
	for _, family := range *families {
		if family.Name == desc.fqName {
			return family
		}
	}
	family := &MetricFamily{Name: desc.fqName, Help: desc.help, Type: valueType}
	*families = append(*families, family)
	return family
}

var defaultRegistry = NewRegistry()

func MustRegister(cs ...Collector) {
	for _, c := range cs {
		if err := defaultRegistry.Register(c); err != nil {
			panic(err)
		}
	}
}

var DefaultGatherer Gatherer = defaultRegistry

type Gatherer interface {
	Gather() ([]*MetricFamily, error)
}

type Gatherers []Gatherer

func (gs Gatherers) Gather() ([]*MetricFamily, error) {
	var families []*MetricFamily
	for _, g := range gs {
		if g == nil {
			continue
		}
		fm, err := g.Gather()
		if err != nil {
			return nil, err
		}
		families = append(families, fm...)
	}
	sort.Slice(families, func(i, j int) bool { return families[i].Name < families[j].Name })
	return families, nil
}

func (m constMetric) String() string {
	return fmt.Sprintf("%s %f", m.desc.fqName, m.value)
}
