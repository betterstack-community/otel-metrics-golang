package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"runtime"
	"time"

	"github.com/joho/godotenv"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

type metrics struct {
	httpRequestCounter         metric.Int64Counter
	activeRequestUpDownCounter metric.Int64UpDownCounter
	memoryUsageObservableGuage metric.Int64ObservableGauge
}

func newMetrics(meter metric.Meter) (*metrics, error) {
	var m metrics

	httpRequestsCounter, err := meter.Int64Counter(
		"http.server.requests",
		metric.WithDescription("Total number of HTTP requests received."),
		metric.WithUnit("{requests}"),
	)
	if err != nil {
		return nil, err
	}

	activeRequestUpDownCounter, err := meter.Int64UpDownCounter(
		"http.server.active_requests",
		metric.WithDescription("Number of in-flight requests."),
		metric.WithUnit("{requests}"),
	)
	if err != nil {
		return nil, err
	}

	m.memoryUsageObservableGuage, err = meter.Int64ObservableGauge(
		"system.memory.heap",
		metric.WithDescription(
			"Memory usage of the allocated heap objects.",
		),
		metric.WithUnit("By"),
		metric.WithInt64Callback(
			func(ctx context.Context, o metric.Int64Observer) error {
				memoryUsage := getMemoryUsage()
				o.Observe(int64(memoryUsage))
				return nil
			},
		),
	)

	m.httpRequestCounter = httpRequestsCounter
	m.activeRequestUpDownCounter = activeRequestUpDownCounter

	return &m, nil
}

func getMemoryUsage() uint64 {
	var memStats runtime.MemStats

	runtime.ReadMemStats(&memStats)

	currentMemoryUsage := memStats.HeapAlloc

	return currentMemoryUsage
}

func init() {
	_ = godotenv.Load()
}

func main() {
	ctx := context.Background()

	otelShutdown, err := setupOTelSDK(ctx)
	if err != nil {
		log.Fatal(err)
	}

	defer func() {
		err = errors.Join(err, otelShutdown(ctx))

		log.Println(err)
	}()

	meter := otel.Meter(os.Getenv("OTEL_SERVICE_NAME"))

	m, err := newMetrics(meter)
	if err != nil {
		panic(err)
	}

	mux := http.NewServeMux()

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		m.httpRequestCounter.Add(
			r.Context(),
			1,
			metric.WithAttributes(
				attribute.String("http.route", r.URL.Path),
			),
		)
		m.activeRequestUpDownCounter.Add(r.Context(), 1)

		time.Sleep(1 * time.Second)

		w.Write([]byte("Hello world!"))

		m.activeRequestUpDownCounter.Add(r.Context(), -1)
	})

	handler := otelhttp.NewHandler(mux, "/")

	log.Println("Starting HTTP server on port 8000")

	if err := http.ListenAndServe(":8000", handler); err != nil {
		log.Fatal("Server failed to start:", err)
	}
}
