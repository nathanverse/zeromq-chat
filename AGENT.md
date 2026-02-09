# Repository Structure

Purpose of key folders:

- `client/`: Chat app client (TypeScript).
- `server/`: WebSocket server implementation (C++).
- `broker/`: Broker service for inter-service messaging (C++).
- `tools/`: Local tooling such as vcpkg.

Notes:
- The `server/` and `broker/` directories are expected to compile with C++ toolchains.
- The `client/` directory is expected to build with a TypeScript toolchain.
