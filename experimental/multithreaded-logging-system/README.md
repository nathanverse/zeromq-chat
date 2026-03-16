# Multithreaded Logging System

This experiment demonstrates the safe ZeroMQ pattern for multithreaded work inside a single process:

- one shared context for the whole process
- each thread or goroutine creates and owns its own socket
- never share a socket across threads
- each thread closes its own socket before exiting

## Architecture

```text
           +-----------+
Worker 1 ->|           |
Worker 2 ->|  Logger   |-> print logs
Worker 3 ->|           |
           +-----------+
```

Socket pattern:

- workers use `PUSH`
- logger uses `PULL`
- all sockets use `inproc://log-pipeline`

`inproc://` is intentional. It only works correctly when every socket is created from the same ZeroMQ context in the same process, so it is a good way to learn the context rule clearly.

## Run

From the existing Go module in [`/Users/vutran/Documents/my_learning/side_project/chat_app/broker`](/Users/vutran/Documents/my_learning/side_project/chat_app/broker):

```bash
go run ../experimental/multithreaded-logging-system/main.go
```

Optional flags:

```bash
go run ../experimental/multithreaded-logging-system/main.go \
  -workers 3 \
  -messages 5 \
  -delay-ms 400
```

## What to Observe

- The context is created once in `main`.
- The logger goroutine creates its own `PULL` socket and binds it.
- Every worker goroutine creates its own `PUSH` socket and connects it.
- No socket pointer is passed between goroutines after creation.
- Each goroutine closes its own socket with `defer socket.Close()`.
- Random delays make message ordering visible.

## Expected Output Shape

```text
09:30:00.123 | worker-1 | processing job 42
09:30:00.188 | worker-3 | error retry
09:30:00.241 | worker-2 | finished task
```
