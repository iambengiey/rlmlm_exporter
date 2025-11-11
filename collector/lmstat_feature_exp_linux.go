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

func (c *lmstatFeatureExpCollector) collectFeatureExpForLicense(ch chan<- prometheus.Metric, license config.License) error {
	if license.LicenseFile == "" && license.LicenseServer == "" {
		err := errors.New("missing license_file or license_server")
		level.Error(c.logger).Log("msg", "skipping license without target", "license", license.Name, "err", err)
		return err
	}

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

	output, err := runRlmstatCommand(args...)
	if err != nil {
		if len(output) == 0 {
			level.Error(c.logger).Log("msg", "failed to execute rlmstat", "license", license.Name, "target", target, "err", err)
			return err
		}
		level.Warn(c.logger).Log("msg", "rlmstat exited with error but produced output", "license", license.Name, "target", target, "err", err)
	}

	records, err := splitFeatureExpOutput(output)
	if err != nil {
		level.Error(c.logger).Log("msg", "failed to split rlmstat output", "license", license.Name, "err", err)
		return err
	}

	features := parseFeatureExpRecords(records)
	featuresToExclude := splitCSVList(license.FeaturesToExclude)
	featuresToInclude := splitCSVList(license.FeaturesToInclude)

	for idx, feature := range features {
		if contains(featuresToExclude, feature.name) {
			continue
		}
		if len(featuresToInclude) > 0 && !contains(featuresToInclude, feature.name) {
			continue
		}

		ch <- prometheus.MustNewConstMetric(
			c.lmstatFeatureExp,
			prometheus.GaugeValue,
			feature.expires,
			license.Name,
			feature.name,
			strconv.Itoa(idx+1),
			feature.licenses,
			feature.vendor,
			feature.version,
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
		for i := range row {
			row[i] = strings.TrimSpace(row[i])
		}
		key := row[0]
		if count, ok := seen[key]; ok {
			seen[key] = count + 1
			row[0] = key + strconv.Itoa(seen[key])
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
			name:     strings.TrimSpace(matches[1]),
			version:  strings.TrimSpace(matches[2]),
			licenses: strings.TrimSpace(matches[3]),
			expires:  expires,
			vendor:   strings.TrimSpace(matches[5]),
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
