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

// The lmstat collector's metrics.
var (
	lmstatupDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "lmstat", "up"),
		"Is the lmstat output parseable.",
		[]string{"license_name", "license_server"},
		nil,
	)
)

// LmstatCollector implements the Collector interface.
type LmstatCollector struct {
	config *config.Config // Fixed: Changed from config.Configuration to *config.Config
	logger log.Logger     // Added: Logger for go-kit/log
}

// NewLmstatCollector creates a new LmstatCollector.
// NOTE: This constructor now accepts config and logger, matching the updated factory signature in collector.go.
func NewLmstatCollector(cfg *config.Config, logger log.Logger) (Collector, error) {
	if logger == nil {
		logger = log.NewNopLogger()
	}

	return &LmstatCollector{
		config: cfg,
		logger: logger,
	}, nil
}

// Update implements the Collector interface.
func (c *LmstatCollector) Update(ch chan<- prometheus.Metric) error {
	for _, license := range c.config.Licenses {
		c.lmstatUpdate(ch, license)
	}

	return nil
}

// lmstatUpdate executes the rlmstat command and updates metrics for a single license.
func (c *LmstatCollector) lmstatUpdate(ch chan<- prometheus.Metric, license config.License) {
	level.Debug(c.logger).Log("msg", "Running rlmstat for license", "name", license.Name)

	var (
		server string
		args   = []string{"-a"} // Default args to show all features
	)

	// Determine the target server/file based on configuration
	if license.LicenseFile != "" {
		server = license.LicenseFile
		args = append(args, "-c", server)
	} else if license.LicenseServer != "" {
		server = license.LicenseServer
		args = append(args, "-c", server)
	} else {
		// Log error using go-kit/log format
		level.Error(c.logger).Log(
			"msg", "Missing license_file or license_server in config",
			"license", license.Name,
		)
		ch <- prometheus.MustNewConstMetric(lmstatupDesc, prometheus.GaugeValue, 0, license.Name, "N/A")
		return
	}

	cmd := exec.Command("rlmstat", args...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		// Log error using go-kit/log format
		level.Error(c.logger).Log(
			"msg", "Failed to create stdout pipe for rlmstat",
			"license", license.Name,
			"err", err,
		)
		ch <- prometheus.MustNewConstMetric(lmstatupDesc, prometheus.GaugeValue, 0, license.Name, server)
		return
	}

	if err := cmd.Start(); err != nil {
		// Log error using go-kit/log format
		level.Error(c.logger).Log(
			"msg", "Failed to start rlmstat command",
			"license", license.Name,
			"cmd", "rlmstat "+strings.Join(args, " "),
			"err", err,
		)
		ch <- prometheus.MustNewConstMetric(lmstatupDesc, prometheus.GaugeValue, 0, license.Name, server)
		return
	}

	// Read and process the output
	lmstatOutput, err := io.ReadAll(stdout)
	if err != nil {
		level.Error(c.logger).Log("msg", "Failed to read rlmstat output", "license", license.Name, "err", err)
		cmd.Wait() // Ensure the command is waited on even if reading failed
		ch <- prometheus.MustNewConstMetric(lmstatupDesc, prometheus.GaugeValue, 0, license.Name, server)
		return
	}

	if err := cmd.Wait(); err != nil {
		// rlmstat often exits with a non-zero code on success (e.g., if no licenses are in use),
		// but we still want to parse the output if we got any.
		if len(lmstatOutput) == 0 {
			level.Error(c.logger).Log(
				"msg", "rlmstat command failed with no output",
				"license", license.Name,
				"err", err,
			)
			ch <- prometheus.MustNewConstMetric(lmstatupDesc, prometheus.GaugeValue, 0, license.Name, server)
			return
		}
	}

	// Processing logic goes here...
	// For simplicity, we assume successful parsing if we got output.
	// A more robust implementation would check for specific error messages in the output.

	ch <- prometheus.MustNewConstMetric(lmstatupDesc, prometheus.GaugeValue, 1, license.Name, server)

	// Here you would continue with the parsing logic, converting lmstatOutput to metrics...

	// Example parsing placeholder (replace with actual parsing):
	c.parseLmstatOutput(ch, license, server, string(lmstatOutput))
}

// Placeholder for the actual parsing logic
func (c *LmstatCollector) parseLmstatOutput(ch chan<- prometheus.Metric, license config.License, server, output string) {
	level.Debug(c.logger).Log("msg", "Placeholder for rlmstat output parsing", "license", license.Name, "output_length", len(output))
}

// init registers the collector.
func init() {
	registerCollector("lmstat", defaultEnabled, NewLmstatCollector)
}
