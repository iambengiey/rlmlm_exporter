module github.com/iambengiey/rlmlm_exporter

go 1.22

require (
        github.com/alecthomas/kingpin/v2 v2.0.0
        github.com/go-kit/log v0.0.0
        github.com/prometheus/client_golang v0.0.0
        github.com/prometheus/common v0.0.0
        gopkg.in/yaml.v2 v2.0.0
)

replace github.com/alecthomas/kingpin/v2 => ./third_party/kingpin
replace github.com/go-kit/log => ./third_party/go-kit-log
replace github.com/prometheus/client_golang => ./third_party/prometheus-client
replace github.com/prometheus/common => ./third_party/prometheus-common
replace gopkg.in/yaml.v2 => ./third_party/yaml
