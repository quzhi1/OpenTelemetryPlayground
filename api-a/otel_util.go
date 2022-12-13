package main

import (
	"context"
	"fmt"
	"time"

	"cloud.google.com/go/pubsub"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.4.0"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func initTracer() *sdktrace.TracerProvider {
	// // Print locally
	// exporter, err := stdouttrace.New(stdouttrace.WithPrettyPrint())

	// Connect to collector
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	conn, err := grpc.DialContext(ctx, "my-opentelemetry-collector.default.svc.cluster.local:4317", grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithBlock())
	if err != nil {
		panic(err)
	}

	// Set up a trace exporter
	exporter, err := otlptracegrpc.New(ctx, otlptracegrpc.WithGRPCConn(conn))

	// Handle error and create tracer provider
	if err != nil {
		panic(err)
	}
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(
			resource.NewWithAttributes(
				semconv.SchemaURL,
				semconv.ServiceNameKey.String("api-a"),
			)),
	)
	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(propagation.TraceContext{}, propagation.Baggage{}))
	return tp
}

func beforePublishMessage(ctx context.Context, topicID string, msg *pubsub.Message) (context.Context, trace.Span) {
	opts := []trace.SpanStartOption{
		trace.WithSpanKind(trace.SpanKindProducer),
		trace.WithAttributes(
			// customizable span attributes
			semconv.MessagingSystemKey.String("pubsub"),
			semconv.MessagingDestinationKey.String(topicID),
			semconv.MessagingDestinationKindTopic,
		),
	}

	ctx, span := tracer.Start(ctx, fmt.Sprintf("%s send", topicID), opts...)

	if msg.Attributes == nil {
		msg.Attributes = make(map[string]string)
	}

	// propagate Span across process boundaries
	otel.GetTextMapPropagator().Inject(ctx, propagation.MapCarrier(msg.Attributes))

	return ctx, span
}

func afterPublishMessage(span trace.Span, messageID string, err error) {
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	} else {
		span.SetAttributes(semconv.MessagingMessageIDKey.String(messageID))
	}
}
