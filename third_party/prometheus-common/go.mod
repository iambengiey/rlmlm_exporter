module github.com/prometheus/common

go 1.22

require (
        github.com/alecthomas/kingpin/v2 v2.0.0
        github.com/go-kit/log v0.0.0
)

replace github.com/alecthomas/kingpin/v2 => ../kingpin
replace github.com/go-kit/log => ../go-kit-log
