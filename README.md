# chat_app

Goal: Build a distributed WebSocket system where backend services publish events to a broker (e.g., ZeroMQ), the broker propagates notifications across services, and WebSocket servers fan out updates to connected clients.

## Docs

- `docs/minikube-deploy.md` - Deploy the Go WebSocket server to Minikube using `k8s/service.yml`.
