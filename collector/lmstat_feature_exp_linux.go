//go:build linux
// +build linux

package collector

import (
	"errors"
	"io"
	"os/exec"
	"regexp"
	"strings"
	"time"

	"github.com/go-kit/log/level"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/iambengiey/rlmlm_exporter/config"
)

// getLmstatFeatureExpDate fetches and exposes feature expiration data for each configured license.
func (c *lmstatFeatureExpCollector) getLmstatFeatureExpDate(ch chan<- prometheus.Metric) error {
	if c.config == nil {
		return nil
	}

	var firstErr error
	for _, license := range c.config.Licenses {
		if err := c.lmstatFeatureExpUpdate(ch, license); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

// lmstatFeatureExpUpdate executes rlmstat and parses the feature expiration information for a single license.
func (c *lmstatFeatureExpCollector) lmstatFeatureExpUpdate(ch chan<- prometheus.Metric, license config.License) error {
	level.Debug(c.logger).Log("msg", "collecting feature expiration", "license", license.Name)

	args, server, err := buildLicenseCommandArgs(license)
	if err != nil {
		level.Error(c.logger).Log("msg", "skipping license without server configuration", "license", license.Name, "err", err)
		return err
	}

	cmd := exec.Command("rlmstat", args...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		level.Error(c.logger).Log("msg", "failed to get rlmstat stdout", "license", license.Name, "err", err)
		return err
	}

	if err := cmd.Start(); err != nil {
		level.Error(c.logger).Log("msg", "failed to start rlmstat", "license", license.Name, "err", err)
		return err
	}

	output, readErr := io.ReadAll(stdout)
	if readErr != nil {
		level.Error(c.logger).Log("msg", "failed to read rlmstat output", "license", license.Name, "err", readErr)
		_ = cmd.Wait()
		return readErr
	}

	if err := cmd.Wait(); err != nil && len(output) == 0 {
		level.Error(c.logger).Log("msg", "rlmstat exited with error and no output", "license", license.Name, "err", err)
		return err
	}

	c.parseLmstatExpirationOutput(ch, license, server, string(output))
	return nil
}

func buildLicenseCommandArgs(license config.License) ([]string, string, error) {
	args := []string{"-a", "-i"}
	switch {
	case license.LicenseFile != "":
		args = append(args, "-c", license.LicenseFile)
		return args, license.LicenseFile, nil
	case license.LicenseServer != "":
		args = append(args, "-c", license.LicenseServer)
		return args, license.LicenseServer, nil
	default:
		return nil, "", errNoLicenseTarget
	}
}

var errNoLicenseTarget = errors.New("missing license_file or license_server")

var expRegexp = regexp.MustCompile(`Expires: (\w{3} \d{2}, \d{4})`)

func (c *lmstatFeatureExpCollector) parseLmstatExpirationOutput(ch chan<- prometheus.Metric, license config.License, server, output string) {
	lines := strings.Split(output, "\n")

	currentFeature := ""
	currentVersion := ""

	for _, line := range lines {
		if strings.HasPrefix(line, "Feature:") {
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				currentFeature = parts[1]
			}
			continue
		}

		match := expRegexp.FindStringSubmatch(line)
		if len(match) < 2 {
			continue
		}

		expiryDateStr := match[1]
		t, err := time.Parse("Jan 02, 2006", expiryDateStr)
		if err != nil {
			level.Error(c.logger).Log("msg", "failed to parse expiration date", "license", license.Name, "value", expiryDateStr, "err", err)
			continue
		}

		if currentFeature == "" {
			continue
		}

		ch <- prometheus.MustNewConstMetric(
			c.lmstatFeatureExp,
			prometheus.GaugeValue,
			float64(t.Unix()),
			license.Name,
			currentFeature,
			"0",
			server,
			"",
			currentVersion,
		)
	}
}
