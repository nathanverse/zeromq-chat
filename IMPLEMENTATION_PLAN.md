# Implementation Plan: WebSocket Rooms + ZeroMQ Broker + Redis Presence

This plan assumes:

- WebSocket servers run in multiple pods.
- A Go ZeroMQ broker handles publish/subscribe fan-out by room.
- Redis is used for global presence (who is in a room).
- Room is derived from the URL path, e.g., `wss://mydomain.com/123`.

## 1) Message Model

Define a single message format used between clients, WS servers, and the broker.

Fields (JSON or protobuf):

- `room_id`
- `sender_id`
- `type` (chat, join, leave, typing, system)
- `payload`
- `timestamp`

## 2) WebSocket Server (pods)

On WebSocket connect:

- Parse `room_id` from URL path.
- Authenticate user (token/cookie).
- Add connection to local room map: `room_id -> []conn`.
- Update Redis presence:
  - `SADD room:<id>:members user:<id>`
  - optional: `SET user:<id>:room <id>`
- Subscribe to broker topic `room_id` if not already subscribed on this pod.

On WebSocket message from client:

- Publish to broker with topic `room_id`.
- Payload includes `sender_id` and message data.

On WebSocket close:

- Remove connection from local room map.
- Update Redis presence:
  - `SREM room:<id>:members user:<id>`
- If local room is empty, unsubscribe from broker for that room.

## 3) ZeroMQ Broker (Go service)

Recommended pattern: XPUB/XSUB.

Behavior:

- Pods publish messages with topic prefix = `room_id`.
- Broker routes messages to all pods subscribed to that `room_id` topic.
- Broker remains stateless; it does not store room membership.

Message frames:

- Frame 1: `room_id` (topic)
- Frame 2: serialized payload

## 4) Redis Presence

Data model:

- `room:<id>:members` = Redis Set of user IDs
- `user:<id>:last_seen` = optional timestamp

Operations:

- On join: `SADD room:<id>:members user:<id>`
- On leave: `SREM room:<id>:members user:<id>`
- Presence query: `SMEMBERS room:<id>:members`

## 5) Reconnect Behavior

- Client reconnects to any pod.
- Pod recomputes room from URL and re-adds presence.
- Pod subscribes to broker if needed.
- No stickiness required.

## 6) Scaling Notes

- Each pod only subscribes to rooms with local members.
- Broker only forwards to subscribed pods.
- Redis enables global presence queries and cross-pod features.

## 7) Failure Modes

- Redis down: allow connect but skip presence (degraded mode).
- Broker down: accept connection but fail publish with error and retry backoff.
- Add reconnect logic for broker sockets.

## 8) Observability

Log per WS connection:

- `pod_name`, `room_id`, `user_id`, `remote_ip`

Metrics:

- active connections
- rooms per pod
- broker publish rate
- presence counts per room

## 9) Kubernetes Wiring

WS pods env:

- `ZMQ_BROKER_ADDR`
- `REDIS_ADDR`
- `POD_NAME`
- `POD_IP`

Deployments:

- `chat-ws` (WebSocket server)
- `zmq-broker` (Go broker)
- `redis` (dev or shared cluster)
