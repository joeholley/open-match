/*
Copyright 2018 Google LLC

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    https://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
package mmforc

import (
	"go.opencensus.io/stats"
	"go.opencensus.io/stats/view"
	"go.opencensus.io/tag"
)

// OpenCensus Measures. These are exported as metrics to your monitoring system
// https://godoc.org/go.opencensus.io/stats
//
// When making opencensus stats, the 'name' param, with forward slashes changed
// to underscores, is appended to the 'namespace' value passed to the
// prometheus exporter to become the Prometheus metric name. You can also look
// into having Prometheus rewrite your metric names on scrape.
//
//  For example:
//   - defining the promethus export namespace "open_match" when instanciating the exporter:
//			pe, err := promethus.NewExporter(promethus.Options{Namespace: "open_match"})
//   - and naming the request counter "backend/requests_total":
//			MGrpcRequests := stats.Int64("backendapi/requests_total", ...
//   - results in the prometheus metric name:
//			open_match_backendapi_requests_total
//   - [note] when using opencensus views to aggregate the metrics into
//     distribution buckets and such, multiple metrics
//     will be generated with appended types ("<metric>_bucket",
//     "<metric>_count", "<metric>_sum", for example)
//
// In addition, OpenCensus stats propogated to Prometheus have the following
// auto-populated labels pulled from kubernetes, which we should avoid to
// prevent overloading and having to use the HonorLabels param in Prometheus.
//
// - Information about the k8s pod being monitored:
//		"pod" (name of the monitored k8s pod)
//		"namespace" (k8s namespace of the monitored pod)
// - Information about how promethus is gathering the metrics:
//		"instance" (IP and port number being scraped by prometheus)
//		"job" (name of the k8s service being scraped by prometheus)
//		"endpoint" (name of the k8s port in the k8s service being scraped by prometheus)
//
var (
	// Logging instrumentation
	// There's no need to record this measurement directly if you use
	// the logrus hook provided in metrics/helper.go after instantiating the
	// logrus instance in your application code.
	// https://godoc.org/github.com/sirupsen/logrus#LevelHooks
	MmforcLogLines = stats.Int64("mmforc/logs_total", "Number of MMF Orchestrator lines logged", "1")

	// Counting operations
	mmforcMmfs         = stats.Int64("mmforc/mmfs_total", "Number of  mmf jobs submitted to kubernetes", "1")
	mmforcMmfFailures  = stats.Int64("mmforc/mmf/failures_total", "Number of failures attempting to submit mmf jobs to kubernetes", "1")
	mmforcEvals        = stats.Int64("mmforc/evaluators_total", "Number of  evaluator jobs submitted to kubernetes", "1")
	mmforcEvalFailures = stats.Int64("mmforc/evaluator/failures_total", "Number of failures attempting to submit evaluator jobs to kubernetes", "1")
)

var (
	// KeyEvalReason is used to tag which code path caused the evaluator to run.
	KeyEvalReason, _ = tag.NewKey("evalReason")
	// KeySeverity is used to tag a the severity of a log message.
	KeySeverity, _ = tag.NewKey("severity")
)

var (
	// Latency in buckets:
	// [>=0ms, >=25ms, >=50ms, >=75ms, >=100ms, >=200ms, >=400ms, >=600ms, >=800ms, >=1s, >=2s, >=4s, >=6s]
	latencyDistribution = view.Distribution(0, 25, 50, 75, 100, 200, 400, 600, 800, 1000, 2000, 4000, 6000)
)

// Package metrics provides some convience views.
// You need to register the views for the data to actually be collected.
// Note: The OpenCensus View 'Description' is exported to Prometheus as the HELP string.
// Note: If you get a "Failed to export to Prometheus: inconsistent label
// cardinality" error, chances are you forgot to set the tags specified in the
// view for a given measure when you tried to do a stats.Record()
var (
	mmforcMmfsCountView = &view.View{
		Name:        "mmforc/mmfs",
		Measure:     mmforcMmfs,
		Description: "The number of mmf jobs submitted to kubernetes",
		Aggregation: view.Count(),
	}

	mmforcMmfFailuresCountView = &view.View{
		Name:        "mmforc/mmf/failures",
		Measure:     mmforcMmfFailures,
		Description: "The number of mmf jobs that failed submission to kubernetes",
		Aggregation: view.Count(),
	}

	mmforcEvalsCountView = &view.View{
		Name:        "mmforc/evaluators",
		Measure:     mmforcEvals,
		Description: "The number of evaluator jobs submitted to kubernetes",
		Aggregation: view.Count(),
		TagKeys:     []tag.Key{KeyEvalReason},
	}

	mmforcEvalFailuresCountView = &view.View{
		Name:        "mmforc/evaluator/failures",
		Measure:     mmforcEvalFailures,
		Description: "The number of evaluator jobs that failed submission to kubernetes",
		Aggregation: view.Count(),
		TagKeys:     []tag.Key{KeyEvalReason},
	}
)

// DefaultMmforcViews are the default matchmaker orchestrator OpenCensus measure views.
var DefaultMmforcViews = []*view.View{
	mmforcEvalsCountView,
	mmforcMmfFailuresCountView,
	mmforcMmfsCountView,
	mmforcEvalFailuresCountView,
}
