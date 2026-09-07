# Flowtel daemon wire

Flowtel exposes a small local daemon for harness adapters. The daemon is a
transport and durability boundary, not a model proxy and not a destination SDK.

## Process shape

```text
harness adapter -> Unix socket -> flowtel daemon -> journal -> OTLP pipeline
```

The daemon accepts newline-delimited JSON-RPC 2.0 frames. The v1 methods are:

- `initialize`: negotiate protocol version and capabilities.
- `event.log`: validate and durably append one vendor-neutral envelope.
- `session.flush`: wait for the session barrier.
- `status.get`: inspect stored events and known sessions.
- `daemon.shutdown`: stop accepting work and close cleanly.

The journal stores only the envelope. Credentials and destination-specific
fields are not resolved by the wire protocol. A future OTLP sink can consume
the same envelope through the `daemon.Sink` interface without changing the
harness contract.

Run it with:

```text
flowtel daemon serve --socket PATH --data-dir PATH
```

`FLOWTEL_DAEMON_SOCKET` and `FLOWTEL_DATA_DIR` provide defaults. The socket
directory is created with mode `0700`; the journal uses mode `0600`.
