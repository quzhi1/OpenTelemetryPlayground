package main

import (
	"context"
	"fmt"
	"time"

	"cloud.google.com/go/pubsub"
	"github.com/rs/zerolog/log"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.4.0"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type PubSubHandler = func(context.Context, *pubsub.Message)

var tracer = otel.Tracer("api-b")
var InstrumentedHandler PubSubHandler

type Flush interface {
	ForceFlush(context.Context) error
}

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
				semconv.ServiceNameKey.String("api-b"),
			)),
	)
	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(propagation.TraceContext{}, propagation.Baggage{}))

	// Instrument pubsub
	InstrumentedHandler = instrumentedHandler(pubSubTopic, pubSubHandler, tp)

	return tp
}

func instrumentedHandler(topicID string, handler PubSubHandler, flush Flush) PubSubHandler {
	return func(ctx context.Context, msg *pubsub.Message) {
		// create span
		ctx, span := beforePubSubHandlerInvoke(ctx, topicID, msg)
		defer span.End()

		// call actual handler function
		handler(ctx, msg)

		// flush spans
		flush.ForceFlush(ctx)
	}
}

func beforePubSubHandlerInvoke(ctx context.Context, topicID string, msg *pubsub.Message) (context.Context, trace.Span) {
	if msg.Attributes != nil {
		// extract propagated span
		propagator := otel.GetTextMapPropagator()
		log.Info().Msg("Extracing traceparent from message attribute")
		ctx = propagator.Extract(ctx, propagation.MapCarrier(msg.Attributes))
	}

	opts := []trace.SpanStartOption{
		trace.WithSpanKind(trace.SpanKindConsumer),
		trace.WithAttributes(
			//customizable attributes
			semconv.FaaSTriggerPubsub,
			semconv.MessagingSystemKey.String("pubsub"),
			semconv.MessagingDestinationKey.String(topicID),
			semconv.MessagingDestinationKindTopic,
			semconv.MessagingOperationProcess,
			semconv.MessagingMessageIDKey.String(msg.ID),
		),
	}

	return tracer.Start(ctx, fmt.Sprintf("%s process", topicID), opts...)
}
