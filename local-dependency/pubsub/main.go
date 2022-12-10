package main

import (
	"context"
	"os"

	"cloud.google.com/go/pubsub"
	gpubsub "cloud.google.com/go/pubsub"
	"github.com/rs/zerolog/log"
)

const projectId string = "example-project"

func main() {
	os.Setenv("PUBSUB_EMULATOR_HOST", "localhost:8085")
	ctx := log.With().Str("component", "module").Logger().WithContext(context.Background())
	client, err := gpubsub.NewClient(context.TODO(), projectId)
	if err != nil {
		panic(err)
	}
	for topic, subscriptions := range getTopicSubscriptions() {
		if err != nil {
			panic(err)
		}
		CreateTopic(ctx, client, topic)
		for _, subscription := range subscriptions {
			CreateSubscription(ctx, client, subscription, topic)
		}
	}
}

func getTopicSubscriptions() map[string][]string {
	return map[string][]string{
		"source-topic": {"source-sub"},
	}
}

func CreateTopic(ctx context.Context, pubSubClient *pubsub.Client, topic string) {
	logger := log.Ctx(ctx)
	topicRef := pubSubClient.Topic(topic)
	exist, err := topicRef.Exists(ctx)

	switch {
	case err != nil:
		logger.Error().Msgf("Error when checking topic existence. Topic: %s", topic)
	case exist:
		logger.Info().Msgf("Topic already exist. Topic: %s", topic)
	default:
		_, err := pubSubClient.CreateTopic(ctx, topic)
		if err != nil {
			logger.Error().Msgf("Unable to create topic. Topic: %s", topic)
		}

		logger.Info().Msgf("Topic created. Topic: %s", topic)
	}
}

func CreateSubscription(ctx context.Context, pubSubClient *pubsub.Client, subscription, topic string) {
	logger := log.Ctx(ctx)
	topicRef := pubSubClient.Topic(topic)
	subscriptionRef := pubSubClient.Subscription(subscription)
	exist, err := subscriptionRef.Exists(ctx)

	switch {
	case err != nil:
		logger.Error().Msgf("Error when checking subscription existence. Topic: %s, subscription: %s", topic, subscription)
	case exist:
		logger.Info().Msgf("Subscription already exist. Topic: %s, subscription: %s", topic, subscription)
	default:
		_, err := pubSubClient.CreateSubscription(ctx, subscription, pubsub.SubscriptionConfig{
			Topic: topicRef,
		})
		if err != nil {
			logger.Error().Msgf("Unable to subscribe to topic. Topic: %s, subscription: %s", topic, subscription)
		}

		logger.Info().Msgf("Subscribed. Topic: %s, subscription: %s", topic, subscription)
	}
}
