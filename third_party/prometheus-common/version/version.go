package version

import (
	"fmt"
	"runtime"
	"strings"

	"github.com/prometheus/client_golang/prometheus"
)

var (
	Version   = "unknown"
	Revision  = "unknown"
	Branch    = "unknown"
	BuildUser = "unknown"
	BuildDate = "unknown"
	GoVersion = runtime.Version()
)

func NewCollector(name string) prometheus.Collector {
	desc := prometheus.NewDesc(
		fmt.Sprintf("%s_build_info", name),
		"A metric with a constant '1' value labeled by version information.",
		[]string{"version", "revision", "branch", "goversion"},
		nil,
	)
	return &versionCollector{desc: desc}
}

type versionCollector struct {
	desc *prometheus.Desc
}

func (c *versionCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.desc
}

func (c *versionCollector) Collect(ch chan<- prometheus.Metric) {
	ch <- prometheus.MustNewConstMetric(c.desc, prometheus.GaugeValue, 1, Version, Revision, Branch, GoVersion)
}

func Info() string {
	parts := []string{
		fmt.Sprintf("version=%s", valueOrUnknown(Version)),
		fmt.Sprintf("revision=%s", valueOrUnknown(Revision)),
		fmt.Sprintf("branch=%s", valueOrUnknown(Branch)),
	}
	return strings.Join(parts, " ")
}

func BuildContext() string {
	parts := []string{
		fmt.Sprintf("build_user=%s", valueOrUnknown(BuildUser)),
		fmt.Sprintf("build_date=%s", valueOrUnknown(BuildDate)),
		fmt.Sprintf("go_version=%s", valueOrUnknown(GoVersion)),
	}
	return strings.Join(parts, " ")
}

func Print(app string) string {
	return fmt.Sprintf("%s, version %s (branch: %s, revision: %s)", app, valueOrUnknown(Version), valueOrUnknown(Branch), valueOrUnknown(Revision))
}

func valueOrUnknown(v string) string {
	if v == "" {
		return "unknown"
	}
	return v
}
