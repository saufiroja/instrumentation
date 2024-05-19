package pkg

import (
	"context"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc"
)

type ITracer interface {
	StartSpan(name string, ctx context.Context) (context.Context, trace.Span)
	Shutdown()
}

type Tracer struct {
	OtelAgentAddr string
	OtlpExporter  *otlptrace.Exporter
	Resource      *resource.Resource
}

func NewTracer(otelAgentAddr string, res *resource.Resource, log ILogger) ITracer {
	traceExp, err := otlptrace.New(
		context.Background(),
		otlptracegrpc.NewClient(
			otlptracegrpc.WithInsecure(),
			otlptracegrpc.WithEndpoint(otelAgentAddr),
			otlptracegrpc.WithDialOption(grpc.WithBlock()),
		),
	)
	if err != nil {
		errMsg := "failed to create trace exporter"
		log.StartLogger("tracer.go", "NewTracer").Error(errMsg)
		panic(errMsg)
	}

	// create a new batch span processor
	bsp := sdktrace.NewBatchSpanProcessor(traceExp)
	tracerProvider := sdktrace.NewTracerProvider(
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
		sdktrace.WithResource(res),
		sdktrace.WithSpanProcessor(bsp),
	)

	// set the global trace provider
	otel.SetTracerProvider(tracerProvider)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(propagation.TraceContext{}, propagation.Baggage{}))

	log.StartLogger("tracer.go", "NewTracer").Info("Tracer provider created")

	return &Tracer{
		OtelAgentAddr: otelAgentAddr,
		OtlpExporter:  traceExp,
		Resource:      res,
	}
}

func (t *Tracer) Shutdown() {
	cxt, cancel := context.WithTimeout(context.Background(), 5)
	defer cancel()
	if err := t.OtlpExporter.Shutdown(cxt); err != nil {
		otel.Handle(err)
	}
}

func (t *Tracer) StartSpan(name string, ctx context.Context) (context.Context, trace.Span) {
	tracer := otel.Tracer("demo-server")
	return tracer.Start(ctx, name)
}
