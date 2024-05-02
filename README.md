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

# Example Usage
Copy the following code into a file called main.go, and run it with `go run main.go`

## Encoding
```
package main

import (
    "fmt"
    "https://github.com/Caicai-Chen/FastSecretStatePIR"
)

func main() {
    q := 31
	d := 2
	pir := NewPIR(Params{uint64(q), uint8(d)})
	configFilename := "../config.json"
	pir.GenFromConfig(configFilename)

	input := "../input/example_db.csv"
	output := "../output/matrix.csv"

	pir.Encode(input, output)
}
```

Then Copy the output file onto server where it can read the matrix and output the sumation of queried points.

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
	d := 2
	pir := NewPIR(Params{uint64(q), uint8(d)})
	configFilename := "../config.json"
	pir.GenFromConfig(configFilename)


    target_position := 28
	sum, points := pir.Query(target, utils.GetParameterConfig(configFilename).ServerUrl)
	target_value := pir.Decode(sum, points)
    fmt.Println("Output target value: ", target_value)
}
```
