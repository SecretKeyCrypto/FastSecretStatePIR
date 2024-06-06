# FastSecretStatePIR
This library provides an implentation of a Private Information Retrieval Protocol based on Permuted Reed Muller Encoding.

# Configuration
The implementation supports reading parameters from the config file which contains the json structure as follows. 
```
type Config struct {
	EncyrptorKey    string `json:"encryptorKey"`
	PermutatorKey   string `json:"permutatorKey"`
	PermutatorTweak string `json:"permutatorTweak"`
	ServerUrl       string `json:"serverUrl"`
}
```

# Testing
To run end-to-end integration test
1. go test -run TestEndToEnd
2. go test -run TestEndToEndFromConfigKey
3. go test -run TestQueryFromServer

To run benchmarks regarding to the query process 
1. go test -bench BenchmarkEncode
2. go test -bench BenchmarkGenerateQuery
3. go test -bench BenchmarkGenerate3DQuery
4. go test -bench BenchmarkGenerate4DQuery
5. go test -bench BenchmarkDecodingLargeRecorddQuery
6. go test -bench BenchmarkDecoding3DQuery
7. go test -bench BenchmarkClientComputation

# Example Usage
Copy the following code into a file called main.go, and run it with `go run main.go`

## Encoding
```
package main

import (
    "fmt"
)

func main() {
    q := 31
	k := 2
	m := 2

	p := pir.NewPIR(pir.Params{Q: uint64(q), K: uint8(k), M: uint8(m)})
	configFilename := "../config.json"
	pir.GenFromConfig(configFilename)

	input := "../input/example_db.csv"
	output := "../output/matrix.csv"

	pir.Encode(input, output)
}
```

After sending the output file onto server where it can read the matrix and output the sumation of queried points.

The request from the clients will be a HTTP request contains a list of positions in the RM codeword space.

## Decoding
```
package main

import (
    "fmt"
    "https://github.com/Caicai-Chen/FastSecretStatePIR"
)

func main() {
    q := 31
	k := 2
	m := 2

	p := pir.NewPIR(pir.Params{Q: uint64(q), K: uint8(k), M: uint8(m)})

	configFilename := "../config.json"
	pir.GenFromConfig(configFilename)


    target_position := 28
	sum, points := pir.Query(target, utils.GetParameterConfig(configFilename).ServerUrl)
	target_value := pir.Decode(sum, points)
    fmt.Println("Output target value: ", target_value)

	// We also support query from local database for sanity check
	for i := 0; i < int(math.Pow(float64(q), float64(m))); i++ {
		sum, points := p.QueryLocal(i, output)
		dec := p.Decode(sum, points)
	}
}
```
