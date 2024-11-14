# Fast Secret-Key PIR

This repository provides an efficient implementation of a Secret Key Private Information Retrieval (PIR) scheme using Permuted Reed Muller Code. PIR schemes allow clients to retrieve data from a server without revealing which data was retrieved. This implementation is designed to handle messages where each element does not exceed the prime `q`. The core functionalities include:

- **Gen**: Generates a secret key necessary for the PIR protocol.
- **Encode**: Encodes messages using a predefined Reed Muller Code with a secret permutation. This encoded data is then sent to the server.
- **Query**: Generates queries for specific indexes within the Reed Muller code space, forming curves essential for decoding. These queries are sent to the server, which executes lookups and sums the queried entries.
- **Decode**: Decodes the server’s response using the secret key.

## Software Requirements
- **Julia 1.10.3**: Dependencies specified in `encoding/Manifest.toml`.
- **Go 1.22.3**

## Configuration
Configuration parameters are specified in a JSON-structured config file in `config.json`:

```json
{
  "EncryptorKey": "value",
  "PermutatorKey": "value",
  "PermutatorTweak": "value",
  "ServerUrl": "value"
}
```

## Testing
The library includes integration tests to verify functionality and benchmark tests to measure performance across different parameters. You can run the tests with the following commands:

### Integration Tests
```bash
go test -run TestEndToEnd
go test -run TestEndToEndFromConfigKey
go test -run TestQueryFromServer
```

### Benchmarks
```bash
go test -bench BenchmarkEncode
go test -bench BenchmarkGenerateQuery
go test -bench BenchmarkGenerate3DQuery
go test -bench BenchmarkGenerate4DQuery
go test -bench BenchmarkDecodingLargeRecorddQuery
go test -bench BenchmarkDecoding3DQuery
go test -bench BenchmarkClientComputation
go test -bench BenchmarkCodewordPermutation
```

## Example Usage
Below is an example to encode and decode using this library. Assume `main.go` is set up as follows:

### Encoding
```go
package main

import "fmt"

func main() {
    q, k, m := 31, 2, 2
    p := pir.NewPIR(pir.Params{Q: uint64(q), K: uint8(k), M: uint8(m)})
    configFilename := "../config.json"
    pir.GenFromConfig(configFilename)

    input, output := "../input/example_db.csv", "../output/matrix.csv"
    pir.Encode(input, output)
}
```

### Decoding
```go
package main

import (
    "fmt"
    "https://github.com/Caicai-Chen/FastSecretStatePIR"
)

func main() {
    q, k, m := 31, 2, 2
    p := pir.NewPIR(pir.Params{Q: uint64(q), K: uint8(k), M: uint8(m)})
    configFilename := "../config.json"
    output := "../output/matrix.csv"

    p.GenFromConfig(configFilename)

    target_position := 28
    sum, points := p.Query(target_position, utils.GetParameterConfig(configFilename).ServerUrl)
    target_value := p.Decode(sum, points)
    fmt.Println("Output target value: ", target_value)

    // Additional local database query for sanity check
    for i := 0; i < int(math.Pow(float64(q), float64(m))); i++ {
        sum, points := p.QueryLocal(i, output)
        dec := p.Decode(sum, points)
    }
}
```

In this example, the encoded data is sent to the server, which returns a sum that the client decodes to retrieve the target data without revealing the specific position and value. The additional query loop verifies the integrity of the decoding process across all positions.

This implementation leverages Permuted Reed Muller Codes for efficiency and security in PIR. Benchmarks show that it performs effectively even with larger data, although complexity increases with the degree of Reed Muller codes and dimensionality.
