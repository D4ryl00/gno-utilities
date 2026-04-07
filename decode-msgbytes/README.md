# decode-msgbytes

Decode `gnoland` validator `msgBytes` payloads into Amino JSON.

## Usage

Run from the module root:

```sh
GOCACHE=/tmp/go-cache go run ./cmd/decode-msgbytes '<hex-or-full-log-line>'
```

You can also pipe input through stdin:

```sh
pbpaste | GOCACHE=/tmp/go-cache go run ./cmd/decode-msgbytes
```

The tool accepts either:

- a raw hex payload
- a full validator log line containing a `msgBytes` field

It prints:

- the decoded outer Amino message type
- the outer Amino JSON
- the inner `std.Tx` Amino JSON when the payload is a mempool `TxMessage`

Supported outer messages include:

- mempool `tm.TxMessage`
- consensus messages such as `tm.NewRoundStepMessage`

## Example

```sh
GOCACHE=/tmp/go-cache go run ./cmd/decode-msgbytes \
  'gnoland-gnoland1  | ... "msgBytes": "0A0D2F746D2E54784D657373616765..."'
```

## Library

The reusable decoder lives in `pkg/decodemsgbytes`.

Available entrypoints:

- `DecodeInput(input string) (*Result, error)`
- `DecodeHex(hexStr string) (*Result, error)`
- `DecodeBytes(rawHex string, raw []byte) (*Result, error)`
- `ExtractHex(input string) (string, error)`
- `AminoJSON(v any) ([]byte, error)`
- `PrettyAminoJSON(v any) ([]byte, error)`

Minimal example:

```go
result, err := decodemsgbytes.DecodeInput(input)
if err != nil {
	// handle error
}

outerJSON, err := decodemsgbytes.PrettyAminoJSON(result.Outer)
if err != nil {
	// handle error
}

if result.Tx != nil {
	txJSON, err := decodemsgbytes.PrettyAminoJSON(result.Tx)
	if err != nil {
		// handle error
	}
	_ = txJSON
}

_ = outerJSON
```
