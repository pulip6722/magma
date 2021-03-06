/*
 * Copyright 2020 The Magma Authors.
 *
 * This source code is licensed under the BSD-style license found in the
 * LICENSE file in the root directory of this source tree.
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package analytics

import (
	"github.com/golang/glog"
	"github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/robfig/cron/v3"
	"magma/cwf/cloud/go/services/analytics/calculations"
	"magma/cwf/cloud/go/services/analytics/query_api"
	"net/http"
)

type Analyzer interface {
	// Schedule the analyzer to run calculations periodically based on the
	// cron expression format schedule parameter
	Schedule(schedule string) error

	// Run triggers the analyzer's cronjob to start running. This function
	// blocks.
	Run()
}

// PrometheusAnalyzer accesses prometheus metrics and performs
// queries/aggregations to calculate various metrics
type PrometheusAnalyzer struct {
	Cron             *cron.Cron
	PrometheusClient query_api.PrometheusAPI
	Calculations     []calculations.Calculation
	Exporter         Exporter
}

func NewPrometheusAnalyzer(prometheusClient v1.API, calculations []calculations.Calculation, exporter Exporter) Analyzer {
	cronJob := cron.New()
	return &PrometheusAnalyzer{
		Cron:             cronJob,
		PrometheusClient: prometheusClient,
		Calculations:     calculations,
		Exporter:         exporter,
	}
}

func (a *PrometheusAnalyzer) Schedule(schedule string) error {
	a.Cron = cron.New()

	_, err := a.Cron.AddFunc(schedule, a.Analyze)
	if err != nil {
		return err
	}
	return nil
}

func (a *PrometheusAnalyzer) Analyze() {
	for _, calc := range a.Calculations {
		results, err := calc.Calculate(a.PrometheusClient)
		if err != nil {
			glog.Errorf("Error calculating metric: %s", err)
			continue
		}
		if a.Exporter == nil {
			continue
		}
		for _, res := range results {
			err = a.Exporter.Export(res, http.DefaultClient)
			if err != nil {
				glog.Errorf("Error exporting result: %v", err)
			} else {
				glog.Infof("Exported %s, %s, %f", res.MetricName(), res.Labels(), res.Value())
			}
		}
	}
}

func (a *PrometheusAnalyzer) Run() {
	a.Cron.Run()
}
