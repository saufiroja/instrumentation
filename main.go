package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"runtime"
	"time"

	"github.com/saufiroja/instrumentation/pkg"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/metric"
)

func main() {
	logs := pkg.NewLogger()

	otelAgentAddr, ok := os.LookupEnv("OTEL_EXPORTER_OTLP_ENDPOINT")
	if !ok {
		otelAgentAddr = "otel-collector:4317"
	}

	provider := pkg.NewProvider(otelAgentAddr, logs)
	defer provider.Shutdown()

	meter := NewTypeMetrics()

	// create a handler wrapped in OpenTelemetry instrumentation
	handler := http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		// sleep
		time.Sleep(1 * time.Second)
		ctx := req.Context()

		tracers := otel.Tracer("demo-server")
		_, span := tracers.Start(ctx, "helloHandler")
		defer span.End()

		// increment the request count
		meter.RequestPerSecondAdd(ctx)
		meter.MemoryUsageAdd(ctx)
		meter.GoroutineCountAdd(ctx)
		meter.MemoryAllocatedAdd(ctx)
		meter.UpTimeAdd(ctx, logs)

		logs.StartLogger("main.go", "handler").Error("Hello World")

		if _, err := w.Write([]byte("Hello World")); err != nil {
			http.Error(w, "write operation failed.", http.StatusInternalServerError)
			return
		}

		meter.ResponseTimeAdd(ctx)
	})

	mux := http.NewServeMux()
	mux.Handle("/hello", otelhttp.NewHandler(handler, "/hello"))
	server := &http.Server{
		Addr:              ":8080",
		Handler:           mux,
		ReadHeaderTimeout: 20 * time.Second,
	}

	meter.UpTimeAdd(context.Background(), logs)

	if err := server.ListenAndServe(); err != http.ErrServerClosed {
		log.Fatalf("failed to listen: %v", err)
	}

}

type TypeMetrics struct {
	MemoryUsage      metric.Meter // gauge
	RequestPerSecond metric.Meter // counter
	GoroutineCount   metric.Meter // gauge
	ResponseTime     metric.Meter // histogram
	MemoryAllocated  metric.Meter // summary
	UpTime           metric.Meter // gauge
}

func NewTypeMetrics() *TypeMetrics {
	meter := otel.Meter("demo-server")
	return &TypeMetrics{
		MemoryUsage:      meter,
		RequestPerSecond: meter,
		GoroutineCount:   meter,
		ResponseTime:     meter,
		MemoryAllocated:  meter,
		UpTime:           meter,
	}
}

func (m *TypeMetrics) MemoryUsageAdd(ctx context.Context) {
	if _, err := m.MemoryUsage.Int64ObservableGauge(
		"memory.heap",
		metric.WithDescription(
			"Memory usage of the allocated heap objects.",
		),
		metric.WithUnit("By"),
		metric.WithInt64Callback(func(_ context.Context, o metric.Int64Observer) error {
			var m runtime.MemStats
			runtime.ReadMemStats(&m)
			o.Observe(int64(m.HeapAlloc))
			return nil
		}),
	); err != nil {
		panic(err)
	}
}

func (m *TypeMetrics) RequestPerSecondAdd(ctx context.Context) {
	meter, err := m.RequestPerSecond.Float64Counter(
		"request.per.second",
		metric.WithDescription("The number of requests per second."),
		metric.WithUnit("1"),
	)
	if err != nil {
		panic(err)
	}

	meter.Add(ctx, 1)
}

func (m *TypeMetrics) GoroutineCountAdd(ctx context.Context) {
	if _, err := m.GoroutineCount.Int64ObservableGauge(
		"goroutine.count",
		metric.WithDescription("The number of goroutines that currently exist."),
		metric.WithUnit("1"),
		metric.WithInt64Callback(func(_ context.Context, o metric.Int64Observer) error {
			o.Observe(int64(runtime.NumGoroutine()))
			return nil
		}),
	); err != nil {
		panic(err)
	}
}

func (m *TypeMetrics) ResponseTimeAdd(ctx context.Context) {
	start := time.Now()
	histogram, err := m.ResponseTime.Float64Histogram(
		"response.time",
		metric.WithDescription("The response time of the server."),
		metric.WithUnit("ms"),
		metric.WithExplicitBucketBoundaries(float64(0), float64(100), float64(200), float64(300), float64(400), float64(500)),
	)
	if err != nil {
		panic(err)
	}

	histogram.Record(ctx, time.Since(start).Seconds())
}

func (m *TypeMetrics) MemoryAllocatedAdd(ctx context.Context) {
	histogram, err := m.MemoryAllocated.Float64Histogram(
		"memory.allocated",
		metric.WithDescription("The amount of memory allocated."),
		metric.WithUnit("By"),
		metric.WithExplicitBucketBoundaries(float64(0), float64(100), float64(200), float64(300), float64(400), float64(500)),
	)
	if err != nil {
		panic(err)
	}

	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)
	histogram.Record(ctx, float64(memStats.Alloc))
}

func (m *TypeMetrics) UpTimeAdd(ctx context.Context, logs pkg.ILogger) {
	uptime := time.Since(time.Now()).Seconds()

	logs.StartLogger("main.go", "uptime").Infof("Uptime: %v", uptime)

	_, err := m.UpTime.Int64ObservableGauge(
		"uptime",
		metric.WithDescription("The time the server has been running."),
		metric.WithUnit("s"),
		metric.WithInt64Callback(func(_ context.Context, o metric.Int64Observer) error {
			o.Observe(int64(uptime))
			return nil
		}),
	)

	if err != nil {
		panic(err)
	}
}
