// Copyright 2025 Greg Drake
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

package main

import (
	"fmt"
	stdlog "log"
	"net/http"
	_ "net/http/pprof"
	"os"
	"strings"

	"github.com/alecthomas/kingpin/v2"
	gokitlog "github.com/go-kit/log"
	"github.com/go-kit/log/level"
	"github.com/iambengiey/rlmlm_exporter/collector"
	"github.com/iambengiey/rlmlm_exporter/config"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/prometheus/common/promlog"
	"github.com/prometheus/common/version"
)

var (
	appConfig  *config.Config
	baseLogger gokitlog.Logger = gokitlog.NewNopLogger()
)

func init() {
	prometheus.MustRegister(version.NewCollector("rlmlm_exporter"))
}

func handler(w http.ResponseWriter, r *http.Request) {
	filters := r.URL.Query()["collect[]"]
	level.Debug(baseLogger).Log("msg", "collect query", "filters", strings.Join(filters, ","))

	nc, err := collector.NewFlexlmCollector(filters...)
	if err != nil {
		level.Warn(baseLogger).Log("msg", "failed to create filtered collector", "filters", strings.Join(filters, ","), "err", err)
		http.Error(w, fmt.Sprintf("Couldn't create collector: %s", err), http.StatusBadRequest)
		return
	}

	registry := prometheus.NewRegistry()
	if err := registry.Register(nc); err != nil {
		level.Error(baseLogger).Log("msg", "failed to register collector", "err", err)
		http.Error(w, fmt.Sprintf("Couldn't register collector: %s", err), http.StatusInternalServerError)
		return
	}

	gatherers := prometheus.Gatherers{
		prometheus.DefaultGatherer,
		registry,
	}

	h := promhttp.HandlerFor(gatherers, promhttp.HandlerOpts{
		ErrorLog:      stdlog.New(os.Stderr, "promhttp: ", stdlog.LstdFlags),
		ErrorHandling: promhttp.ContinueOnError,
	})
	h.ServeHTTP(w, r)
}

func main() {
	var (
		listenAddress = kingpin.Flag("web.listen-address", "Address on which to expose metrics and web interface.").Default(":9319").String()
		metricsPath   = kingpin.Flag("web.telemetry-path", "Path under which to expose metrics.").Default("/metrics").String()
		configPath    = kingpin.Flag("path.config", "Configuration YAML file path.").Default("licenses.yml").String()
	)

	promlogConfig := promlog.Config{}
	promlogflag.AddFlags(kingpin.CommandLine, &promlogConfig)
	kingpin.Version(version.Print("rlmlm_exporter"))
	kingpin.HelpFlag.Short('h')
	kingpin.Parse()

	baseLogger = promlog.New(promlogConfig)
	collector.SetLogger(baseLogger)
	config.SetLogger(baseLogger)

	level.Info(baseLogger).Log("msg", "Starting rlmlm_exporter", "version", version.Info())
	level.Info(baseLogger).Log("msg", "Build context", "context", version.BuildContext())

	cfg, err := config.Load(*configPath)
	if err != nil {
		level.Error(baseLogger).Log("msg", "failed to load configuration", "path", *configPath, "err", err)
		os.Exit(1)
	}
	appConfig = cfg
	collector.SetConfig(appConfig)

	nc, err := collector.NewFlexlmCollector()
	if err != nil {
		level.Error(baseLogger).Log("msg", "failed to create collector", "err", err)
		os.Exit(1)
	}
	level.Info(baseLogger).Log("msg", "Enabled collectors")
	for name := range nc.Collectors {
		level.Info(baseLogger).Log("msg", "collector enabled", "collector", name)
	}

	http.HandleFunc(*metricsPath, handler)
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if _, err := fmt.Fprintf(w, `<html>
                        <head><title>RLMlm Exporter</title></head>
                        <body>
                        <h1>RLMlm Exporter</h1>
                        <p><a href="%s">Metrics</a></p>
                        </body>
                        </html>`, *metricsPath); err != nil {
			level.Error(baseLogger).Log("msg", "failed to write index page", "err", err)
		}
	})

	level.Info(baseLogger).Log("msg", "Listening", "address", *listenAddress)
	if err := http.ListenAndServe(*listenAddress, nil); err != nil {
		level.Error(baseLogger).Log("msg", "server exited", "err", err)
		os.Exit(1)
	}
}
