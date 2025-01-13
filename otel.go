package main

import (
	"context"
	"errors"
	"regexp"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	"go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
)

func setupOTelSDK(
	ctx context.Context,
) (shutdown func(context.Context) error, err error) {
	var shutdownFuncs []func(context.Context) error

	shutdown = func(ctx context.Context) error {
		var err error

		for _, fn := range shutdownFuncs {
			err = errors.Join(err, fn(ctx))
		}

		shutdownFuncs = nil

		return err
	}

	handleErr := func(inErr error) {
		err = errors.Join(inErr, shutdown(ctx))
	}

	res, err := newResource()
	if err != nil {
		handleErr(err)
		return
	}

	meterProvider, err := newMeterProvider(ctx, res)
	if err != nil {
		handleErr(err)
		return
	}

	shutdownFuncs = append(shutdownFuncs, meterProvider.Shutdown)

	otel.SetMeterProvider(meterProvider)

	return
}

func newResource() (*resource.Resource, error) {
	return resource.Merge(resource.Default(),
		resource.NewWithAttributes(semconv.SchemaURL,
			semconv.ServiceName("my-service"),
			semconv.ServiceVersion("0.1.0"),
		))
}

func newMeterProvider(
	ctx context.Context,
	res *resource.Resource,
) (*metric.MeterProvider, error) {
	// metricExporter, err := stdoutmetric.New(stdoutmetric.WithPrettyPrint())
	metricExporter, err := otlpmetrichttp.New(ctx)
	if err != nil {
		return nil, err
	}

	re := regexp.MustCompile(`http\.server\.(request|response)\.size`)
	var dropMetricsView metric.View = func(i metric.Instrument) (metric.Stream, bool) {
		// In a custom View function, you need to explicitly copy
		// the name, description, and unit.
		s := metric.Stream{
			Name:        i.Name,
			Description: i.Description,
			Unit:        i.Unit,
			Aggregation: metric.AggregationDrop{},
		}

		if re.MatchString(i.Name) {
			return s, true
		}

		return s, false
	}

	meterProvider := metric.NewMeterProvider(
		metric.WithResource(res),
		metric.WithReader(metric.NewPeriodicReader(metricExporter,
			// Default is 1m. Set to 3s for demonstrative purposes.
			metric.WithInterval(3*time.Second))),
		metric.WithView(
			dropMetricsView,
		),
	)

	return meterProvider, nil
}
