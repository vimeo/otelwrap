package otregistrar

import (
	"context"
	"testing"

	"go.opentelemetry.io/otel/metric"
	otsdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
)

func TestRegisterIntMetricsSuccess(t *testing.T) {
	r := otsdkmetric.NewManualReader()
	mp := otsdkmetric.NewMeterProvider(otsdkmetric.WithReader(r))

	m := mp.Meter("fizzlebat")

	type deeperStruct struct {
		IntUpDownCounterPathCB metric.Int64ObservableUpDownCounter `otelname:"ijklm.fizzlebat"` // no description or unit
	}

	type anyStruct struct {
		IntUpDownCounterPathCB metric.Int64ObservableUpDownCounter `otelname:"ijklm.fizzlebat.any"` // no description or unit
	}

	ch := make(chan struct{})
	s := struct {
		unexportedThing        struct{}
		unexportedInt          int
		ExportedButSkippedInt  int
		IntCounter             metric.Int64Counter                 `otelname:"abcde.defg" oteldesc:"fizzlebat fizzle boo" otelunit:"s"`
		IntUpDownCounter       metric.Int64UpDownCounter           `otelname:"abcde.defg.updown" oteldesc:"fizzlebat fizzle boo, but updown" otelunit:"s"`
		IntGauge               metric.Int64Gauge                   `otelname:"abcde.defg.gauge" oteldesc:"foobar gauge" otelunit:"By"`
		IntHistogramNoBounds   metric.Int64Histogram               `otelname:"abcde.defg.histogram.nobounds" oteldesc:"foobar histogram" otelunit:"By/s"`
		IntHistogramBounds     metric.Int64Histogram               `otelname:"abcde.defg.histogram.bounds" oteldesc:"foobar histogram" otelunit:"By/s" otelhistbounds:"intbounds"`
		IntCounterCB           metric.Int64ObservableCounter       `otelname:"abcde.defg.callback" oteldesc:"foobar counter callback" otelunit:"s/s" otelcallback:"fooblebit"`
		IntUpDownCounterCB     metric.Int64ObservableUpDownCounter `otelname:"abcde.defg.updown.callback" oteldesc:"foobar updown counter callback" otelunit:"s/s" otelcallback:"seven"`
		IntCounterCBNoCB       metric.Int64ObservableCounter       `otelname:"abcde.defg.callback.nocb" oteldesc:"foobar counter callback" otelunit:"s/s"`
		IntUpDownCounterCBNoCB metric.Int64ObservableUpDownCounter `otelname:"abcde.defg.updown.callback.nocb" oteldesc:"foobar updown counter callback" otelunit:"s/s"`
		IntGaugeCB             metric.Int64ObservableGauge         `otelname:"abcde.defg.gauge.cb" oteldesc:"foobar gauge callback"` // no unit
		Deep                   *deeperStruct
		DeepInterface          any
		ChanPtr                *chan struct{}
	}{Deep: &deeperStruct{}, DeepInterface: &anyStruct{}, ChanPtr: &ch}

	if regErr := RegisterInstruments(m, Options{
		HistogramBounds: map[string][]float64{
			"intbounds": {0, 1, 2, 3, 5, 4, 6},
		},
		IntCallbacks: map[string][]metric.Int64Callback{
			"fooblebit": {func(ctx context.Context, obs metric.Int64Observer) error {
				obs.Observe(6)
				return nil
			}},
			"seven": {func(ctx context.Context, obs metric.Int64Observer) error {
				obs.Observe(7)
				return nil
			}},
			"Deep.IntUpDownCounterPathCB": {func(ctx context.Context, obs metric.Int64Observer) error {
				obs.Observe(8)
				return nil
			}},
			"IntGaugeCB": {func(ctx context.Context, obs metric.Int64Observer) error {
				obs.Observe(9)
				return nil
			}},
			"DeepInterface.IntUpDownCounterPathCB": {func(ctx context.Context, obs metric.Int64Observer) error {
				obs.Observe(10)
				return nil
			}},
		},
	}, &s); regErr != nil {
		t.Errorf("failed to register instruments: %s", regErr)
		return
	}
	t.Logf("got %+v", s)
	ctx := t.Context()
	// Record something so the metrics show up
	s.IntCounter.Add(ctx, 1)
	s.IntUpDownCounter.Add(ctx, 2)
	s.IntGauge.Record(ctx, 3)
	s.IntHistogramNoBounds.Record(ctx, 4)
	s.IntHistogramBounds.Record(ctx, 5)

	rm := metricdata.ResourceMetrics{}
	if collectErr := r.Collect(ctx, &rm); collectErr != nil {
		t.Fatalf("failed to collect metrics: %s", collectErr)
	}
	expVals := map[string]int64{
		"abcde.defg":                    1,
		"abcde.defg.updown":             2,
		"abcde.defg.gauge":              3,
		"abcde.defg.histogram.nobounds": 4,
		"abcde.defg.histogram.bounds":   5,
		"abcde.defg.callback":           6,
		"abcde.defg.updown.callback":    7,
		// "abcde.defg.callback.nocb" : 0, // not registered
		// "abcde.defg.updown.callback.nocb": 0, // not registered
		"abcde.defg.gauge.cb": 9,

		// inner
		"ijklm.fizzlebat":     8,
		"ijklm.fizzlebat.any": 10,
	}
	for _, scope := range rm.ScopeMetrics {
		for _, m := range scope.Metrics {
			exp, expOK := expVals[m.Name]
			if !expOK {
				t.Errorf("unexpected metric/instrument %q", m.Name)
				continue
			}
			switch d := m.Data.(type) {
			case metricdata.Sum[int64]:
				// we're only recording once
				if len(d.DataPoints) != 1 {
					t.Errorf("metric %q has %d datapoints; expected 1", m.Name, len(d.DataPoints))
					continue
				}
				for _, dp := range d.DataPoints {
					if dp.Value != exp {
						t.Errorf("unexpected value for metric %q: %d (expected %d)", m.Name, dp.Value, exp)
					}
				}
			case metricdata.Gauge[int64]:
				// we're only recording once
				if len(d.DataPoints) != 1 {
					t.Errorf("metric %q has %d datapoints; expected 1", m.Name, len(d.DataPoints))
					continue
				}
				for _, dp := range d.DataPoints {
					if dp.Value != exp {
						t.Errorf("unexpected value for metric %q: %d (expected %d)", m.Name, dp.Value, exp)
					}
				}
			case metricdata.ExponentialHistogram[int64]:
				// we're only recording once
				if len(d.DataPoints) != 1 {
					t.Errorf("metric %q has %d datapoints; expected 1", m.Name, len(d.DataPoints))
					continue
				}
				for _, dp := range d.DataPoints {
					if dp.Sum != exp {
						t.Errorf("unexpected sum for histogram metric %q: %d (expected %d)", m.Name, dp.Sum, exp)
					}
					if dp.Count != 1 {
						t.Errorf("unexpected count for histogram metric %q: %d (expected %d)", m.Name, dp.Count, 1)
					}
				}
			case metricdata.Histogram[int64]:
				// we're only recording once
				if len(d.DataPoints) != 1 {
					t.Errorf("metric %q has %d datapoints; expected 1", m.Name, len(d.DataPoints))
					continue
				}
				for _, dp := range d.DataPoints {
					if dp.Sum != exp {
						t.Errorf("unexpected sum for histogram metric %q: %d (expected %d)", m.Name, dp.Sum, exp)
					}
					if dp.Count != 1 {
						t.Errorf("unexpected count for histogram metric %q: %d (expected %d)", m.Name, dp.Count, 1)
					}
				}
			default:
				t.Errorf("unexpected data type for scalar values of type %T on metric %q: got data type %T", int64(0), m.Name, m.Data)
			}
		}
	}
}

func TestRegisterIntMetricsNoTags(t *testing.T) {
	r := otsdkmetric.NewManualReader()
	mp := otsdkmetric.NewMeterProvider(otsdkmetric.WithReader(r))

	m := mp.Meter("fizzlebat")

	s := struct {
		unexportedThing        struct{}
		unexportedInt          int
		ExportedButSkippedInt  int
		IntCounter             metric.Int64Counter
		IntUpDownCounter       metric.Int64UpDownCounter
		IntGauge               metric.Int64Gauge
		IntHistogramNoBounds   metric.Int64Histogram
		IntHistogramBounds     metric.Int64Histogram
		IntCounterCB           metric.Int64ObservableCounter
		IntUpDownCounterCB     metric.Int64ObservableUpDownCounter
		IntCounterCBNoCB       metric.Int64ObservableCounter
		IntUpDownCounterCBNoCB metric.Int64ObservableUpDownCounter
		IntGaugeCB             metric.Int64ObservableGauge
	}{}

	if regErr := RegisterInstruments(m, Options{
		HistogramBounds: map[string][]float64{
			"intbounds": {0, 1, 2, 3, 5, 4, 6},
		},
		IntCallbacks: map[string][]metric.Int64Callback{
			"fooblebit": {func(ctx context.Context, obs metric.Int64Observer) error {
				obs.Observe(6)
				return nil
			}},
			"seven": {func(ctx context.Context, obs metric.Int64Observer) error {
				obs.Observe(7)
				return nil
			}},
			"Deep.IntUpDownCounterPathCB": {func(ctx context.Context, obs metric.Int64Observer) error {
				obs.Observe(8)
				return nil
			}},
		},
	}, &s); regErr != nil {
		t.Errorf("failed to register instruments: %s", regErr)
		return
	}
}
func TestRegisterFloatMetricsSuccess(t *testing.T) {
	r := otsdkmetric.NewManualReader()
	mp := otsdkmetric.NewMeterProvider(otsdkmetric.WithReader(r))

	m := mp.Meter("fizzlebat")

	type deeperStruct struct {
		FloatUpDownCounterPathCB metric.Float64ObservableUpDownCounter `otelname:"ijklm.fizzlebat"` // no description or unit
	}

	s := struct {
		unexportedThing          struct{}
		unexportedFloat          float32
		ExportedButSkippedFloat  float64
		FloatCounter             metric.Float64Counter                 `otelname:"abcde.defg" oteldesc:"fizzlebat fizzle boo" otelunit:"s"`
		FloatUpDownCounter       metric.Float64UpDownCounter           `otelname:"abcde.defg.updown" oteldesc:"fizzlebat fizzle boo, but updown" otelunit:"s"`
		FloatGauge               metric.Float64Gauge                   `otelname:"abcde.defg.gauge" oteldesc:"foobar gauge" otelunit:"By"`
		FloatHistogramNoBounds   metric.Float64Histogram               `otelname:"abcde.defg.histogram.nobounds" oteldesc:"foobar histogram" otelunit:"By/s"`
		FloatHistogramBounds     metric.Float64Histogram               `otelname:"abcde.defg.histogram.bounds" oteldesc:"foobar histogram" otelunit:"By/s" otelhistbounds:"floatbounds"`
		FloatCounterCB           metric.Float64ObservableCounter       `otelname:"abcde.defg.callback" oteldesc:"foobar counter callback" otelunit:"s/s" otelcallback:"fooblebit"`
		FloatUpDownCounterCB     metric.Float64ObservableUpDownCounter `otelname:"abcde.defg.updown.callback" oteldesc:"foobar updown counter callback" otelunit:"s/s" otelcallback:"seven"`
		FloatCounterCBNoCB       metric.Float64ObservableCounter       `otelname:"abcde.defg.callback.nocb" oteldesc:"foobar counter callback" otelunit:"s/s"`
		FloatUpDownCounterCBNoCB metric.Float64ObservableUpDownCounter `otelname:"abcde.defg.updown.callback.nocb" oteldesc:"foobar updown counter callback" otelunit:"s/s"`
		FloatGaugeCB             metric.Float64ObservableGauge         `otelname:"abcde.defg.gauge.cb" oteldesc:"foobar gauge callback"` // no unit
		Deep                     deeperStruct
	}{}

	if regErr := RegisterInstruments(m, Options{
		HistogramBounds: map[string][]float64{
			"floatbounds": {0, 1, 2, 3, 5, 4, 6},
		},
		FloatCallbacks: map[string][]metric.Float64Callback{
			"fooblebit": {func(ctx context.Context, obs metric.Float64Observer) error {
				obs.Observe(6)
				return nil
			}},
			"seven": {func(ctx context.Context, obs metric.Float64Observer) error {
				obs.Observe(7)
				return nil
			}},
			"Deep.FloatUpDownCounterPathCB": {func(ctx context.Context, obs metric.Float64Observer) error {
				obs.Observe(8)
				return nil
			}},
			"FloatGaugeCB": {func(ctx context.Context, obs metric.Float64Observer) error {
				obs.Observe(9)
				return nil
			}},
		},
	}, &s); regErr != nil {
		t.Errorf("failed to register instruments: %s", regErr)
		return
	}
	t.Logf("got %+v", s)
	ctx := t.Context()
	// Record something so the metrics show up
	s.FloatCounter.Add(ctx, 1)
	s.FloatUpDownCounter.Add(ctx, 2)
	s.FloatGauge.Record(ctx, 3)
	s.FloatHistogramNoBounds.Record(ctx, 4)
	s.FloatHistogramBounds.Record(ctx, 5)

	rm := metricdata.ResourceMetrics{}
	if collectErr := r.Collect(ctx, &rm); collectErr != nil {
		t.Fatalf("failed to collect metrics: %s", collectErr)
	}
	expVals := map[string]float64{
		"abcde.defg":                    1,
		"abcde.defg.updown":             2,
		"abcde.defg.gauge":              3,
		"abcde.defg.histogram.nobounds": 4,
		"abcde.defg.histogram.bounds":   5,
		"abcde.defg.callback":           6,
		"abcde.defg.updown.callback":    7,
		// "abcde.defg.callback.nocb" : 0, // not registered
		// "abcde.defg.updown.callback.nocb": 0, // not registered
		"abcde.defg.gauge.cb": 9,

		// inner
		"ijklm.fizzlebat": 8,
	}
	for _, scope := range rm.ScopeMetrics {
		for _, m := range scope.Metrics {
			exp, expOK := expVals[m.Name]
			if !expOK {
				t.Errorf("unexpected metric/instrument %q", m.Name)
				continue
			}
			switch d := m.Data.(type) {
			case metricdata.Sum[float64]:
				// we're only recording once
				if len(d.DataPoints) != 1 {
					t.Errorf("metric %q has %d datapoints; expected 1", m.Name, len(d.DataPoints))
					continue
				}
				for _, dp := range d.DataPoints {
					if dp.Value != exp {
						t.Errorf("unexpected value for metric %q: %g (expected %g)", m.Name, dp.Value, exp)
					}
				}
			case metricdata.Gauge[float64]:
				// we're only recording once
				if len(d.DataPoints) != 1 {
					t.Errorf("metric %q has %d datapoints; expected 1", m.Name, len(d.DataPoints))
					continue
				}
				for _, dp := range d.DataPoints {
					if dp.Value != exp {
						t.Errorf("unexpected value for metric %q: %g (expected %g)", m.Name, dp.Value, exp)
					}
				}
			case metricdata.ExponentialHistogram[float64]:
				// we're only recording once
				if len(d.DataPoints) != 1 {
					t.Errorf("metric %q has %d datapoints; expected 1", m.Name, len(d.DataPoints))
					continue
				}
				for _, dp := range d.DataPoints {
					if dp.Sum != exp {
						t.Errorf("unexpected sum for histogram metric %q: %g (expected %g)", m.Name, dp.Sum, exp)
					}
					if dp.Count != 1 {
						t.Errorf("unexpected count for histogram metric %q: %d (expected %d)", m.Name, dp.Count, 1)
					}
				}
			case metricdata.Histogram[float64]:
				// we're only recording once
				if len(d.DataPoints) != 1 {
					t.Errorf("metric %q has %d datapoints; expected 1", m.Name, len(d.DataPoints))
					continue
				}
				for _, dp := range d.DataPoints {
					if dp.Sum != exp {
						t.Errorf("unexpected sum for histogram metric %q: %g (expected %g)", m.Name, dp.Sum, exp)
					}
					if dp.Count != 1 {
						t.Errorf("unexpected count for histogram metric %q: %d (expected %d)", m.Name, dp.Count, 1)
					}
				}
			default:
				t.Errorf("unexpected data type for scalar values of type %T on metric %q: got data type %T", float64(0), m.Name, m.Data)
			}
		}
	}
}

func TestRegisterFloatMetricsNoTags(t *testing.T) {
	r := otsdkmetric.NewManualReader()
	mp := otsdkmetric.NewMeterProvider(otsdkmetric.WithReader(r))

	m := mp.Meter("fizzlebat")

	s := struct {
		unexportedThing          struct{}
		unexportedFloat          float32
		ExportedButSkippedFloat  int
		FloatCounter             metric.Float64Counter
		FloatUpDownCounter       metric.Float64UpDownCounter
		FloatGauge               metric.Float64Gauge
		FloatHistogramNoBounds   metric.Float64Histogram
		FloatHistogramBounds     metric.Float64Histogram
		FloatCounterCB           metric.Float64ObservableCounter
		FloatUpDownCounterCB     metric.Float64ObservableUpDownCounter
		FloatCounterCBNoCB       metric.Float64ObservableCounter
		FloatUpDownCounterCBNoCB metric.Float64ObservableUpDownCounter
		FloatGaugeCB             metric.Float64ObservableGauge
	}{}

	if regErr := RegisterInstruments(m, Options{}, &s); regErr != nil {
		t.Errorf("failed to register instruments: %s", regErr)
		return
	}
}
