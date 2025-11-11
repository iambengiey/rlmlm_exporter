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
