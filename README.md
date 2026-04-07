# gno-utilities

Small standalone utilities related to the `gno` ecosystem.

## Submodules

- `decode-msgbytes`: Decode `gnoland` validator `msgBytes` payloads into Amino JSON, either from raw hex or copied log lines.
- `val-tests`: Local Gnoland validator network harness running in Docker, with scripted failure/recovery scenarios (node stop/reset/restart, realm deployment, sentry IP rotation).
