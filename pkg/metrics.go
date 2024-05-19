package pkg

import (
	"context"
	"fmt"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
)

type IMetrics interface {
	Shutdown()
}

type Metrics struct {
	OtelAgentAddr       string
	OtlpMetricsExporter *otlpmetricgrpc.Exporter
	Resource            *resource.Resource
}

func NewMetrics(otelAgentAddr string, res *resource.Resource, log ILogger) IMetrics {
	metricExp, err := otlpmetricgrpc.New(
		context.Background(),
		otlpmetricgrpc.WithInsecure(),
		otlpmetricgrpc.WithEndpoint(otelAgentAddr),
	)
	if err != nil {
		errMsg := fmt.Sprintf("failed to create metric exporter: %v", err)
		log.StartLogger("metrics.go", "NewMetrics").Error(errMsg)
		panic(err)
	}

	meterProvider := sdkmetric.NewMeterProvider(
		sdkmetric.WithResource(res),
		sdkmetric.WithReader(
			sdkmetric.NewPeriodicReader(
				metricExp,
				sdkmetric.WithInterval(2*time.Second),
			),
		),
	)

	// set the global meter provider
	otel.SetMeterProvider(meterProvider)

	log.StartLogger("metrics.go", "NewMetrics").Info("Meter provider created")

	return &Metrics{
		OtelAgentAddr:       otelAgentAddr,
		OtlpMetricsExporter: metricExp,
		Resource:            res,
	}
}

func (m *Metrics) Shutdown() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := m.OtlpMetricsExporter.Shutdown(ctx); err != nil {
		panic(err)
	}
}
