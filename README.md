# Fast Secret-Key PIR
This repository provides an implementation of a Secret Key Private Information Retrieval (PIR) scheme using Permuted Reed Muller Code. It is designed to handle messages where each element does not exceed the prime `q`. The library includes functionalities for:

- **Gen**: Generates a secret key necessary for the PIR protocol.
- **Encode**: Encodes messages using a predefined-size Reed Muller Code with a secret permutation. The encoded data is transmitted to the server.
- **Query**: Generates queries for specific indexes in the Reed Muller code space, creating curves essential for decoding targeted points. These queries are sent to the server, which then executes lookups and sums the queried entries.
- **Decode**: Decodes the server's response using the secret key.

## Software Requirements
- **Julia 1.10.3**: Dependencies are specified in `encoding/Manifest.toml`.
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
Run integration tests and benchmarks using the following commands:

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
