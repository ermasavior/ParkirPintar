package pkg_otel

import (
	"context"
	"log"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploghttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/log/global"
	"go.opentelemetry.io/otel/propagation"
	sdklogger "go.opentelemetry.io/otel/sdk/log"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.4.0"
	"go.opentelemetry.io/otel/trace"
)

func NewOpenTelemetry(serviceHost, serviceName, appEnv string) *OpenTelemetry {
	var (
		attributeName = semconv.ServiceNameKey.String(serviceName)
		ctx           = context.Background()
	)

	res, err := resource.New(ctx,
		resource.WithAttributes(
			attributeName,
			attribute.String("env", appEnv),
			attribute.String("version", "ver.1"),
			attribute.String("platform", "go"),
		),
	)
	if err != nil {
		log.Fatal(ctx, "error init otel resource", err)
	}

	tp := initTracerProvider(ctx, res)
	mp := initMeterProvider(ctx, res)
	lp := initLoggerProvider(ctx, res)

	tracer := tp.Tracer(serviceName)
	meter := mp.Meter(serviceName)
	logger := lp.Logger(serviceName)

	return &OpenTelemetry{
		Tracer: tracer,
		Meter:  meter,
		Logger: logger,
	}
}

func initTracerProvider(ctx context.Context, res *resource.Resource) *sdktrace.TracerProvider {
	exp, err := otlptracehttp.New(ctx)
	if err != nil {
		log.Fatal(ctx, "error init tracer provider", err)
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithResource(res),
		sdktrace.WithBatcher(exp),
	)

	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.TraceContext{})

	return tp
}

func initMeterProvider(ctx context.Context, res *resource.Resource) *sdkmetric.MeterProvider {
	exp, err := otlpmetrichttp.New(ctx)
	if err != nil {
		log.Fatal(ctx, "error init meter provider", err)
	}

	mp := sdkmetric.NewMeterProvider(
		sdkmetric.WithReader(sdkmetric.NewPeriodicReader(exp)),
		sdkmetric.WithResource(res),
	)

	otel.SetMeterProvider(mp)

	return mp
}

func initLoggerProvider(ctx context.Context, res *resource.Resource) *sdklogger.LoggerProvider {
	exp, err := otlploghttp.New(ctx)
	if err != nil {
		log.Fatal(ctx, "error init logger provider", err)
	}
	lp := sdklogger.NewLoggerProvider(
		sdklogger.WithProcessor(sdklogger.NewBatchProcessor(exp)),
		sdklogger.WithResource(res),
	)

	global.SetLoggerProvider(lp)

	return lp
}

// EndAPM shuts down the tracer
func (o *OpenTelemetry) EndAPM(ctx context.Context) error {
	if tp, ok := otel.GetTracerProvider().(*sdktrace.TracerProvider); ok {
		return tp.Shutdown(ctx)
	}
	if mp, ok := otel.GetMeterProvider().(*sdkmetric.MeterProvider); ok {
		return mp.Shutdown(ctx)
	}
	if lp, ok := global.GetLoggerProvider().(*sdklogger.LoggerProvider); ok {
		return lp.Shutdown(ctx)
	}
	return nil
}

// StartTransaction starts a new OpenTelemetry span
func (o *OpenTelemetry) StartTransaction(ctx context.Context, name string, attributes ...trace.SpanStartOption) (context.Context, interface{}) {
	ctx, span := o.Tracer.Start(ctx, name, attributes...)
	return ctx, span
}

// EndTransaction ends the given OpenTelemetry span
func (o *OpenTelemetry) EndTransaction(txn interface{}) {
	span := txn.(trace.Span)
	span.End()
}
