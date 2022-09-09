package main

import (
	"context"
	"errors"
	"log"
	"time"

	"go.opentelemetry.io/otel/sdk/resource"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/gofiber/fiber/v2"

	"github.com/gofiber/contrib/otelfiber"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"

	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"

	"go.opentelemetry.io/otel/propagation"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.4.0"
	oteltrace "go.opentelemetry.io/otel/trace"
)

var tracer = otel.Tracer("api-b")

func main() {
	tp := initTracer()
	defer func() {
		if err := tp.Shutdown(context.Background()); err != nil {
			log.Printf("Error shutting down tracer provider: %v", err)
		}
	}()

	app := fiber.New()

	// customise span name
	//app.Use(otelfiber.Middleware("my-server", otelfiber.WithSpanNameFormatter(func(ctx *fiber.Ctx) string {
	//	return fmt.Sprintf("%s - %s", ctx.Method(), ctx.Route().Path)
	//})))

	app.Use(otelfiber.Middleware("my-server"))

	app.Get("/error", func(ctx *fiber.Ctx) error {
		return errors.New("abc")
	})

	app.Get("/db/:id", getDb)

	log.Fatal(app.Listen(":3010"))
}

func initTracer() *sdktrace.TracerProvider {
	// Print locally
	// exporter, err := stdout.New(stdout.WithPrettyPrint())

	// Connect to collector
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	conn, err := grpc.DialContext(ctx, "my-opentelemetry-collector.default.svc.cluster.local:4317", grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithBlock())
	if err != nil {
		log.Fatal(err)
		panic(err)
	}

	// Set up a trace exporter
	exporter, err := otlptracegrpc.New(ctx, otlptracegrpc.WithGRPCConn(conn))
	if err != nil {
		log.Fatal(err)
		panic(err)
	}
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(
			resource.NewWithAttributes(
				semconv.SchemaURL,
				semconv.ServiceNameKey.String("my-service"),
			)),
	)
	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(propagation.TraceContext{}, propagation.Baggage{}))
	return tp
}

// getUser return user name by id
func getDb(c *fiber.Ctx) error {
	id := c.Params("id")
	traceIdRaw := c.Get("Trace-Id")
	var ctx context.Context
	if traceIdRaw != "" {
		traceId, err := trace.TraceIDFromHex(traceIdRaw)
		if err != nil {
			return c.Status(400).SendString("Invalid trace id")
		}
		spanCtx := trace.NewSpanContext(trace.SpanContextConfig{
			TraceID: traceId,
		})
		ctx = trace.ContextWithRemoteSpanContext(c.UserContext(), spanCtx)
	} else {
		ctx = c.UserContext()
	}

	thisCtx, span := tracer.Start(ctx, "getDb", oteltrace.WithAttributes(attribute.String("id", id)))
	defer span.End()
	name := readDb(thisCtx, id)
	return c.SendString(name)
}

// readDb pretend to read from database
func readDb(ctx context.Context, id string) string {
	_, span := tracer.Start(ctx, "readDb", oteltrace.WithAttributes(attribute.String("id", id)), trace.WithSpanKind(trace.SpanKindInternal))
	defer span.End()
	if id == "123" {
		return "otelfiber tester"
	}
	return "unknown"
}
