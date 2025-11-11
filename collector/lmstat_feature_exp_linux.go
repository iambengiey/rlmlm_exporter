package collector

import (
	"io"
	"os/exec"
	"regexp"
	"strings"
	"time"

	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/iambengiey/rlmlm_exporter/config"
)

// Metric Descriptors (Assuming they were here)
var (
	// Placeholder for metric descriptors related to feature expiration
	lmstatExpMetricDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "feature", "expiration_timestamp_seconds"),
		"RLMLM feature expiration date as a Unix timestamp.",
		[]string{"license_name", "license_server", "feature", "version"},
		nil,
	)
)

// LmstatFeatureExpCollector implements the Collector interface.
type LmstatFeatureExpCollector struct {
	config *config.Config
	logger log.Logger
}

// NewLmstatFeatureExpLinuxCollector creates a new LmstatFeatureExpCollector for Linux.
// This is called by the generic NewLmstatFeatureExpCollector factory.
func NewLmstatFeatureExpLinuxCollector(cfg *config.Config, logger log.Logger) (Collector, error) {
	if logger == nil {
		logger = log.NewNopLogger()
	}

	return &LmstatFeatureExpCollector{
		config: cfg,
		logger: logger,
	}, nil
}

// Update implements the Collector interface.
func (c *LmstatFeatureExpCollector) Update(ch chan<- prometheus.Metric) error {
	for _, license := range c.config.Licenses {
		c.lmstatFeatureExpUpdate(ch, license)
	}

	return nil
}

// lmstatFeatureExpUpdate executes the rlmstat command to get expiration information.
func (c *LmstatFeatureExpCollector) lmstatFeatureExpUpdate(ch chan<- prometheus.Metric, license config.License) {
	level.Debug(c.logger).Log("msg", "Running rlmstat for feature expiration", "name", license.Name)

	var (
		server string
		args   = []string{"-a", "-i"} // -i for license information
	)

	if license.LicenseFile != "" {
		server = license.LicenseFile
		args = append(args, "-c", server)
	} else if license.LicenseServer != "" {
		server = license.LicenseServer
		args = append(args, "-c", server)
	} else {
		// FIX: Replaced undefined log.Errorf with go-kit/log
		level.Error(c.logger).Log(
			"msg", "Missing license_file or license_server for expiration collector",
			"license", license.Name,
		)
		return
	}

	cmd := exec.Command("rlmstat", args...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		// FIX: Replaced undefined log.Errorf with go-kit/log
		level.Error(c.logger).Log(
			"msg", "Failed to create stdout pipe for rlmstat exp",
			"license", license.Name,
			"err", err,
		)
		return
	}

	if err := cmd.Start(); err != nil {
		// FIX: Replaced undefined log.Errorf with go-kit/log
		level.Error(c.logger).Log(
			"msg", "Failed to start rlmstat exp command",
			"license", license.Name,
			"cmd", "rlmstat "+strings.Join(args, " "),
			"err", err,
		)
		return
	}

	lmstatOutput, err := io.ReadAll(stdout)
	if err != nil {
		// FIX: Replaced undefined log.Errorln with go-kit/log
		level.Error(c.logger).Log("msg", "Failed to read rlmstat exp output", "license", license.Name, "err", err)
		cmd.Wait()
		return
	}

	if err := cmd.Wait(); err != nil {
		// This block is often where a log.Fatalf/Fatalln was used.
		// Since collectors shouldn't crash the main process, we log an error and return.

		// FIX: Replaced undefined log.Fatalf/Fatalln with level.Error and return
		if strings.Contains(string(lmstatOutput), "License server status: Error") {
			level.Error(c.logger).Log(
				"msg", "License server error during expiration check (rlmstat -i)",
				"license", license.Name,
				"err", err,
			)
			return
		}

		level.Error(c.logger).Log(
			"msg", "rlmstat exp command failed with error",
			"license", license.Name,
			"err", err,
		)
		return
	}

	// Logic to parse expiration date from output
	c.parseLmstatExpirationOutput(ch, license, server, string(lmstatOutput))
}

// Regex to find expiration lines (Example structure, adjust as needed)
var expRegexp = regexp.MustCompile(`Expires: (\w{3} \d{2}, \d{4})`)

func (c *LmstatFeatureExpCollector) parseLmstatExpirationOutput(ch chan<- prometheus.Metric, license config.License, server, output string) {
	lines := strings.Split(output, "\n")

	// Placeholder for tracking current feature/version being parsed
	currentFeature := ""
	currentVersion := ""

	for _, line := range lines {
		if strings.HasPrefix(line, "Feature:") {
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				currentFeature = parts[1]
			}
			// Version parsing logic might be needed here or another line
			continue
		}

		match := expRegexp.FindStringSubmatch(line)
		if len(match) > 1 {
			expiryDateStr := match[1]

			// Parse the date (e.g., "Dec 31, 2025")
			t, err := time.Parse("Jan 02, 2006", expiryDateStr)
			if err != nil {
				level.Error(c.logger).Log(
					"msg", "Failed to parse expiration date",
					"date", expiryDateStr,
					"err", err,
				)
				continue
			}

			// Report the metric (assuming we found a feature/version context)
			if currentFeature != "" {
				ch <- prometheus.MustNewConstMetric(
					lmstatExpMetricDesc,
					prometheus.GaugeValue,
					float64(t.Unix()),
					license.Name,
					server,
					currentFeature,
					currentVersion,
				)
			}
		}
	}
}
