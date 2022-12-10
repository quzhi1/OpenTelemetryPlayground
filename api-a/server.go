package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"cloud.google.com/go/pubsub"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/sdk/resource"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/gofiber/fiber/v2"
	"github.com/quzhi1/open-telemetry-playground/util"

	"github.com/gofiber/contrib/otelfiber"
	"github.com/rs/zerolog/log"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"

	"go.opentelemetry.io/otel/propagation"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.4.0"
	oteltrace "go.opentelemetry.io/otel/trace"
)

var tracer = otel.Tracer("api-a")
var pubSubTopic = "source-topic"

type ApiAServer struct {
	PubSubClient *pubsub.Client
}

func main() {
	// Server context
	ctx := context.Background()

	// Create open telemetry client
	tp := initTracer()
	defer func() {
		if err := tp.Shutdown(context.Background()); err != nil {
			log.Printf("Error shutting down tracer provider: %v", err)
		}
	}()

	// Create pubsub client
	client, err := pubsub.NewClient(ctx, "example-project")
	if err != nil {
		panic(fmt.Sprintf("Failed to create pubsub client: %v", err))
	}
	defer client.Close()

	// Create ApiAServer
	apiAServer := ApiAServer{
		PubSubClient: client,
	}

	// Create fiber server
	app := fiber.New()

	// customise span name
	//app.Use(otelfiber.Middleware("my-server", otelfiber.WithSpanNameFormatter(func(ctx *fiber.Ctx) string {
	//	return fmt.Sprintf("%s - %s", ctx.Method(), ctx.Route().Path)
	//})))

	// otel middleware
	app.Use(otelfiber.Middleware("api-a"))

	// Define routes
	app.Get("/users/:id", apiAServer.getUser)
	app.Post("/publish", apiAServer.publish)

	// Listen and serve
	err = app.Listen(":3000")
	if err != nil {
		panic(err)
	}
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
				semconv.ServiceNameKey.String("api-a"),
			)),
	)
	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(propagation.TraceContext{}, propagation.Baggage{}))
	return tp
}

// getUser return user name by id
func (aas ApiAServer) getUser(c *fiber.Ctx) error {
	// Parse parameter
	id := c.Params("id")

	// Create span for handler
	ctx := c.UserContext()
	thisCtx, span := tracer.Start(ctx, "getUser", oteltrace.WithAttributes(attribute.String("id", id)))
	defer span.End()

	// Create logger
	contextLogger := log.With().Str("trace_id", util.GetTraceIdFromSpan(span)).Logger()

	// Call api-b
	name, err := callApiB(thisCtx, id)
	if err != nil {
		contextLogger.Error().Str("span_id", util.GetSpanIdFromSpan(span)).Msgf("Error in calling api-b: %s", err.Error())
		return err
	} else {
		contextLogger.Info().Str("span_id", util.GetSpanIdFromSpan(span)).Msgf("Got name from api-b: %s", name)
	}

	// Return response
	return c.JSON(fiber.Map{"id": id, name: name})
}

// publish send a message to PubSub
func (aas ApiAServer) publish(c *fiber.Ctx) error {
	ctx := c.UserContext()

	// Get topic
	topic := aas.PubSubClient.Topic(pubSubTopic)

	// Check existence
	exists, err := topic.Exists(ctx)
	if !exists || err != nil {
		log.Error().Err(err).Msgf("Error loading Topic %v. Exists: %v Err: %v", pubSubTopic, exists, err)
		return c.JSON(fiber.Map{"status": "error"})
	}

	// Construct payload
	data := map[string]string{
		"hello": "world",
	}

	dataSerialized, err := json.Marshal(data)
	if err != nil {
		log.Error().Err(err).Msgf("Error serializing data %v.", data)
		return c.JSON(fiber.Map{"status": "error"})
	}

	log.Info().Msgf("Publishing data: %v", string(dataSerialized))

	// Publish
	result := topic.Publish(ctx, &pubsub.Message{Data: dataSerialized})

	messageId, err := result.Get(ctx)
	if err != nil {
		log.Error().Err(err).Msgf("Error publishing data %v.", string(dataSerialized))
		return c.JSON(fiber.Map{"status": "error"})
	}

	log.Info().Msgf("Publish success, messageId: %s", messageId)

	return c.JSON(fiber.Map{"status": "published"})
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
