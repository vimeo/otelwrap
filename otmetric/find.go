package otmetric

import (
	"context"
	"fmt"

	"go.opentelemetry.io/otel/attribute"
	otsdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
)

// FindMetricVal returns the current value of a [metricName] from the [ManualReader].
func FindMetricVal[V int64 | float64](ctx context.Context, r *otsdkmetric.ManualReader, metricName string, kvs attribute.Set) (V, error) {
	var zv V
	res := metricdata.ResourceMetrics{}
	if err := r.Collect(ctx, &res); err != nil {
		return zv, err
	}

	for _, sm := range res.ScopeMetrics {
		for _, m := range sm.Metrics {
			if m.Name != metricName {
				continue
			}
			switch vs := m.Data.(type) {
			case metricdata.Gauge[V]:
				for _, d := range vs.DataPoints {
					if !kvs.Equals(&d.Attributes) {
						continue
					}
					return d.Value, nil
				}
			case metricdata.Sum[V]:
				for _, d := range vs.DataPoints {
					if !kvs.Equals(&d.Attributes) {
						continue
					}
					return d.Value, nil
				}
			default:
				// wrong metric-type (or histogram, which we'll have to implement later)
				continue
			}
		}
	}
	return zv, fmt.Errorf("metric %s not found", metricName)
}
