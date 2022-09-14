package main

import (
	"context"
	"errors"
	"log"
	"time"

	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/sdk/resource"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/gofiber/fiber/v2"

	"github.com/gofiber/contrib/otelfiber"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"

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
	// // Print locally
	// exporter, err := stdouttrace.New(stdouttrace.WithPrettyPrint())

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

	// Handle error and create tracer provider
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
	// Parse id
	id := c.Params("id")

	// Get context
	ctx := c.UserContext()

	// Parse baggage if there is one
	// bag := baggage.FromContext(ctx)
	// span.AddEvent("handling this...", trace.WithAttributes(attribute.Key("username").String(bag.Member("username").Value())))

	// Create new span
	thisCtx, span := tracer.Start(ctx, "getDb", oteltrace.WithAttributes(attribute.String("id", id)))
	defer span.End()

	// Get name and return
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
