package pkg

import (
	"context"
	"fmt"

	"go.opentelemetry.io/otel/sdk/resource"
	semconv "go.opentelemetry.io/otel/semconv/v1.4.0"
)

type IProvider interface {
	Shutdown()
}

type Provider struct {
	Tracer  ITracer
	Metrics IMetrics
}

func NewProvider(otelAgentAddr string, log ILogger) IProvider {
	res, err := resource.New(context.Background(),
		resource.WithFromEnv(),
		resource.WithProcess(),
		resource.WithTelemetrySDK(),
		resource.WithHost(),
		resource.WithAttributes(
			semconv.ServiceNameKey.String("demo-server"),
		),
	)

	if err != nil {
		errMsg := fmt.Sprintf("failed to create resource: %v", err)
		log.StartLogger("provider.go", "NewProvider").Error(errMsg)
		panic(errMsg)
	}

	return &Provider{
		Tracer:  NewTracer(otelAgentAddr, res, log),
		Metrics: NewMetrics(otelAgentAddr, res, log),
	}
}

func (p *Provider) Shutdown() {
	p.Tracer.Shutdown()
	p.Metrics.Shutdown()
}
