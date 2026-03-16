## Experiments

Small, isolated ZeroMQ learning exercises live here.

- `multithreaded-logging-system/`
  - one shared context
  - one logger goroutine with its own `PULL` socket
  - multiple worker goroutines, each with its own `PUSH` socket
  - `inproc://` transport to make the shared-context rule explicit
