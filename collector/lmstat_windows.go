// Copyright 2025 Greg Drake
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
//go:build windows
// +build windows

package collector

import (
	"io"
	"os/exec"
	"strings"

	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/iambengiey/rlmlm_exporter/config"
)

var (
	lmstatupDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "lmstat", "up"),
		"Is the lmstat output parseable.",
		[]string{"license_name", "license_server"},
		nil,
	)
)

type LmstatCollector struct {
	config *config.Config
	logger log.Logger
}

func NewLmstatCollector(cfg *config.Config, logger log.Logger) (Collector, error) {
	if logger == nil {
		logger = log.NewNopLogger()
	}

	return &LmstatCollector{
		config: cfg,
		logger: logger,
	}, nil
}

func (c *LmstatCollector) Update(ch chan<- prometheus.Metric) error {
	if c.config == nil {
		return nil
	}

	for _, license := range c.config.Licenses {
		c.lmstatUpdate(ch, license)
	}

	return nil
}

func (c *LmstatCollector) lmstatUpdate(ch chan<- prometheus.Metric, license config.License) {
	level.Debug(c.logger).Log("msg", "running rlmstat", "license", license.Name)

	var (
		server string
		args   = []string{"-a"}
	)

	switch {
	case license.LicenseFile != "":
		server = license.LicenseFile
		args = append(args, "-c", server)
	case license.LicenseServer != "":
		server = license.LicenseServer
		args = append(args, "-c", server)
	default:
		level.Error(c.logger).Log(
			"msg", "missing license target", "license", license.Name,
		)
		ch <- prometheus.MustNewConstMetric(lmstatupDesc, prometheus.GaugeValue, 0, license.Name, "N/A")
		return
	}

	cmd := exec.Command(*rlmstatPath, args...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		level.Error(c.logger).Log("msg", "failed to create stdout pipe", "license", license.Name, "err", err)
		ch <- prometheus.MustNewConstMetric(lmstatupDesc, prometheus.GaugeValue, 0, license.Name, server)
		return
	}

	if err := cmd.Start(); err != nil {
		level.Error(c.logger).Log(
			"msg", "failed to start rlmstat", "license", license.Name,
			"cmd", strings.Join(cmd.Args, " "), "err", err,
		)
		ch <- prometheus.MustNewConstMetric(lmstatupDesc, prometheus.GaugeValue, 0, license.Name, server)
		return
	}

	output, err := io.ReadAll(stdout)
	if err != nil {
		level.Error(c.logger).Log("msg", "failed to read rlmstat output", "license", license.Name, "err", err)
		cmd.Wait()
		ch <- prometheus.MustNewConstMetric(lmstatupDesc, prometheus.GaugeValue, 0, license.Name, server)
		return
	}

	if err := cmd.Wait(); err != nil {
		if len(output) == 0 {
			level.Error(c.logger).Log("msg", "rlmstat exited with error", "license", license.Name, "err", err)
			ch <- prometheus.MustNewConstMetric(lmstatupDesc, prometheus.GaugeValue, 0, license.Name, server)
			return
		}
	}

	ch <- prometheus.MustNewConstMetric(lmstatupDesc, prometheus.GaugeValue, 1, license.Name, server)
	c.parseLmstatOutput(ch, license, server, string(output))
}

func (c *LmstatCollector) parseLmstatOutput(ch chan<- prometheus.Metric, license config.License, server, output string) {
	level.Debug(c.logger).Log(
		"msg", "received rlmstat output", "license", license.Name,
		"target", server, "bytes", len(output),
	)
}

func init() {
	registerCollector("lmstat", defaultEnabled, NewLmstatCollector)
}
