package config

import (
	"strings"

	clowder "github.com/redhatinsights/app-common-go/pkg/api/v1"
	"github.com/spf13/viper"
)

func AddEventConfigDefaults(options *viper.Viper) {
	options.SetDefault("kafka.timeout", 10000)
	options.SetDefault("kafka.group.id", "content-sources")
	options.SetDefault("kafka.auto.offset.reset", "latest")
	options.SetDefault("kafka.auto.commit.interval.ms", 5000)
	options.SetDefault("kafka.request.required.acks", -1) // -1 == "all"
	options.SetDefault("kafka.message.send.max.retries", 15)
	options.SetDefault("kafka.retry.backoff.ms", 100)
	if clowder.IsClowderEnabled() {
		cfg := clowder.LoadedConfig
		broker := cfg.Kafka.Brokers[0]
		// options.SetDefault("web.port", cfg.PublicPort)
		options.SetDefault("kafka.bootstrap.servers", strings.Join(clowder.KafkaServers, ","))
		options.SetDefault("topic.repos", clowder.KafkaTopics["repos-introspect"].Name)

		if broker.Authtype != nil {
			options.Set("kafka.sasl.username", *broker.Sasl.Username)
			options.Set("kafka.sasl.password", *broker.Sasl.Password)
			// options.Set("kafka.sasl.mechanism", *broker.Sasl.SaslMechanism)
			// options.Set("kafka.sasl.protocol", *broker.Sasl.SecurityProtocol)
		}
		if broker.Cacert != nil {
			caPath, err := cfg.KafkaCa(broker)
			if err != nil {
				panic("Kafka CA failed to write")
			}
			options.Set("kafka.capath", caPath)
		}
	} else {
		// TOOD Review, probably clean-up this else
		// If cloweder is not present, set defaults to local configuration
		// options.SetDefault("web.port", 8000)
		// This port should match with the exposed by the local container
		options.SetDefault("kafka.bootstrap.servers", "localhost:9092")
		options.SetDefault("topic.repos", "platform.playbook-dispatcher.runner-updates")
	}
}

// func SetContextLogger(ctx context.Context) {
// 	context.Context.Value("logger", log.Logger)
// }
