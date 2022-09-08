package main

import (
	"context"
	"errors"
	"io/ioutil"
	"log"
	"net/http"

	"go.opentelemetry.io/otel/sdk/resource"
	"go.opentelemetry.io/otel/trace"

	"github.com/gofiber/fiber/v2"

	"github.com/gofiber/contrib/otelfiber"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	stdout "go.opentelemetry.io/otel/exporters/stdout/stdouttrace"

	//"go.opentelemetry.io/otel/exporters/jaeger"
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
	exporter, err := stdout.New(stdout.WithPrettyPrint())
	//exporter, err := jaeger.New(jaeger.WithCollectorEndpoint(jaeger.WithEndpoint("http://localhost:14268/api/traces")))
	if err != nil {
		log.Fatal(err)
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
	_, span := tracer.Start(ctx, "readDb", oteltrace.WithAttributes(attribute.String("id", id)), trace.WithSpanKind(trace.SpanKindInternal))
	defer span.End()

	client := &http.Client{}
	req, err := http.NewRequest(
		"GET",
		"http://api-b.default.svc.cluster.local:3010/db/"+id,
		nil,
	)

	if err != nil {
		return "", err
	}

	// Add trace id
	req.Header.Add("Trace-Id", trace.SpanFromContext(ctx).SpanContext().TraceID().String())

	res, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()

	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return "", err
	}
	return string(body), nil
}
