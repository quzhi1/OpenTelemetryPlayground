package main

import (
	"context"
	"errors"
	"io"
	"log"
	"net/http"
	"time"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/sdk/resource"
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

var tracer = otel.Tracer("api-a")

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

	app.Get("/users/:id", getUser)

	log.Fatal(app.Listen(":3000"))
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
func getUser(c *fiber.Ctx) error {
	id := c.Params("id")
	ctx := c.UserContext()

	thisCtx, span := tracer.Start(ctx, "getUser", oteltrace.WithAttributes(attribute.String("id", id)))
	defer span.End()
	name, err := callApiB(thisCtx, id)
	if err != nil {
		return err
	}
	return c.JSON(fiber.Map{"id": id, name: name})
}

// readDb pretend to read from database
func callApiB(ctx context.Context, id string) (string, error) {
	// Create client with otel
	client := http.Client{Transport: otelhttp.NewTransport(http.DefaultTransport)}

	// Add baggage if you want
	// bag, _ := baggage.Parse("username=donuts")
	// ctx = baggage.ContextWithBaggage(ctx, bag)

	req, err := http.NewRequestWithContext(
		ctx,
		"GET",
		"http://api-b.default.svc.cluster.local:3010/db/"+id,
		nil,
	)

	if err != nil {
		return "", err
	}

	res, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()

	body, err := io.ReadAll(res.Body)
	if err != nil {
		return "", err
	}
	return string(body), nil
}
