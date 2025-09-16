package otmetric

import (
	"context"
	"testing"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	otsdkmetric "go.opentelemetry.io/otel/sdk/metric"
)

func TestFindMetricVal(t *testing.T) {
	ctx := context.Background()

	r := otsdkmetric.NewManualReader()
	mp := otsdkmetric.NewMeterProvider(otsdkmetric.WithReader(r))

	otM := mp.Meter("foobar")
	attrSet := attribute.NewSet(attribute.String("fizzlebit", "foobar"), attribute.String("ouch", "icky"))

	// read an int64 counter
	counter, countErr := otM.Int64Counter("a_counter_int")
	if countErr != nil {
		t.Fatalf("failed to register counter metric: %s", countErr)
	}
	counter.Add(ctx, 2, metric.WithAttributeSet(attrSet))

	counterVal, counterValErr := FindVal[int64](ctx, r, "a_counter_int", attrSet)
	if counterValErr != nil {
		t.Errorf("failed to fetch counter metric value: %s", counterValErr)
	}
	if counterVal != 2 {
		t.Errorf("unexpected counter metric value: want %d; got %d", 2, counterVal)
	}

	// read an int64 gauge
	gauge, gaugeErr := otM.Int64Gauge("a_gauge_int")
	if gaugeErr != nil {
		t.Fatalf("failed to register gauge metric: %s", gaugeErr)
	}
	gauge.Record(ctx, 3, metric.WithAttributeSet(attrSet))

	gaugeVal, gaugeValErr := FindVal[int64](ctx, r, "a_gauge_int", attrSet)
	if gaugeValErr != nil {
		t.Errorf("failed to fetch gauge metric value: %s", gaugeValErr)
	}
	if gaugeVal != 3 {
		t.Errorf("unexpected gauge metric value: want %d; got %d", 3, gaugeVal)
	}
}
