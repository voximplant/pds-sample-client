# Voximplant PDS sample client

Minimal Go client for [Voximplant Predictive Dialer System (PDS)](https://voximplant.com/) over gRPC.

The repository shows how to:

- generate Go code from `proto/pds.proto`
- open a bidirectional PDS stream
- send the `INIT` handshake
- react to `GET_TASK` requests from the server
- push call-list records as `PUT_TASK` messages

## Requirements

- Go 1.22+
- `protoc` (only if you regenerate protobuf code)
- Voximplant account with PDS, SmartQueue, and a VoxEngine scenario

## Quick start

1. Copy environment template:

```bash
cp .env.example .env
```

2. Fill in your account credentials and PDS parameters in `.env`.

3. Build and run:

```bash
make build
./bin/pds-sample-client
```

Configuration is loaded with [cleanenv](https://github.com/ilyakaznacheev/cleanenv) via `client.LoadConfig()` from `.env` (if present) and process environment variables. Env vars override file values.

## Protocol flow

1. Open a bidirectional stream via `PDS.Start` (predictive) or `PDS.StartProgressive`.
2. Send `RequestMessage` with type `INIT`.
3. Wait for `ServiceMessage` with type `INIT_RESPONSE` and store `session_id`.
4. Wait for `ServiceMessage` with type `GET_TASK`.
5. Send exactly the requested number of `PUT_TASK` messages.
6. Handle `TASK_EVENT` updates from the server.

Important rules:

- Do not send tasks before `GET_TASK`.
- Do not send more tasks than requested — the connection will be closed.
- On disconnect, reconnect and repeat initialization. Reuse `session_id` to keep accumulated statistics.

## Project layout

```text
proto/pds.proto          # PDS gRPC contract
protogen/                # generated Go types and gRPC stubs
client/                  # PDS client + Config (cleanenv)
main.go                  # runnable example
```

## Regenerate protobuf code

```bash
make install-tools
make generate
```

## Custom task source

Replace `feedSampleTasks` in `main.go` with your own integration. Each task is a JSON payload passed to the VoxEngine scenario:

```go
agent.Tasks() <- client.Task{
    CustomData: map[string]any{
        "phone_number": "+1234567890",
        "customer_id":  42,
    },
}
```

## Progressive mode

Set in `.env`:

```env
PDS_MODE=progressive
PDS_TASK_MULTIPLIER=1
```
