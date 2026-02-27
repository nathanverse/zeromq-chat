# Deploy chat-ws to Minikube

This guide deploys the WebSocket server from `server_go/main.go` to Minikube using:

- `k8s/dep_ws.yml` (Deployment)
- `k8s/service.yml` (Service)

The server listens on `:8080`, exposes `/healthz` for probes, and `/ws` for WebSocket connections.

## Prerequisites

- `minikube`
- `kubectl`
- `docker`

## 1) Start Minikube

```bash
minikube start
```

## 2) Build the image inside Minikube

Use Minikube's Docker daemon so the cluster can pull the image without pushing to a registry.

```bash
eval "$(minikube -p minikube docker-env)"
docker build -t chat-ws:1.0.0 -f server_go/Dockerfile server_go
```

Alternative (single command):

```bash
minikube image build -t chat-ws:1.0.0 -f server_go/Dockerfile server_go
```

## 3) Apply the Kubernetes manifests

```bash
kubectl apply -f k8s/dep_ws.yml
kubectl apply -f k8s/service.yml
```

The Service selects pods with label `app: chat-ws` and routes port `80` to container port `8080`.

## 4) Verify rollout

```bash
kubectl get deployments
kubectl get pods -l app=chat-ws
kubectl get svc chat-ws
```

Wait until all pods are `READY`.

## 5) Access the service

Option A: Port-forward

```bash
kubectl port-forward svc/chat-ws 8080:80
```

Then:

- Health check: `http://127.0.0.1:8080/healthz`
- WebSocket: `ws://127.0.0.1:8080/ws`

Option B: Minikube service URL

```bash
minikube service chat-ws --url
```

Use the printed URL with `/healthz` or `/ws`.

## 6) Clean up

```bash
kubectl delete -f k8s/service.yml
kubectl delete -f k8s/dep_ws.yml
```

## Notes

- The Deployment uses `image: chat-ws:1.0.0` and `imagePullPolicy: IfNotPresent`. Rebuild the image and reapply if you change the server.
- `k8s/dep_ws.yml` defines readiness and liveness probes against `/healthz`, matching `server_go/main.go`.
