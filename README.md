# Fast Secret-Key PIR

Implementation of secret-key Private Information Retrieval (PIR) schemes based on permuted Reed-Muller (RM) codes and their extensions. Supports two main constructions:

- **RM-based PIR** — queries a degree-k curve over F_q^m; decoding uses sumcheck over q−1 points.
- **Concatenated RM (Conc. RM)** — queries a degree-t curve over GF(q²)^m; supports larger databases at rate ≈ 1/m! with O(t)-cost decoding.

Both constructions support GF(2^n) fields (XOR-based accumulation) in addition to prime-order fields.

## Requirements

- **Go 1.22+**
- **Julia 1.10+** (optional — only needed for the offline RM encoding step; the online query/decode benchmarks do not require Julia)
  - Dependencies: `encoding/Project.toml` / `encoding/Manifest.toml`
- **C++ compiler with OpenMP** (optional — alternative to Julia for encoding; see `encoding/rme.cpp`)

## Building

```bash
go build ./...
```

For the C++ RM encoder (optional):

```bash
cd encoding && make
```

## Running Tests

All tests are in `pir/pir_test.go`. Run from the `pir/` directory:

```bash
cd pir
go test -v -run TestEndToEnd
go test -v -run TestEndToEndFromConfigKey
```

`TestQueryFromServer` requires a running server and is skipped automatically if unreachable.

## Benchmarks

All benchmarks are in `pir/pir_test.go`. Run from the `pir/` directory:

```bash
cd pir
```

### RM Query Generation (prime field)

```bash
go test -bench BenchmarkGenerateQuery       # 2D, m=2
go test -bench BenchmarkGenerate3DQuery     # 3D, m=3
go test -bench BenchmarkGenerate4DQuery     # 4D, m=4
```

### RM Query Generation (GF(2^n))

```bash
go test -bench BenchmarkQueryGenGF2n2D      # 2D over GF(2^n)
go test -bench BenchmarkQueryGenGF2n3D      # 3D over GF(2^n)
```

### RM Decoding

```bash
go test -bench BenchmarkDecoding3DQuery     # prime field, m=3
go test -bench BenchmarkDecodingGF2n3D      # GF(2^n), m=3
```

### Concatenated RM (Conc. RM) — Query Generation

Generates the full ℓ = s·q query point set for the Conc. RM construction over GF(q²)^m.

```bash
go test -bench BenchmarkQueryGenConcRM3D -benchtime=1x -timeout=300s
```

This benchmark is slow for large parameters (see table below) because it must stream all ℓ query positions. Increase `-timeout` as needed.

### Concatenated RM — Decoding

```bash
go test -bench BenchmarkDecodeConcRM3D -benchtime=100x
```

Decoding uses Lagrange interpolation on t+1 points and runs in O(t) time (sub-microsecond).

## Parameter Configuration

Parameters are defined in `pir/pir_test.go`. To evaluate different field sizes or curve degrees, edit the relevant `params` table at the top of the benchmark group. Key parameters:

| Symbol | Meaning |
|--------|---------|
| `n` | GF(2^n) field degree |
| `q = 2^n` | Field size |
| `t` | Curve degree |
| `m` | Database dimension (2 = matrix, 3 = cube) |
| `s = (q−1)t + 1` | Number of curve evaluation points |
| `ℓ = s·q` | Total query positions (Conc. RM) |

### Conc. RM default parameters (`paramsConcRM3D`)

| n | q | t | s | ℓ | Query gen | Decode |
|---|---|---|---|---|-----------|--------|
| 13 | 8192 | 7 | 57338 | 470M | ~820ms | <1µs |
| 14 | 16384 | 6 | 98299 | 1.6B | ~2.8s | <1µs |
| 16 | 65536 | 5 | 327676 | 21.5B | ~38s | <1µs |
| 16 | 65536 | 6 | 393211 | 25.8B | ~47s | <1µs |

Query gen is dominated by the φ-projection (O(s·q) GF(q) operations, single-threaded). Decode uses only t+1 ≤ 8 Lagrange weights and takes O(t) GF(q²) multiplications.

## Field Arithmetic

- **`utils/gf2n.go`** — GF(2^n) via log/exp tables. Supported degrees: 4, 8, 13, 14, 16, 17, 18, 20.
- **`utils/gf2n_ext.go`** — GF(q²) = GF(q)[β]/(β²+β+C) degree-2 extension; C is chosen automatically with Tr(C)=1.

## Configuration File

`config.json` stores base64-encoded keys for the permutator and encryptor (used in the offline encoding and online query phases):

```json
{
  "EncryptorKey": "<base64>",
  "PermutatorKey": "<base64>",
  "PermutatorTweak": "<base64>",
  "ServerUrl": "http://..."
}
```

Generate fresh keys by calling `p.Gen()` and saving the output, or use the provided `config.json` for reproducing the test vectors.

## Project Structure

```
pir/          — PIR protocol (Gen, Encode, Query, Decode)
utils/        — Field arithmetic, permutator, encryptor, matrix I/O
encoding/     — Offline RM encoder (Julia + C++)
input/        — Example database CSVs
output/       — Encoded matrix CSVs
server/       — Server-side query handler (Julia)
```
