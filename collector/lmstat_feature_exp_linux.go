//go:build linux
// +build linux

package collector

import (
	"bytes"
	"encoding/csv"
	"errors"
	"fmt"
	"math"
	"os"
	"os/exec"
	"strconv"
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
		if err := c.collectFeatureExpForLicense(ch, license); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

// lmstatFeatureExpUpdate executes the rlmstat command to get expiration information.
func (c *LmstatFeatureExpCollector) lmstatFeatureExpUpdate(ch chan<- prometheus.Metric, license config.License) {
	level.Debug(c.logger).Log("msg", "Running rlmstat for feature expiration", "name", license.Name)

	if license.FeaturesToExclude != "" && license.FeaturesToInclude != "" {
		err := fmt.Errorf("features_to_include and features_to_exclude are both set for %s", license.Name)
		level.Error(c.logger).Log("msg", "invalid feature filter configuration", "license", license.Name, "err", err)
		return err
	}

	args := []string{"-i"}
	target := license.LicenseServer
	if license.LicenseFile != "" {
		target = license.LicenseFile
	}
	args = append(args, "-c", target)

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

	rlmstatOutput, err := io.ReadAll(stdout)
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
		if strings.Contains(string(rlmstatOutput), "License server status: Error") {
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
	}

	return nil
}

func runRlmstatCommand(args ...string) ([]byte, error) {
	cmd := exec.Command("rlmstat", args...)
	cmd.Env = append(os.Environ(), "LANG=C")

	out, err := cmd.Output()
	if err != nil {
		// Preserve stdout/stderr content for debugging if available.
		if exitErr, ok := err.(*exec.ExitError); ok {
			out = append(out, exitErr.Stderr...)
		}
		return out, err
	}
	return out, nil
}

func splitFeatureExpOutput(raw []byte) ([][]string, error) {
	r := csv.NewReader(bytes.NewReader(raw))
	r.Comma = 'Ž'
	r.LazyQuotes = true
	r.Comment = '#'
	records, err := r.ReadAll()
	if err != nil {
		return nil, err
	}

	filtered := make([][]string, 0, len(records))
	seen := make(map[string]int)
	for _, row := range records {
		if len(row) == 0 {
			continue
		}
		key := row[0]
		if count, ok := seen[key]; ok {
			seen[key] = count + 1
			row[0] = strings.TrimSpace(row[0]) + strconv.Itoa(seen[key])
		} else {
			seen[key] = 1
		}
		filtered = append(filtered, row)
	}
	return filtered, nil
}

func parseFeatureExpRecords(records [][]string) []*featureExp {
	features := make([]*featureExp, 0, len(records))
	for _, row := range records {
		if len(row) == 0 {
			continue
		}
		line := strings.Join(row, "")
		matches := lmutilLicenseFeatureExpRegex.FindStringSubmatch(line)
		if matches == nil {
			continue
		}

		expires := parseExpiry(matches[4])
		features = append(features, &featureExp{
			name:     matches[1],
			version:  matches[2],
			licenses: matches[3],
			expires:  expires,
			vendor:   matches[5],
		})
	}
	return features
}

func parseExpiry(raw string) float64 {
	if raw == "" {
		return math.Inf(1)
	}

	if strings.EqualFold(raw, "permanent") || strings.EqualFold(raw, "none") {
		return math.Inf(1)
	}

	parts := strings.Split(raw, "-")
	if len(parts) == 3 {
		day := parts[0]
		month := strings.Title(strings.ToLower(parts[1]))
		year := parts[2]
		if len(day) == 1 {
			day = "0" + day
		}
		if len(year) == 1 {
			year = "000" + year
		}
		if t, err := time.Parse("02-Jan-2006", fmt.Sprintf("%s-%s-%s", day, month, year)); err == nil {
			if t.Unix() <= 0 {
				return math.Inf(1)
			}
			return float64(t.Unix())
		}
	}

	if t, err := time.Parse("Jan 02, 2006", raw); err == nil {
		if t.Unix() <= 0 {
			return math.Inf(1)
		}
		return float64(t.Unix())
	}

	return math.Inf(1)
}

func splitCSVList(value string) []string {
	if value == "" {
		return nil
	}
	parts := strings.Split(value, ",")
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		trimmed := strings.TrimSpace(p)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}

func contains(slice []string, item string) bool {
	for _, v := range slice {
		if v == item {
			return true
		}
	}
	return false
}
