# zeromq-chat

Distributed WebSocket chat with:

- Go WebSocket servers in multiple pods
- a ZeroMQ `XPUB/XSUB` broker for cross-pod room fanout
- Kubernetes ingress stickiness for stable client routing

## Components

- `/Users/knorexvn/Documents/side/zeromq-chat/server_go/main.go`
  - WebSocket server
  - accepts clients on port `8080`
  - exposes `GET /healthz`
  - accepts WS on `/ws`
  - room can be passed as `?room=<room_id>` or `/ws/<room_id>`
- `/Users/knorexvn/Documents/side/zeromq-chat/broker/main.go`
  - ZeroMQ broker
  - binds `XSUB` on `5556`
  - binds `XPUB` on `5557`
- `/Users/knorexvn/Documents/side/zeromq-chat/k8s/dep_broker.yml`
  - broker deployment
- `/Users/knorexvn/Documents/side/zeromq-chat/k8s/service_broker.yml`
  - broker service
- `/Users/knorexvn/Documents/side/zeromq-chat/k8s/dep_ws.yml`
  - WebSocket deployment
- `/Users/knorexvn/Documents/side/zeromq-chat/k8s/service.yml`
  - WebSocket service
- `/Users/knorexvn/Documents/side/zeromq-chat/k8s/ingress_stickiness.yml`
  - nginx ingress with sticky sessions

## How It Works

Each WebSocket pod subscribes to room topics only when it has local clients in that room.

- client connects to a WS pod for room `R`
- WS pod subscribes to ZeroMQ topic `R`
- when a client sends a message, the pod publishes to topic `R`
- broker forwards that message to all WS pods subscribed to `R`
- each receiving pod fans the message out to its local clients in room `R`

The broker is stateless. There is no explicit room creation step in the broker. A room exists implicitly when pods subscribe or publish on that topic.

## Prerequisites

- Docker installed
- Minikube installed
- `kubectl` installed
- `wscat` installed for manual testing

Example install for `wscat`:

```bash
npm install -g wscat
```

## Deploy On Minikube

### 1. Start Minikube

```bash
minikube start
```

### 2. Enable nginx ingress

```bash
minikube addons enable ingress
```

### 3. Point Docker to Minikube

Build images inside Minikube's Docker daemon so Kubernetes can use them without pushing to a registry.

```bash
eval "$(minikube -p minikube docker-env)"
```

### 4. Build the broker image

```bash
docker build -t zmq-broker:1.0.0 /Users/knorexvn/Documents/side/zeromq-chat/broker
```

### 5. Build the WebSocket server image

```bash
docker build -t chat-ws:1.0.0 /Users/knorexvn/Documents/side/zeromq-chat/server_go
```

### 6. Apply Kubernetes manifests

```bash
kubectl apply -f /Users/knorexvn/Documents/side/zeromq-chat/k8s/dep_broker.yml
kubectl apply -f /Users/knorexvn/Documents/side/zeromq-chat/k8s/service_broker.yml
kubectl apply -f /Users/knorexvn/Documents/side/zeromq-chat/k8s/dep_ws.yml
kubectl apply -f /Users/knorexvn/Documents/side/zeromq-chat/k8s/service.yml
kubectl apply -f /Users/knorexvn/Documents/side/zeromq-chat/k8s/ingress_stickiness.yml
```

### 7. Verify rollout

```bash
kubectl get pods
kubectl get svc
kubectl get ingress
kubectl rollout status deployment/zmq-broker
kubectl rollout status deployment/chat-ws
```

### 8. Check logs

Broker:

```bash
kubectl logs -f -l app=zmq-broker --prefix=true
```

WebSocket pods:

```bash
kubectl logs -f -l app=chat-ws --prefix=true
```

## Health Check

Port-forward the WebSocket service:

```bash
kubectl port-forward svc/chat-ws 8080:80
```

In another terminal:

```bash
curl http://127.0.0.1:8080/healthz
```

Expected response:

```text
ok
```

## Test Cross-Pod Fanout

To prove the broker is forwarding room messages between different WS pods, connect directly to individual pods.

### 1. List the chat pods

```bash
kubectl get pods -l app=chat-ws -o wide
```

### 2. Port-forward two different pods

Replace the pod names with actual values from the previous command.

Terminal 1:

```bash
kubectl port-forward pod/chat-ws-abc123 8081:8080
```

Terminal 2:

```bash
kubectl port-forward pod/chat-ws-def456 8082:8080
```

### 3. Open WebSocket clients to the same room on different pods

If using query parameter room selection:

Terminal 3:

```bash
wscat -c ws://127.0.0.1:8081/ws?room=room1
```

Terminal 4:

```bash
wscat -c ws://127.0.0.1:8082/ws?room=room1
```

If using path-based room selection:

```bash
wscat -c ws://127.0.0.1:8081/ws/room1
wscat -c ws://127.0.0.1:8082/ws/room1
```

### 4. Send a message on one pod

In one `wscat` session:

```text
hello from pod A
```

### 5. Verify the message appears in the client connected to the other pod

If the second `wscat` session receives the message, cross-pod room fanout is working.

### 6. Verify logs

Check the WS logs while testing:

```bash
kubectl logs -f -l app=chat-ws --prefix=true
```

Check the broker logs:

```bash
kubectl logs -f -l app=zmq-broker --prefix=true
```

## Ingress Access

The ingress host is configured in `/Users/knorexvn/Documents/side/zeromq-chat/k8s/ingress_stickiness.yml`.

Get the Minikube IP:

```bash
minikube ip
```

Add a hosts entry:

```bash
sudo sh -c 'echo "<MINIKUBE_IP> chat.example.com" >> /etc/hosts'
```

Then connect clients to:

```text
ws://chat.example.com/ws?room=room1
```

## Common Commands

Show pods:

```bash
kubectl get pods -o wide
```

Describe failed pod:

```bash
kubectl describe pod <pod-name>
```

Tail one pod:

```bash
kubectl logs -f pod/<pod-name>
```

Restart rollout:

```bash
kubectl rollout restart deployment/zmq-broker
kubectl rollout restart deployment/chat-ws
```

Delete everything:

```bash
kubectl delete -f /Users/knorexvn/Documents/side/zeromq-chat/k8s/ingress_stickiness.yml
kubectl delete -f /Users/knorexvn/Documents/side/zeromq-chat/k8s/service.yml
kubectl delete -f /Users/knorexvn/Documents/side/zeromq-chat/k8s/dep_ws.yml
kubectl delete -f /Users/knorexvn/Documents/side/zeromq-chat/k8s/service_broker.yml
kubectl delete -f /Users/knorexvn/Documents/side/zeromq-chat/k8s/dep_broker.yml
```
