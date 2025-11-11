// Copyright 2017 Mario Trangoni
// Copyright 2015 The Prometheus Authors
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package collector includes all individual collectors to gather and export rlmlm metrics.
package collector

import (
	"fmt"
	"sync"
	"time"

	"github.com/alecthomas/kingpin/v2"
	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/iambengiey/rlmlm_exporter/config" // Import config package
)

// Namespace defines the common namespace to be used by all metrics.
const namespace = "rlmlm"

var (
	scrapeDurationDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "scrape", "collector_duration_seconds"),
		"rlmlm_exporter: Duration of a collector scrape.",
		[]string{"collector"},
		nil,
	)
	scrapeSuccessDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "scrape", "collector_success"),
		"rlmlm_exporter: Whether a collector succeeded.",
		[]string{"collector"},
		nil,
	)
)

const (
	defaultEnabled = true
	upString       = "UP"
)

var (
	factories      = make(map[string]func(*config.Config, log.Logger) (Collector, error))
	collectorState = make(map[string]*bool)
	defaultConfig  *config.Config
	defaultLogger  log.Logger = log.NewNopLogger()
)

// SetConfig allows the main package to provide the parsed configuration so that
// helper constructors (like the legacy NewFlexlmCollector) can continue to
// operate without requiring callers to thread the value through manually.
func SetConfig(cfg *config.Config) {
	defaultConfig = cfg
}

// SetLogger stores a reusable logger for helper constructors and collectors
// that rely on a shared instance.
func SetLogger(logger log.Logger) {
	if logger != nil {
		defaultLogger = logger
	}
}

// NewFlexlmCollector keeps backwards compatibility with historical callers
// that only provided a list of collector filters. It relies on the
// configuration and logger set via SetConfig/SetLogger.
func NewFlexlmCollector(filters ...string) (*RlmlmCollector, error) {
	return NewRlmlmCollector(defaultConfig, defaultLogger, filters...)
}

// Collector is the interface a collector has to implement.
type Collector interface {
	// Get new metrics and expose them via prometheus registry.
	Update(ch chan<- prometheus.Metric) error
}

func registerCollector(collector string, isDefaultEnabled bool, factory func(*config.Config, log.Logger) (Collector, error)) {
	var helpDefaultState string
	if isDefaultEnabled {
		helpDefaultState = "enabled"
	} else {
		helpDefaultState = "disabled"
	}

	flagName := fmt.Sprintf("collector.%s", collector)
	flagHelp := fmt.Sprintf("Enable the %s collector (default: %s).", collector, helpDefaultState)
	defaultValue := fmt.Sprintf("%v", isDefaultEnabled)

	flag := kingpin.Flag(flagName, flagHelp).Default(defaultValue).Bool()
	collectorState[collector] = flag

	factories[collector] = factory
}

// RlmlmCollector implements the prometheus.Collector interface, storing config and logger.
type RlmlmCollector struct {
	Config     *config.Config
	Logger     log.Logger
	Collectors map[string]Collector
}

// NewRlmlmCollector creates a new RlmlmCollector, replacing the old NewFlexlmCollector.
func NewRlmlmCollector(cfg *config.Config, logger log.Logger, filters ...string) (*RlmlmCollector, error) {
	if logger == nil {
		logger = defaultLogger
	}
	if logger == nil {
		logger = log.NewNopLogger()
	}

	if cfg == nil {
		cfg = defaultConfig
	}
	if cfg == nil {
		return nil, fmt.Errorf("no configuration loaded")
	}

	f := make(map[string]bool)
	for _, filter := range filters {
		enabled, exist := collectorState[filter]
		if !exist {
			return nil, fmt.Errorf("missing collector: %s", filter)
		}
		if !*enabled {
			return nil, fmt.Errorf("disabled collector: %s", filter)
		}
		f[filter] = true
	}

	collectors := make(map[string]Collector)
	for key, enabled := range collectorState {
		if *enabled {
			// Pass config and logger to the factory function
			collector, err := factories[key](cfg, logger)
			if err != nil {
				return nil, err
			}
			if len(f) == 0 || f[key] {
				collectors[key] = collector
			}
		}
	}

	return &RlmlmCollector{
		Config:     cfg,
		Logger:     logger,
		Collectors: collectors,
	}, nil
}

// Describe implements the prometheus.Collector interface.
func (c RlmlmCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- scrapeDurationDesc
	ch <- scrapeSuccessDesc
}

// Collect implements the prometheus.Collector interface.
func (c RlmlmCollector) Collect(ch chan<- prometheus.Metric) {
	wg := sync.WaitGroup{}
	wg.Add(len(c.Collectors))
	for name, collector := range c.Collectors {
		go func(name string, collector Collector) {
			c.execute(name, collector, ch)
			wg.Done()
		}(name, collector)
	}
	wg.Wait()
}

// execute runs the collector and handles logging the result.
func (c RlmlmCollector) execute(name string, collector Collector, ch chan<- prometheus.Metric) {
	begin := time.Now()
	err := collector.Update(ch)
	duration := time.Since(begin)
	var success float64

	if err != nil {
		// --- LOGGING MIGRATION: log.Errorf -> level.Error(c.Logger).Log() ---
		level.Error(c.Logger).Log(
			"msg", "collector failed",
			"collector", name,
			"duration_seconds", duration.Seconds(),
			"err", err,
		)
		success = 0
	} else {
		// --- LOGGING MIGRATION: log.Debugf -> level.Debug(c.Logger).Log() ---
		level.Debug(c.Logger).Log(
			"msg", "collector succeeded",
			"collector", name,
			"duration_seconds", duration.Seconds(),
		)
		success = 1
	}

	ch <- prometheus.MustNewConstMetric(scrapeDurationDesc, prometheus.GaugeValue, duration.Seconds(), name)
	ch <- prometheus.MustNewConstMetric(scrapeSuccessDesc, prometheus.GaugeValue, success, name)
}

type typedDesc struct {
	desc      *prometheus.Desc
	valueType prometheus.ValueType
}

func (d *typedDesc) mustNewConstMetric(value float64, labels ...string) prometheus.Metric {
	return prometheus.MustNewConstMetric(d.desc, d.valueType, value, labels...)
}
