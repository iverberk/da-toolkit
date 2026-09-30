# analysis decrypt local file

## Brainstorm

- Add a CLI subcommand to decrypt a local encrypted analysis artifact using a caller-provided base64 AES-256 key.
- Reuse the upload encryption framing in reverse: each chunk is stored as 8-byte big-endian length, nonce, AES-GCM ciphertext/tag.
- Keep AES keys memory-only; do not persist keys in checkpoints or config.
- Output should require an explicit path and avoid overwriting unless requested.

## Scope

- Add internal decryption logic next to `internal/analysis/encryption.go`.
- Add `analysis decrypt <encrypted-file> --aes-key <key> --output <path> [--overwrite]`.
- Add tests for encryption/decryption round trip, externally sized chunks, and CLI behavior.

## Validation

- `gofmt -w main.go cmd internal/config internal/analysis` passed.
- `go test ./...` passed.
- `go vet ./...` passed.
- Debugged `VOSviewer.exe.zip`: first chunk length is 33,554,460 bytes, indicating a 32 MiB encrypted chunk plus 12-byte nonce and 16-byte GCM tag; decryption now accepts valid chunks larger than the toolkit upload chunk size.
- `scripts/build-binary.sh /tmp/da-toolkit` failed while building the existing `leechcorepyc==2.23.3` dependency for `volatility3[full]`: QEMU device warnings treated as errors and linker could not find `-lusb-1.0` in this environment.
