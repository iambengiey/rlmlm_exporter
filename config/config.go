// Package config includes all individual types and functions to gather
// the monitored licences.
// (C) Copyright 2017 Mario Trangoni.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package config

import (
	"errors"
	"os"
	"path/filepath"

	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
	"gopkg.in/yaml.v2"
)

// ---------- Package logger (safe default) ----------
var cfgLogger log.Logger = log.NewNopLogger()

// SetLogger allows main to inject a real logger.
func SetLogger(l log.Logger) { if l != nil { cfgLogger = l } }

// ---------- YAML type definitions ----------

// Licence individual configuration type.
type License struct {
	Name                string `yaml:"name"`
	LicenseFile         string `yaml:"license_file,omitempty"`
	LicenseServer       string `yaml:"license_server,omitempty"`
	FeaturesToExclude   string `yaml:"features_to_exclude,omitempty"`
	FeaturesToInclude   string `yaml:"features_to_include,omitempty"`
	MonitorUsers        bool   `yaml:"monitor_users"`
	MonitorReservations bool   `yaml:"monitor_reservations"`
}

// Configuration type for all licences.
type Config struct {
	Licenses []License `yaml:"licenses"`
}

// Load parses the YAML file at path and returns a Config.
func Load(path string) (*Config, error) {
	if path == "" {
		return nil, errors.New("config path is empty")
	}

	clean := filepath.Clean(path)
	level.Info(cfgLogger).Log("msg", "loading config", "path", clean)

	data, err := os.ReadFile(clean)
	if err != nil {
		level.Error(cfgLogger).Log("msg", "failed to read config file", "path", clean, "err", err)
		return nil, err
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		level.Error
::contentReference[oaicite:0]{index=0}
