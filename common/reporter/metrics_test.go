// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package reporter_test

import (
	"math"
	"testing"

	"github.com/prometheus/client_golang/prometheus"

	"akvorado/common/helpers"
	"akvorado/common/reporter"
)

func TestMetrics(t *testing.T) {
	r := reporter.NewMock(t)

	counter1 := r.Counter(reporter.CounterOpts{
		Name: "counter1",
		Help: "Some counter",
	})
	counter1.Add(18)

	r.CounterFunc(reporter.CounterOpts{
		Name: "counter2",
		Help: "Some other counter",
	}, func() float64 { return 1.17 })

	counter3 := r.CounterVec(reporter.CounterOpts{
		Name: "counter3",
		Help: "Another counter",
	}, []string{"label1", "label2"})
	counter3.WithLabelValues("value1", "value2").Add(42)
	counter3.WithLabelValues("value3 space", "value4").Add(167)

	gauge1 := r.Gauge(reporter.GaugeOpts{
		Name: "gauge1",
		Help: "Some gauge",
	})
	gauge1.Set(1717)

	r.GaugeFunc(reporter.GaugeOpts{
		Name: "gauge2",
		Help: "Another gauge",
	}, func() float64 { return 77 })

	gauge3 := r.GaugeVec(reporter.GaugeOpts{
		Name: "gauge3",
		Help: "Another gauge",
	},
		[]string{"label1", "label2"})
	gauge3.WithLabelValues("value1", "value2").Set(44)
	gauge3.WithLabelValues("value3", "value4").Set(48)

	histo1 := r.Histogram(reporter.HistogramOpts{
		Name:    "histo1",
		Help:    "Some histogram",
		Buckets: []float64{0, 1, 2, 10, 100},
	})
	histo1.Observe(5)
	histo1.Observe(6)
	histo1.Observe(1)
	histo1.Observe(5.5)

	histo2 := r.HistogramVec(reporter.HistogramOpts{
		Name:    "histo2",
		Help:    "Another histogram",
		Buckets: []float64{0, 1, 2, 10, 100},
	}, []string{"label"})
	histo2.WithLabelValues("value1").Observe(10)
	histo2.WithLabelValues("value1").Observe(4)
	histo2.WithLabelValues("value1").Observe(5)
	histo2.WithLabelValues("value2").Observe(2)
	histo2.WithLabelValues("value2").Observe(2)
	histo2.WithLabelValues("value2").Observe(2.4)

	summary1 := r.Summary(reporter.SummaryOpts{
		Name:       "summary1",
		Help:       "Some summary",
		Objectives: map[float64]float64{0.5: 0.05, 0.9: 0.01, 0.99: 0.001},
	})
	for i := range 1000 {
		summary1.Observe(30 + math.Floor(120*math.Sin(float64(i)*0.1))/10)
	}

	summary2 := r.SummaryVec(reporter.SummaryOpts{
		Name:       "summary2",
		Help:       "Another summary",
		Objectives: map[float64]float64{0.5: 0.05, 0.9: 0.01, 0.99: 0.001},
	}, []string{"label"})
	for i := range 1000 {
		summary2.WithLabelValues("value1").Observe(10 + math.Floor(120*math.Sin(float64(i)*0.1))/10)
		summary2.WithLabelValues("value2").Observe(15)
	}

	got := r.GetMetrics("akvorado_common_reporter_test_")
	expected := map[string]string{
		`counter1`: "18",
		`counter2`: "1.17",
		`counter3{label1="value1",label2="value2"}`:       "42",
		`counter3{label1="value3 space",label2="value4"}`: "167",
		`gauge1`: "1717",
		`gauge2`: "77",
		`gauge3{label1="value1",label2="value2"}`:  "44",
		`gauge3{label1="value3",label2="value4"}`:  "48",
		`histo1_bucket{le="+Inf"}`:                 "4",
		`histo1_bucket{le="0"}`:                    "0",
		`histo1_bucket{le="1"}`:                    "1",
		`histo1_bucket{le="10"}`:                   "4",
		`histo1_bucket{le="100"}`:                  "4",
		`histo1_bucket{le="2"}`:                    "1",
		`histo1_count`:                             "4",
		`histo1_sum`:                               "17.5",
		`histo2_bucket{label="value1",le="+Inf"}`:  "3",
		`histo2_bucket{label="value1",le="0"}`:     "0",
		`histo2_bucket{label="value1",le="1"}`:     "0",
		`histo2_bucket{label="value1",le="10"}`:    "3",
		`histo2_bucket{label="value1",le="100"}`:   "3",
		`histo2_bucket{label="value1",le="2"}`:     "0",
		`histo2_bucket{label="value2",le="+Inf"}`:  "3",
		`histo2_bucket{label="value2",le="0"}`:     "0",
		`histo2_bucket{label="value2",le="1"}`:     "0",
		`histo2_bucket{label="value2",le="10"}`:    "3",
		`histo2_bucket{label="value2",le="100"}`:   "3",
		`histo2_bucket{label="value2",le="2"}`:     "2",
		`histo2_count{label="value1"}`:             "3",
		`histo2_count{label="value2"}`:             "3",
		`histo2_sum{label="value1"}`:               "19",
		`histo2_sum{label="value2"}`:               "6.4",
		`summary1_count`:                           "1000",
		`summary1_sum`:                             "29969.50000000001",
		`summary1{quantile="0.5"}`:                 "31.1",
		`summary1{quantile="0.9"}`:                 "41.3",
		`summary1{quantile="0.99"}`:                "41.9",
		`summary2_count{label="value1"}`:           "1000",
		`summary2_count{label="value2"}`:           "1000",
		`summary2_sum{label="value1"}`:             "9969.499999999998",
		`summary2_sum{label="value2"}`:             "15000",
		`summary2{label="value1",quantile="0.5"}`:  "11.1",
		`summary2{label="value1",quantile="0.9"}`:  "21.3",
		`summary2{label="value1",quantile="0.99"}`: "21.9",
		`summary2{label="value2",quantile="0.5"}`:  "15",
		`summary2{label="value2",quantile="0.9"}`:  "15",
		`summary2{label="value2",quantile="0.99"}`: "15",
	}
	if diff := helpers.Diff(got, expected); diff != "" {
		t.Fatalf("metrics (-got, +want):\n%s", diff)
	}

	got = r.GetMetrics("akvorado_common_reporter_test_",
		"counter1", "counter2", "counter3")
	expected = map[string]string{
		`counter1`: "18",
		`counter2`: "1.17",
		`counter3{label1="value1",label2="value2"}`:       "42",
		`counter3{label1="value3 space",label2="value4"}`: "167",
	}
	if diff := helpers.Diff(got, expected); diff != "" {
		t.Fatalf("subsetted metrics (-got, +want):\n%s", diff)
	}
}

type customMetrics struct {
	metric1 *reporter.MetricDesc
	metric2 *reporter.MetricDesc
}

func (m customMetrics) Describe(ch chan<- *prometheus.Desc) {
	ch <- m.metric1
	ch <- m.metric2
}

func (m customMetrics) Collect(ch chan<- prometheus.Metric) {
	ch <- prometheus.MustNewConstMetric(m.metric1, prometheus.GaugeValue, 18)
	ch <- prometheus.MustNewConstMetric(m.metric2, prometheus.GaugeValue, 30)
}

func TestMetricCollector(t *testing.T) {
	r := reporter.NewMock(t)

	m := customMetrics{}
	m.metric1 = prometheus.NewDesc("metric1", "Custom metric 1", nil, nil)
	m.metric2 = prometheus.NewDesc("metric2", "Custom metric 2", nil, nil)
	r.RegisterMetricCollector(m)

	got := r.GetMetrics("akvorado_common_reporter_test_")
	expected := map[string]string{
		`metric1`: "18",
		`metric2`: "30",
	}
	if diff := helpers.Diff(got, expected); diff != "" {
		t.Fatalf("collected metrics (-got, +want):\n%s", diff)
	}

	r.UnregisterMetricCollector(m)
	got = r.GetMetrics("akvorado_common_reporter_test_")
	if diff := helpers.Diff(got, map[string]string{}); diff != "" {
		t.Fatalf("collected metrics (-got, +want):\n%s", diff)
	}

	r.RegisterMetricCollector(m)
	got = r.GetMetrics("akvorado_common_reporter_test_")
	if diff := helpers.Diff(got, expected); diff != "" {
		t.Fatalf("collected metrics (-got, +want):\n%s", diff)
	}
}
