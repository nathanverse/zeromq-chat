 
 # Websocket
 1) Connection states

  - Track: connecting → open → closing → closed.
  - Only send when open.

  2) Authentication on connect

  - Expect a token in the connection URL or first message.
  - If invalid, close immediately with a clear close code/reason.

  3) Keepalive

  - Send ping on an interval (e.g., every 20–30s).
  - If no pong within timeout, close the connection.

  4) Backpressure

  - Per‑connection outgoing queue with a max size.
  - If queue exceeds size: drop oldest/newest or disconnect slow client.

  5) Error handling

  - Wrap send/receive in try/catch.
  - On error, close cleanly and release resources.

  6) Clean shutdown

  - On close: remove from subscriptions, clear timers, release queues.

  7) Reconnect support

  - Clients should retry with exponential backoff.
  - Allow “resume” by accepting a last‑event ID if you use it.


  ## Building

  - Port mismatch everywhere: the server listens on 9002, but Deployment uses 8080 and Service routes to 8080. Ingress routes to Service port 80 →
    targetPort 8080. Unless you changed the server, traffic won’t reach it.
  - Health probes are HTTP /healthz, but your server doesn’t expose HTTP. These probes will fail and kill pods.
  - Ingress path /ws expects the WS server to accept /ws (or you need a rewrite). Your server currently accepts on / by default unless you enforce a
    path.
  - No readiness/liveness for a TCP-only server: should use tcpSocket on the WS port or add a small HTTP health endpoint.

  Optional but recommended

  - Add terminationGracePeriodSeconds and a preStop hook to stop accepting new connections before SIGTERM, for graceful WS disconnects.
  - Add resource requests/limits.