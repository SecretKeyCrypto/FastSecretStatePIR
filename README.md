# sk-DEPIR: Secret-Key Doubly Efficient Private Information Retrieval

Implementation of secret-key Doubly Efficient PIR (sk-DEPIR) schemes based on permuted smooth LDCs. It contains two client constructions:

- **sk-PIR based on curve-lifted RS with curve decoding (Lift-RS)** - The LDC is the curve-lifted `(q-2,q)` Reed-Solomon code. The decoder queries the other `q-1` points on a degree-`t` curve through the requested location.
- **sk-PIR based on RM with concatenated-curve decoding (RM-Conc)** - The LDC is a standard Reed-Muller code. The decoder uses an outer degree-`t` curve over an extension field and inner degree-`(e-1)` curves over the base field.

The Go implementations are named `LiftedRS` and `RMConc`. Both satisfy the common `PIR` interface in `pir/pir_api.go`, including decoding:

```go
type PIR interface {
    Gen()
    Encode(input, output string) (Matrix, error)
    Query(index int, serverURL string) QueryResult
    Decode(QueryResult) ([]int, error)
    Close() error
}
```

`Params.Q`, `Params.K`, and `Params.M` denote the field size `q`, curve degree `t`, and database dimension `m`. `Params.D` is the RM degree bound `d` used by RM-Conc; `NewRMConc` interprets zero as the compatibility default `d=q-1`.

`NewPIR` is retained as a compatibility alias for `NewLiftedRS`. New code should call `NewLiftedRS` or `NewRMConc` explicitly.

## Implementation status and relaxations

The online algorithms and the offline encoders do not have the same completeness. In particular, **there is no integrated Lift-RS encoder**. `encoding/rme.cpp` retains an experimental `LRSE2D` helper for future work, but the encoder command protocol never selects it and `LiftedRS.Encode` calls RMEncoding instead.

| Construction | Query generation and decode | `Encode` method | End-to-end correctness coverage | Main relaxation or gap |
|---|---|---|---|---|
| Lift-RS (`LiftedRS`) | Implemented for the specialization `ℓ=q-1`, with `m=2,3`; prime fields and the listed `GF(2^n)` fields | Calls the shared **RMEncoding** wrapper | Prime-field tests call `Encode`; a separate `GF(16)` test uses a directly evaluated low-degree codeword | General locality `ℓ` and a genuine Lift-RS encoder are not implemented. The RM-encoded placeholder validates the online protocol only on codewords that also satisfy the Lift-RS curve restriction; it does not validate Lift-RS encoding, rate, storage overhead, or encoding time. |
| RM-Conc (`RMConc`) | General supported parameters are implemented for `m=3`, `e=2`, `q=2^n`, and configurable `1 <= d <= q-1`, using `s=dt+1` and `r=d+1` | Also calls the shared **RMEncoding** wrapper | A `GF(16)` end-to-end test directly evaluates a genuine degree-2 RM codeword, then runs permutation, masking, query generation, local server evaluation, and decode | The external encoder paths follow prime-modulus/prime-field conventions and do not share this repository's `GF(2^n)` element representation. Consequently, there is currently no compatible RM-Conc end-to-end test that calls `Encode`. |

Shared implementation choices and limitations:

- `NewRMConc` uses `d=q-1` when `Params.D` is zero. Smaller `d` values are supported and tested.
- The manuscript locality `ℓ` counts genuine LDC positions only. Production `Query` and `QueryLocal` calls append 128 dummy positions, so their transmitted query length is `L=ℓ+128`.
- The extension degree is fixed to `e=2`; higher extension degrees from the manuscript are not implemented.
- RM-Conc currently supports only three-dimensional databases and binary extension base fields.
- Curve sampling, dummy positions, and shuffling use Go's `math/rand`. This is adequate for experiments and benchmarks but is not a cryptographic randomness source. A production security implementation must use a CSPRNG.
- Local end-to-end tests replace the HTTP server with file-backed query evaluation. `TestQueryFromServer` separately exercises the network path, but it is a smoke test and does not compare decoded values with an expected database.

## Future work

The Lift-RS construction is defined for general locality `ℓ`, but the current implementation supports only the optimized `ℓ=q-1` case: it queries every point on the base-field curve except the point corresponding to the requested location and uses the resulting full-field sum identity for decoding, avoiding the need for general interpolation. Supporting general `ℓ` requires retaining the selected curve parameters and target parameter with each query, then performing general interpolation over either a prime field or `GF(2^n)`. The product-tree interpolation machinery currently used by RM-Conc can be refactored behind field-generic arithmetic and reused for this purpose.

RM-Conc already supports general `d` within its implemented scope (`m=3`, `e=2`, and `q=2^n`): query generation and decoding derive and use all `s=dt+1` outer blocks and `r=d+1` inner points. Extending RM-Conc beyond that scope, particularly to other extension degrees `e`, remains future work.

## Requirements

- **Go 1.21.6**
- A **C++17 compiler** to build the preferred external RM encoder, or **Julia 1.10+** for its fallback implementation
  - Julia dependencies are recorded in `encoding/Project.toml` and `encoding/Manifest.toml`.

The external encoder is not needed for the direct `GF(16)` end-to-end tests or the online query/decode benchmarks. It is required by `BenchmarkEncode` and by the two end-to-end tests whose names contain `WithRMEncoding`.

## Building

```bash
go build ./...
```

To use the shared RMEncoding path and run the complete test suite, build the C++ encoder (or install the Julia fallback):

```bash
make -C encoding rme
```

## Running tests

Tests are in `pir/pir_api_test.go`, `pir/pir_test.go`, `pir/db_test.go`, and the `utils` package. Run from the repository root:

```bash
go test ./...
```

### End-to-end and integration-test coverage

| Test | Construction | Calls `Encode`? | Database/codeword used | Server path | What it establishes |
|---|---|---:|---|---|---|
| `TestLiftedRSWithRMEncodingEndToEnd` | Lift-RS online path | Yes | Degree-2 RM codeword over prime field `F_31`, used as a Lift-RS-compatible placeholder | Local CSV | Exhaustively checks all `q^2` codeword locations through query and decode. It does **not** test a Lift-RS encoder. |
| `TestLiftedRSWithRMEncodingEndToEndFromConfigKey` | Lift-RS online path | Yes | Degree-3 RM codeword over `F_31`, with keys loaded from `config.json` | Local CSV | Same online check using configured keys. It writes the example input/output CSV files while running; it still does not test a Lift-RS encoder. |
| `TestLiftedRSEndToEndWithGF2nCodeword` | Lift-RS online path | No | Directly evaluated degree-2 polynomial over `GF(16)` whose curve restrictions have degree below `q-1` | Temporary local CSV | Tests binary-field curve generation, permutation, masking, dummy handling, and decode. No offline encoder is exercised. |
| `TestRMConcEndToEndWithGF2nCodeword` | RM-Conc | No | Directly evaluated genuine degree-2 RM codeword over `GF(16)` | Temporary local CSV | Tests the production variable-`d` query and fast-weight decode path at selected locations. No external encoder or HTTP server is exercised. |
| `TestQueryFromServer` | Lift-RS online path | No | Assumes a compatible database is already served | HTTP | Network smoke test only; it skips when the configured server is unavailable and does not assert the decoded values. |

Additional component tests compare the fast outer interpolation weights with the retained `O(s^2)` reference implementation, exercise arbitrary inner evaluation points and the full-outer-field sum shortcut, and verify that RM-Conc uses `s=dt+1` outer blocks and `r=d+1` inner points for `e=2`.

The encoder-backed tests write `input/db.csv`, `output/matrix.csv`, `input/example_db.csv`, and `output/example_matrix.csv`. Build `encoding/rme` first, or install the Julia fallback, before running the complete suite. The direct online tests do not require either encoder.

## Benchmarks

Most benchmarks are in `pir/pir_test.go`; the fast-versus-reference interpolation benchmark is in `utils/gf2n_ext_poly_test.go`. The benchmark groups deliberately isolate client components rather than running a database server end to end.

| Benchmark group | Timed work | Deliberately excluded or simplified |
|---|---|---|
| `BenchmarkEncode` | External RMEncoding plus secret permutation and masking, for a small prime-field instance | This is **RM encoding**, even though `NewPIR` selects the Lift-RS online construction. It is not a Lift-RS encoding benchmark. File I/O and the external process are included. |
| `BenchmarkQueryGenLiftedRS2D`, `BenchmarkQueryGen3D_t5`, `BenchmarkQueryGen3D_t6`, `BenchmarkQueryGenLiftedRS3D_q13_t7` | Curve generation, secret permutation, 128 dummy positions, and shuffle | No database encoding, server evaluation, response transfer, or network. Prime-field and binary-field cases use different field implementations. |
| Corresponding `BenchmarkDecode...` groups | Lookup of genuine positions in a shuffled query, mask removal, and client combination | Responses are random mock values. Query generation, server work, network, and correctness checking are outside the timed loop. |
| `BenchmarkQueryGenConcRM3D` | Outer-curve sampling/evaluation, all `ℓ=sr` inner projections, 128 dummy samples, and fast outer-weight precomputation | Uses `d=q-1`, `e=2`; streams positions into a checksum instead of allocating the transmitted query. It does **not** apply the secret permutation, materialize/shuffle the query, contact a server, or encode a database. |
| `BenchmarkDecodeConcRM3D` | Direct `φ^-1` processing of all `s` blocks plus the outer weighted sum | Uses small parameters (`q=16,d=3` and `q=256,d=5`) and random mock blocks. Outer weights are prepared before timing. It excludes query lookup, dummy removal, mask removal, server/network work, and large Table 2 parameters. |
| `BenchmarkLagrangeWeightsAt0Implementations` | Fast product-tree weight generation versus the retained `O(s^2)` reference | Isolated arithmetic microbenchmark; not a complete query or decode benchmark. |

Several older microbenchmarks remain in `pir/pir_test.go`; some explicitly use `noiseCount=0`. Use the named LiftedRS groups below when measuring the current 128-dummy online path.

Run the principal client benchmarks from the repository root:

```bash
go test ./pir -bench 'Benchmark(QueryGenLiftedRS2D|DecodeLiftedRS2D|QueryGen3D_t5|Decode3D_t5|QueryGenConcRM3D|DecodeConcRM3D)' -run '^$'
```

Run the shared RMEncoding benchmark separately only when an external encoder is available:

```bash
go test ./pir -run '^$' -bench BenchmarkEncode
```

### Lift-RS query generation

```bash
go test ./pir -run '^$' -bench BenchmarkQueryGenLiftedRS2D    # m=2, prime + GF(2^n)
go test ./pir -run '^$' -bench BenchmarkQueryGen3D_t5         # m=3, t=5
go test ./pir -run '^$' -bench BenchmarkQueryGen3D_t6         # m=3, t=6
go test ./pir -run '^$' -bench BenchmarkQueryGenLiftedRS3D_q13_t7 # m=3, q=2^13, t=7
```

These groups append 128 dummy positions and shuffle all `L=(q-1)+128` positions. In the manuscript's notation, `ℓ=q-1` is the LDC locality and `L=ℓ+128` is the transmitted length used here. Labels that show database sizes assume the near-rate-one Lift-RS storage regime; they are parameter-derived estimates, not measurements from an implemented Lift-RS encoder.

### Lift-RS decoding

```bash
go test ./pir -run '^$' -bench BenchmarkDecodeLiftedRS2D # m=2, prime + GF(2^n)
go test ./pir -run '^$' -bench BenchmarkDecode3D_t5  # m=3, t=5
go test ./pir -run '^$' -bench BenchmarkDecode3D_t6  # m=3, t=6
```

The timed decode loop builds a lookup from shuffled positions to mock responses, recovers the `q-1` genuine responses, removes their masks, and performs the field combination. It does not verify a database value during benchmarking.

### RM-Conc — Query Generation

Generates the query positions for RM-Conc over `GF(q²)^3`. For RM degree `d` and extension degree `e=2`, Figure 4 uses

- `s = dt+1` outer evaluation points;
- `r = d(e-1)+1 = d+1` inner base-field points per outer point; and
- locality `ℓ = sr`, excluding dummy queries.

The production transmitted query has `L=ℓ+128` positions. Only at the default `d=q-1` do we have `r=q` and hence `ℓ=sq`. The benchmark uses precisely this default-`d` case, but streams the genuine and dummy positions into a checksum. It therefore measures their generation without the otherwise prohibitive query allocation, secret permutation, and shuffle.

```bash
go test ./pir -run '^$' -bench BenchmarkQueryGenConcRM3D -benchtime=1x -timeout=300s
```

This benchmark is slow for large parameters (see table below) because it must stream all ℓ query positions. Increase `-timeout` as needed.

### RM-Conc — Decoding

```bash
go test ./pir -run '^$' -bench BenchmarkDecodeConcRM3D -benchtime=100x
```

Production decoding uses all `s=dt+1` outer blocks, not only `t+1`. It first applies the inner inverse map `φ^-1` to each block of `r=d+1` responses and then evaluates the degree-at-most-`dt` outer polynomial at zero. General-`d` inner interpolation currently costs `O(sr^2)`; when `d=q-1`, the full-base-field `PhiInv` identity reduces this step to `O(sq)`. Outer interpolation normally uses query-time precomputed weights and costs `O(s)` online. When `s=q²-1`, query generation instead enumerates every nonzero element of `GF(q²)` and omits the nodes and weights; decoding uses the characteristic-two identity `f(0)=Σ_{z∈GF(q²)*}f(z)` for `deg(f)≤q²-2`.

The decode benchmark is intentionally not run at the huge default-`d` Table 2 sizes. It times the arithmetic kernel on smaller variable-`d` instances with mock response blocks and excludes permutation lookup, dummy filtering, and mask removal.

## Parameter Configuration

Parameters are defined in `pir/pir_test.go`. To evaluate different field sizes or curve degrees, edit the relevant `params` table at the top of the benchmark group. Key parameters:

| Symbol | Meaning |
|--------|---------|
| `n` | GF(2^n) field degree |
| `q = 2^n` | Field size |
| `t` | Outer curve degree |
| `m` | Database dimension (2 = matrix, 3 = cube) |
| `d` | RM polynomial degree bound (`q-1` by default) |
| `e` | Extension degree (fixed to 2 in this implementation) |
| `s = dt + 1` | Number of outer curve evaluation points |
| `r = d(e-1)+1 = d+1` | Inner evaluation points per outer point |
| `ℓ = s·r` | Manuscript locality: non-dummy query positions |
| `L = ℓ+128` | Total transmitted query positions in this implementation |

### Representative default-`d` RM-Conc parameter sizes

| n | q | t | d | s | r | ℓ |
|---|---:|---:|---:|---:|---:|---:|
| 13 | 8192 | 7 | 8191 | 57338 | 8192 | 469,712,896 |
| 14 | 16384 | 6 | 16383 | 98299 | 16384 | 1,610,530,816 |
| 16 | 65536 | 5 | 65535 | 327676 | 65536 | 21,474,574,336 |
| 16 | 65536 | 6 | 65535 | 393211 | 65536 | 25,769,476,096 |

These are default-`d` sizes, not stored timing claims. Re-run the benchmarks on the target machine for current measurements. Query generation is dominated by producing `ℓ=sr` positions. Default-`d` decoding uses `O(sq)` full-field `φ⁻¹` work followed by an `O(s)` weighted outer sum.

Additional parameter selections are defined in `paramsConcRM3D` in `pir/pir_test.go`.

## Field Arithmetic

- **`utils/gf2n.go`** — GF(2^n) via log/exp tables. Supported degrees: 4, 8, 12, 13, 14, 16, 17, 18, 20.
- **`utils/gf2n_ext.go`** — GF(q²) = GF(q)[β]/(β²+β+C) degree-2 extension; C is chosen automatically with Tr(C)=1.
- **`utils/gf2n_ext_poly.go`** — polynomial operations used by fast outer interpolation-weight precomputation. The original `O(s²)` implementation remains available as a reference.

## Configuration File

`config.json` stores base64-encoded keys for the permutator and encryptor (used in the offline encoding and online query phases):

```json
{
  "encryptorKey": "<base64>",
  "permutatorKey": "<base64>",
  "permutatorTweak": "<base64>",
  "serverUrl": "http://..."
}
```

`GenFromConfig` loads these values. The configuration helper fills in and persists missing key fields; `Gen` instead creates ephemeral in-memory keys.

## Project Structure

```
pir/          — common PIR API, LiftedRS, RMConc, and RMEncoding wrapper
utils/        — field arithmetic, curve generation, permutation, masks, and matrix I/O
encoding/     — Offline RM encoder (Julia + C++)
input/        — Example database CSVs
output/       — Encoded matrix CSVs
server/       — Server-side query handler (Julia)
```
