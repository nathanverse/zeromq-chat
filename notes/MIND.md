 
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