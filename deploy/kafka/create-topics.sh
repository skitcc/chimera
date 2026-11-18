#!/bin/sh
# Creates the event topics and their DLQs. Brokers run with auto-create disabled,
# so a typo in a topic name fails loudly instead of creating a dead topic.
set -eu

bootstrap=${KAFKA_BOOTSTRAP:?KAFKA_BOOTSTRAP is required}
partitions=${KAFKA_TOPIC_PARTITIONS:-3}
replicas=${KAFKA_TOPIC_REPLICAS:-1}
topics=/opt/kafka/bin/kafka-topics.sh

for topic in identity.user.v1 catalog.track.v1; do
	"$topics" --bootstrap-server "$bootstrap" --create --if-not-exists \
		--topic "$topic" --partitions "$partitions" --replication-factor "$replicas" \
		--config retention.ms=604800000
	"$topics" --bootstrap-server "$bootstrap" --create --if-not-exists \
		--topic "$topic.dlq" --partitions 1 --replication-factor "$replicas" \
		--config retention.ms=2592000000
done
"$topics" --bootstrap-server "$bootstrap" --list
