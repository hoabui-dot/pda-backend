# Kafka Test Environment Request — BE-08

Please provide a Kafka-compatible broker environment for full PDA backend integration testing.

## Minimum broker

- Kafka protocol reachable from the development machine.
- One broker is sufficient for functional testing; three brokers are preferred for failover testing.
- Bootstrap address, for example `host:9092`.
- Ability to create and delete test topics.
- Broker must accept JSON event payloads up to at least 1 MiB.

## Required topics

Create these topics, or grant the test identity topic-create permission:

- `pda.task.events.v1`
- `pda.receiving.events.v1`
- `pda.movement.events.v1`
- `pda.inventory.events.v1`
- `pda.shipping.events.v1`
- `pda.dlq`

Recommended settings: replication factor 1 for local testing, 3 for failover testing; at least 3 partitions for ordering/partition-key tests; retention of at least 24 hours.

## Required identity and permissions

Provide a dedicated test principal and credentials with:

- `DESCRIBE` and `READ` on the test topics.
- `WRITE` and `DESCRIBE` on the event topics.
- `WRITE` on `pda.dlq`.
- Consumer-group permissions for groups beginning with `pda-be08-`.
- Topic create/describe permission if topics are not pre-created.

## Security details

Please specify one of:

- `PLAINTEXT` (local-only testing), or
- `SSL` / `SASL_SSL`, including CA certificate, client certificate/key if required, SASL mechanism, username, and password.

Do not send production credentials. A disposable test principal is required.

## Values to provide

```text
PDA_KAFKA_BROKERS=host:port[,host:port]
PDA_KAFKA_GROUP_ID=pda-be08
PDA_KAFKA_TOPIC_PREFIX=pda
PDA_KAFKA_SECURITY_PROTOCOL=PLAINTEXT|SSL|SASL_SSL
PDA_KAFKA_SASL_MECHANISM=<mechanism, if applicable>
PDA_KAFKA_USERNAME=<test principal, if applicable>
PDA_KAFKA_PASSWORD=<test secret, if applicable>
PDA_KAFKA_TLS_CA_FILE=<path, if applicable>
```

## Verification flow

1. Validate broker reachability and topic metadata.
2. Publish an event with aggregate ID as the Kafka key.
3. Consume it using a `pda-be08-*` consumer group and verify inbox idempotency.
4. Replay the same event and verify no duplicate processing.
5. Force handler failures and verify bounded retries and `pda.dlq` persistence.
6. Stop/restart a broker and verify outbox retry and eventual delivery.
7. Publish multiple events for one aggregate and verify ordering.
8. Capture consumer lag/backlog metrics.
9. Verify ACL denial with an unauthorized test principal.

The shared MES platform now supplies the local PLAINTEXT test environment used by PDA: `platform-kafka`, reachable from the host at `127.0.0.1:19092` and from `platform-net` containers as `kafka:9092`. The PDA repository no longer starts a second broker. The test command provisions the required topics idempotently before exercising producer and consumer behavior. The current implementation still fails closed for unsupported security protocols; ACL/TLS verification requires a security-enabled broker.
