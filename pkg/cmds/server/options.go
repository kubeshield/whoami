/*
Copyright AppsCode Inc. and Contributors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package server

import (
	"kubeops.dev/ui-server/pkg/apiserver"

	"github.com/spf13/pflag"
)

type ExtraOptions struct {
	QPS   float64
	Burst int

	TelemetryHost string
	TelemetryPort int
}

func NewExtraOptions() *ExtraOptions {
	return &ExtraOptions{
		QPS:           1e6,
		Burst:         1e6,
		TelemetryHost: "::",
		TelemetryPort: 8081,
	}
}

func (s *ExtraOptions) AddFlags(fs *pflag.FlagSet) {
	fs.Float64Var(&s.QPS, "qps", s.QPS, "The maximum QPS to the master from this client")
	fs.IntVar(&s.Burst, "burst", s.Burst, "The maximum burst for throttle")

	fs.StringVar(&s.TelemetryHost, "telemetry-host", s.TelemetryHost, `Host to expose self metrics on.`)
	fs.IntVar(&s.TelemetryPort, "telemetry-port", s.TelemetryPort, `Port to expose self metrics on.`)
}

func (s *ExtraOptions) ApplyTo(cfg *apiserver.ExtraConfig) error {
	cfg.ClientConfig.QPS = float32(s.QPS)
	cfg.ClientConfig.Burst = s.Burst

	return nil
}
