package promhttp

import (
	"fmt"
	"log"
	"net/http"
	"sort"
	"strings"

	"github.com/prometheus/client_golang/prometheus"
)

type HandlerErrorHandling int

const (
	HTTPErrorOnError HandlerErrorHandling = iota
	ContinueOnError
)

type HandlerOpts struct {
	ErrorLog      *log.Logger
	ErrorHandling HandlerErrorHandling
}

func HandlerFor(g prometheus.Gatherer, opts HandlerOpts) http.Handler {
	if g == nil {
		g = prometheus.DefaultGatherer
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		families, err := g.Gather()
		if err != nil {
			handleError(w, err, opts)
			return
		}
		writeFamilies(w, families)
	})
}

func handleError(w http.ResponseWriter, err error, opts HandlerOpts) {
	if opts.ErrorLog != nil {
		opts.ErrorLog.Println("promhttp: error gathering metrics:", err)
	}
	if opts.ErrorHandling == ContinueOnError {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, "# error: %v\n", err)
		return
	}
	http.Error(w, err.Error(), http.StatusInternalServerError)
}

func writeFamilies(w http.ResponseWriter, families []*prometheus.MetricFamily) {
	w.Header().Set("Content-Type", "text/plain; version=0.0.4")
	for _, fam := range families {
		fmt.Fprintf(w, "# HELP %s %s\n", fam.Name, sanitize(fam.Help))
		fmt.Fprintf(w, "# TYPE %s %s\n", fam.Name, fam.Type)
		sort.Slice(fam.Metrics, func(i, j int) bool {
			return labelSignature(fam.Metrics[i].Labels) < labelSignature(fam.Metrics[j].Labels)
		})
		for _, metric := range fam.Metrics {
			fmt.Fprintf(w, "%s%s %v\n", fam.Name, formatLabels(metric.Labels), metric.Value)
		}
	}
}

func sanitize(help string) string {
	help = strings.ReplaceAll(help, "\n", " ")
	help = strings.ReplaceAll(help, "\r", " ")
	return help
}

func formatLabels(labels map[string]string) string {
	if len(labels) == 0 {
		return ""
	}
	keys := make([]string, 0, len(labels))
	for k := range labels {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	pairs := make([]string, 0, len(keys))
	for _, k := range keys {
		pairs = append(pairs, fmt.Sprintf("%s=\"%s\"", k, escapeLabel(labels[k])))
	}
	return fmt.Sprintf("{%s}", strings.Join(pairs, ","))
}

func escapeLabel(v string) string {
	v = strings.ReplaceAll(v, "\\", "\\\\")
	v = strings.ReplaceAll(v, "\n", "\\n")
	v = strings.ReplaceAll(v, "\"", "\\\"")
	return v
}

func labelSignature(labels map[string]string) string {
	keys := make([]string, 0, len(labels))
	for k := range labels {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, k+"="+labels[k])
	}
	return strings.Join(parts, ",")
}
