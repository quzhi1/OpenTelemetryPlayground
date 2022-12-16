package main

import (
	"context"
	"encoding/json"
	"fmt"

	"cloud.google.com/go/pubsub"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/gofiber/fiber/v2"
	"github.com/quzhi1/open-telemetry-playground/util"

	"github.com/gofiber/contrib/otelfiber"
	"go.opentelemetry.io/otel/attribute"

	oteltrace "go.opentelemetry.io/otel/trace"
)

var pubSubTopic = "source-topic"
var subName = "source-sub"

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

	// Receive pubsub
	go receivePubSub(ctx)

	// Create fiber server
	app := fiber.New()

	// customise span name
	//app.Use(otelfiber.Middleware("my-server", otelfiber.WithSpanNameFormatter(func(ctx *fiber.Ctx) string {
	//	return fmt.Sprintf("%s - %s", ctx.Method(), ctx.Route().Path)
	//})))

	// otel middleware
	app.Use(otelfiber.Middleware("api-b"))

	// Define routes
	app.Get("/db/:id", getDb)

	// Listen and serve
	err := app.Listen(":3010")
	if err != nil {
		panic(err)
	}
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

	// Create logger
	contextLogger := log.With().Str("trace_id", util.GetTraceIdFromSpan(span)).Logger()

	// Get name and return
	name := readDb(thisCtx, id, contextLogger)
	contextLogger.Info().Str("span_id", util.GetSpanIdFromSpan(span)).Msgf("Got name from database: %s", name)
	return c.SendString(name)
}

// readDb pretend to read from database
func readDb(ctx context.Context, id string, contextLogger zerolog.Logger) string {
	// Create new span
	_, span := tracer.Start(ctx, "readDb", oteltrace.WithAttributes(attribute.String("id", id)), oteltrace.WithSpanKind(oteltrace.SpanKindInternal))
	defer span.End()

	contextLogger.Info().Str("span_id", util.GetSpanIdFromSpan(span)).Msgf("Reading database, id: %s", id)
	if id == "123" {
		return "otelfiber tester"
	}
	return "unknown"
}

// receivePubSub handles pubsub messages
func receivePubSub(ctx context.Context) {
	// Create pubsub client
	client, err := pubsub.NewClient(ctx, "example-project")
	if err != nil {
		panic(fmt.Sprintf("Failed to create pubsub client: %v", err))
	}
	defer client.Close()

	// Get subscription
	subscription := client.Subscription(subName)

	exists, err := subscription.Exists(ctx)
	if !exists || err != nil {
		panic(fmt.Sprintf("Failed to create pubsub client: %v", err))
	}

	// Handling messages
	err = subscription.Receive(ctx, wrapPubSubHandlerWithTelemetry(pubSubTopic, pubSubHandler))

	if err != nil {
		panic(err)
	}
}

func pubSubHandler(_ context.Context, m *pubsub.Message) {
	var data map[string]string
	err := json.Unmarshal(m.Data, &data)
	if err != nil {
		log.Error().Err(err)
		m.Nack()
		return
	}

	log.Info().Msgf("Received PubSub message, id: %s, data: %s, attribute: %v", m.ID, data, m.Attributes)
	m.Ack()
}
