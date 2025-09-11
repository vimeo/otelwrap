// Package otregistrar provides functionality for batch-registering metrics/instruments with an Opentelemetry [metric.Meter].
package otregistrar

import (
	"fmt"
	"maps"
	"reflect"
	"slices"

	"go.opentelemetry.io/otel/metric"
)

const (
	metricNameTag        = "otelname"
	metricDescriptionTag = "oteldesc"
	metricUnitTag        = "otelunit"
	histogramBoundRefTag = "otelhistbounds"
	callbackRefTag       = "otelcallback"
)

// Options defines parameters for [RegisterInstruments], providing information that cannot be supplied directly by
// struct-tags.
type Options struct {
	// registry of histogram bounds for both int and float histograms
	// referenced by `otelhistbounds` tags on metric-fields.
	//
	// NOTE: [RegisterInstruments] will return an error if a histogram-typed field has a `otelhistbounds` tag and
	// there is no matching entry in this map.
	HistogramBounds map[string][]float64

	// Callbacks for integer callbacks default lookup is by the field name, but that can be overridden with `otelcallback`
	// When recursing into inner-struct fields, fields-names or `otelcallback` values must be
	// prefixed with the dot-delimited path to that field (possibly
	// substituted by any `otelcallback` values on the parent field)
	//
	// Field-names must be an exact string-match.
	//
	// One can also register callbacks later with [metric.Meter.RegisterCallback] on the meter.
	IntCallbacks map[string][]metric.Int64Callback

	// Callbacks for float callbacks default lookup is by the field name, but that can be overridden with `otelcallback`
	// When recursing into inner-struct fields, fields-names or `otelcallback` values must be
	// prefixed with the dot-delimited path to that field (possibly
	// substituted by any `otelcallback` values on the parent field)
	//
	// Field-names must be an exact string-match.
	//
	// One can also register callbacks later with [metric.Meter.RegisterCallback] on the meter.
	FloatCallbacks map[string][]metric.Float64Callback
}

// RegisterInstruments registers metrics based on the otelname, oteldesc, and
// otelunit tags on the fields of the passed struct.
func RegisterInstruments[T any](m metric.Meter, opts Options, s *T) error {
	rv := reflect.ValueOf(s).Elem()

	return registerInstruments(m, rv, "", opts)
}

func registerInstruments(m metric.Meter, rv reflect.Value, pathPrefix string, opts Options) error {
	rt := rv.Type()
	for i := range rv.Type().NumField() {
		ft := rt.Field(i)
		if !ft.IsExported() {
			// we can't do anything with unexported fields
			continue
		}
		f := rv.Field(i)
		// skip anything that's not a pointer, struct or interface
		// (instrument types are all interfaces as far as we're concerned)
		switch f.Kind() {
		case reflect.Struct, reflect.Pointer, reflect.Interface:
		default:
			continue
		}
		metricName, metricNameOK := ft.Tag.Lookup(metricNameTag)
		genericOpts := make([]metric.InstrumentOption, 0, 2)
		if desc, descOK := ft.Tag.Lookup(metricDescriptionTag); descOK {
			genericOpts = append(genericOpts, metric.WithDescription(desc))
		}
		if unit, unitOK := ft.Tag.Lookup(metricUnitTag); unitOK {
			genericOpts = append(genericOpts, metric.WithUnit(unit))
		}
		histBoundName, histBoundOK := ft.Tag.Lookup(histogramBoundRefTag)

		fPathComponent := func() string {
			if cbRT, ok := ft.Tag.Lookup(callbackRefTag); ok {
				return cbRT
			}
			return ft.Name
		}

		fi := f.Addr().Interface()
		switch t := fi.(type) {
		case *metric.Int64Counter:
			if !metricNameOK {
				continue
			}
			mOpts := make([]metric.Int64CounterOption, len(genericOpts))
			for i, opt := range genericOpts {
				mOpts[i] = opt
			}
			mt, mtErr := m.Int64Counter(metricName, mOpts...)
			if mtErr != nil {
				return fmt.Errorf("failed to register metric %q for field %q (idx %d): %w", metricName, ft.Name, i, mtErr)
			}
			*t = mt
		case *metric.Int64UpDownCounter:
			if !metricNameOK {
				continue
			}
			mOpts := make([]metric.Int64UpDownCounterOption, len(genericOpts))
			for i, opt := range genericOpts {
				mOpts[i] = opt
			}
			mt, mtErr := m.Int64UpDownCounter(metricName, mOpts...)
			if mtErr != nil {
				return fmt.Errorf("failed to register metric %q for field %q (idx %d): %w", metricName, ft.Name, i, mtErr)
			}
			*t = mt
		case *metric.Int64Histogram:
			if !metricNameOK {
				continue
			}
			mOpts := make([]metric.Int64HistogramOption, len(genericOpts), len(genericOpts)+1)
			for i, opt := range genericOpts {
				mOpts[i] = opt
			}
			if histBoundOK && opts.HistogramBounds != nil {
				// TODO: add a way to make this map lookup for histogram bounds besteffort
				if bounds, boundsOK := opts.HistogramBounds[histBoundName]; boundsOK {
					mOpts = append(mOpts, metric.WithExplicitBucketBoundaries(slices.Sorted(slices.Values(bounds))...))
				} else {
					return fmt.Errorf("field %q has the %q tag set to %q, but no such entry exists in the bounds map; keys: %v",
						ft.Name, histogramBoundRefTag, histBoundName, slices.Collect(maps.Keys(opts.HistogramBounds)))
				}
				// TODO: add fallbacks based on the unit and type
			}
			mt, mtErr := m.Int64Histogram(metricName, mOpts...)
			if mtErr != nil {
				return fmt.Errorf("failed to register metric %q for field %q (idx %d): %w", metricName, ft.Name, i, mtErr)
			}
			*t = mt
		case *metric.Int64Gauge:
			if !metricNameOK {
				continue
			}
			mOpts := make([]metric.Int64GaugeOption, len(genericOpts))
			for i, opt := range genericOpts {
				mOpts[i] = opt
			}
			mt, mtErr := m.Int64Gauge(metricName, mOpts...)
			if mtErr != nil {
				return fmt.Errorf("failed to register metric %q for field %q (idx %d): %w", metricName, ft.Name, i, mtErr)
			}
			*t = mt
		case *metric.Int64ObservableCounter:
			if !metricNameOK {
				continue
			}
			mOpts := make([]metric.Int64ObservableCounterOption, len(genericOpts), len(genericOpts)+1)
			for i, opt := range genericOpts {
				mOpts[i] = opt
			}
			if cbs, cbOK := opts.IntCallbacks[pathPrefix+fPathComponent()]; cbOK {
				for _, cb := range cbs {
					mOpts = append(mOpts, metric.WithInt64Callback(cb))
				}
			}

			mt, mtErr := m.Int64ObservableCounter(metricName, mOpts...)
			if mtErr != nil {
				return fmt.Errorf("failed to register metric %q for field %q (idx %d): %w", metricName, ft.Name, i, mtErr)
			}
			*t = mt
		case *metric.Int64ObservableUpDownCounter:
			if !metricNameOK {
				continue
			}
			mOpts := make([]metric.Int64ObservableUpDownCounterOption, len(genericOpts), len(genericOpts)+1)
			for i, opt := range genericOpts {
				mOpts[i] = opt
			}
			if cbs, cbOK := opts.IntCallbacks[pathPrefix+fPathComponent()]; cbOK {
				for _, cb := range cbs {
					mOpts = append(mOpts, metric.WithInt64Callback(cb))
				}
			}
			mt, mtErr := m.Int64ObservableUpDownCounter(metricName, mOpts...)
			if mtErr != nil {
				return fmt.Errorf("failed to register metric %q for field %q (idx %d): %w", metricName, ft.Name, i, mtErr)
			}
			*t = mt
		case *metric.Int64ObservableGauge:
			if !metricNameOK {
				continue
			}
			mOpts := make([]metric.Int64ObservableGaugeOption, len(genericOpts), len(genericOpts)+1)
			for i, opt := range genericOpts {
				mOpts[i] = opt
			}
			if cbs, cbOK := opts.IntCallbacks[pathPrefix+fPathComponent()]; cbOK {
				for _, cb := range cbs {
					mOpts = append(mOpts, metric.WithInt64Callback(cb))
				}
			}
			mt, mtErr := m.Int64ObservableGauge(metricName, mOpts...)
			if mtErr != nil {
				return fmt.Errorf("failed to register metric %q for field %q (idx %d): %w", metricName, ft.Name, i, mtErr)
			}
			*t = mt
		case *metric.Float64Counter:
			if !metricNameOK {
				continue
			}
			mOpts := make([]metric.Float64CounterOption, len(genericOpts))
			for i, opt := range genericOpts {
				mOpts[i] = opt
			}
			mt, mtErr := m.Float64Counter(metricName, mOpts...)
			if mtErr != nil {
				return fmt.Errorf("failed to register metric %q for field %q (idx %d): %w", metricName, ft.Name, i, mtErr)
			}
			*t = mt
		case *metric.Float64UpDownCounter:
			if !metricNameOK {
				continue
			}
			mOpts := make([]metric.Float64UpDownCounterOption, len(genericOpts))
			for i, opt := range genericOpts {
				mOpts[i] = opt
			}
			mt, mtErr := m.Float64UpDownCounter(metricName, mOpts...)
			if mtErr != nil {
				return fmt.Errorf("failed to register metric %q for field %q (idx %d): %w", metricName, ft.Name, i, mtErr)
			}
			*t = mt
		case *metric.Float64Histogram:
			if !metricNameOK {
				continue
			}
			mOpts := make([]metric.Float64HistogramOption, len(genericOpts), len(genericOpts)+1)
			for i, opt := range genericOpts {
				mOpts[i] = opt
			}
			if histBoundOK && opts.HistogramBounds != nil {
				// TODO: add a way to make this map lookup for histogram bounds besteffort
				if bounds, boundsOK := opts.HistogramBounds[histBoundName]; boundsOK {
					mOpts = append(mOpts, metric.WithExplicitBucketBoundaries(slices.Sorted(slices.Values(bounds))...))
				} else {
					return fmt.Errorf("field %q has the %q tag set to %q, but no such entry exists in the bounds map; keys: %v",
						ft.Name, histogramBoundRefTag, histBoundName, slices.Collect(maps.Keys(opts.HistogramBounds)))
				}
			}
			// TODO: add fallbacks based on the unit and/or type
			mt, mtErr := m.Float64Histogram(metricName, mOpts...)
			if mtErr != nil {
				return fmt.Errorf("failed to register metric %q for field %q (idx %d): %w", metricName, ft.Name, i, mtErr)
			}
			*t = mt
		case *metric.Float64Gauge:
			if !metricNameOK {
				continue
			}
			mOpts := make([]metric.Float64GaugeOption, len(genericOpts))
			for i, opt := range genericOpts {
				mOpts[i] = opt
			}
			mt, mtErr := m.Float64Gauge(metricName, mOpts...)
			if mtErr != nil {
				return fmt.Errorf("failed to register metric %q for field %q (idx %d): %w", metricName, ft.Name, i, mtErr)
			}
			*t = mt
		case *metric.Float64ObservableCounter:
			if !metricNameOK {
				continue
			}
			mOpts := make([]metric.Float64ObservableCounterOption, len(genericOpts), len(genericOpts)+1)
			for i, opt := range genericOpts {
				mOpts[i] = opt
			}
			if cbs, cbOK := opts.FloatCallbacks[pathPrefix+fPathComponent()]; cbOK {
				for _, cb := range cbs {
					mOpts = append(mOpts, metric.WithFloat64Callback(cb))
				}
			}
			mt, mtErr := m.Float64ObservableCounter(metricName, mOpts...)
			if mtErr != nil {
				return fmt.Errorf("failed to register metric %q for field %q (idx %d): %w", metricName, ft.Name, i, mtErr)
			}
			*t = mt
		case *metric.Float64ObservableUpDownCounter:
			if !metricNameOK {
				continue
			}
			mOpts := make([]metric.Float64ObservableUpDownCounterOption, len(genericOpts), len(genericOpts)+1)
			for i, opt := range genericOpts {
				mOpts[i] = opt
			}
			if cbs, cbOK := opts.FloatCallbacks[pathPrefix+fPathComponent()]; cbOK {
				for _, cb := range cbs {
					mOpts = append(mOpts, metric.WithFloat64Callback(cb))
				}
			}
			mt, mtErr := m.Float64ObservableUpDownCounter(metricName, mOpts...)
			if mtErr != nil {
				return fmt.Errorf("failed to register metric %q for field %q (idx %d): %w", metricName, ft.Name, i, mtErr)
			}
			*t = mt
		case *metric.Float64ObservableGauge:
			if !metricNameOK {
				continue
			}
			mOpts := make([]metric.Float64ObservableGaugeOption, len(genericOpts), len(genericOpts)+1)
			for i, opt := range genericOpts {
				mOpts[i] = opt
			}
			if cbs, cbOK := opts.FloatCallbacks[pathPrefix+fPathComponent()]; cbOK {
				for _, cb := range cbs {
					mOpts = append(mOpts, metric.WithFloat64Callback(cb))
				}
			}
			mt, mtErr := m.Float64ObservableGauge(metricName, mOpts...)
			if mtErr != nil {
				return fmt.Errorf("failed to register metric %q for field %q (idx %d): %w", metricName, ft.Name, i, mtErr)
			}
			*t = mt
		default:
			// iteratively unwrap pointers/interfaces:
			fe := f
			switch ft.Type.Kind() {
			case reflect.Pointer, reflect.Interface:
				// don't even think about following nil pointers
			PTRSTRIP:
				for !fe.IsZero() {
					switch fe.Kind() {
					case reflect.Pointer, reflect.Interface:
						fe = fe.Elem()
					default:
						break PTRSTRIP
					}
				}

			}
			// If it's a struct, then it's not invalid :)
			if fe.Kind() == reflect.Struct {
				// TODO: handle reference-cycles
				if recErr := registerInstruments(m, fe, pathPrefix+"."+fPathComponent()+".", opts); recErr != nil {
					return fmt.Errorf("failed to recurse into %s (type %s): %w", ft.Name, ft.Type, recErr)
				}
			}
		}
	}
	return nil
}
