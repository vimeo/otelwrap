package otmetric

import (
	"context"
	"fmt"

	"go.opentelemetry.io/otel/attribute"
	otsdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
)

// FindVal is a testing utility to find the value of a specific metric
// collected by the sdk [metric.ManualReader].
//
// It searches for a metric matching the provided metricName and attribute.Set.
// The type parameter V specifies the expected value type, which must be
// either int64 or float64.
//
// An error is returned if a metric matching the name and attributes is not found,
// or if the found metric's value type does not match V.
func FindVal[V int64 | float64](ctx context.Context, r *otsdkmetric.ManualReader, metricName string, kvs attribute.Set) (V, error) {
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
