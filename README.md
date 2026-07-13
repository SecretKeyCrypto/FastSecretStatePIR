# sk-DEPIR: Secret-Key Doubly Efficient Private Information Retrieval

Implementation of secret-key Doubly Efficient PIR (sk-DEPIR) schemes based on permuted Reed-Muller (RM) codes and their extensions. Supports two main constructions:

- **Lifted-RM PIR (PLDN)** — queries a degree-t curve over F_q^m built on lifted Reed–Solomon codes (rate → 1, low storage overhead). The client uses the **Permuted Low-Degree with Noise (PLDN)** protocol: it injects `λ = 128` random *noise* positions into the q−1 real curve points and shuffles all of them, so the server cannot tell real from fake. The server therefore returns **per-position** values (it cannot aggregate, as noise would corrupt a sum); at decode the client maps the responses back to the real positions, discards the noise, and recovers the record by sumcheck over the q−1 real points. This is the construction used in all benchmarks below.
- **Concatenated RM (Conc. RM)** — queries a degree-t curve over GF(q²)^m; supports larger databases at rate ≈ 1/m! with O(t)-cost decoding.

Both constructions support GF(2^n) fields (XOR-based accumulation) in addition to prime-order fields.

> A no-noise "plain" RM path (server aggregates into a single sum, `Decode`) also exists for comparison, but the PLDN path is the one that provides query privacy and is what the benchmarks and paper tables measure.

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

All Lifted-RM benchmarks inject `noiseCount = 128` (PLDN). Each sub-benchmark covers both prime-order and GF(2^n) fields at matching bandwidth.

### Lifted-RM (PLDN) Query Generation

```bash
go test -bench BenchmarkQueryGenPLDN2D        # m=2, prime + GF(2^n)
go test -bench BenchmarkQueryGen3D_t5         # m=3, t=5
go test -bench BenchmarkQueryGen3D_t6         # m=3, t=6
go test -bench BenchmarkQueryGenPLDN3D_q13_t7 # m=3, q=2^13, t=7
```

Query generation appends the 128 noise positions and shuffles all `L = (q−1) + 128` positions; the noise overhead over the plain path is negligible (≈2%).

### Lifted-RM (PLDN) Decoding

```bash
go test -bench BenchmarkDecodePLDN2D          # m=2, prime + GF(2^n)
go test -bench BenchmarkDecode3D_t5           # m=3, t=5
go test -bench BenchmarkDecode3D_t6           # m=3, t=6
```

The decode time includes the **map-back** step: build a hashmap of the q−1 real positions, scan all `L` server responses to extract the values at real positions (discarding noise), then run the sumcheck. This map-back is O(q) and dominates decode at large q.

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
| 13 | 8192 | 7 | 57338 | 470M | ~820ms | ~220µs |
| 14 | 16384 | 6 | 98299 | 1.6B | ~2.8s | ~320µs |
| 16 | 65536 | 5 | 327676 | 21.5B | ~38s | ~1.0ms |
| 16 | 65536 | 6 | 393211 | 25.8B | ~47s | ~1.0ms |

Query gen is dominated by the φ-projection (O(s·q) GF(q) operations, single-threaded). Decode uses the φ⁻¹ step (O(q·(t+1)) scalar-GF(q²) multiplications using precomputed (β+w)⁻¹ weights) followed by outer Lagrange at 0 (O(t) GF(q²) multiplications).

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
