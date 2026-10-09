package pir

import (
	"encoding/base64"
	"fmt"

	"rme/utils"
)

// Params contains the public construction parameters.
type Params struct {
	// Q is the base-field size q.
	Q uint64
	// K is the query-curve degree t. The name is retained for API compatibility.
	K uint8
	// M is the database/RM-code dimension.
	M uint8
	// D is the RM polynomial-degree bound d used by RMConc. A zero value passed
	// to NewRMConc selects the default q-2.
	D uint64
}

type Keys struct {
	PermKey   []byte
	PermTweak []byte
	EncKey    []byte
}

func GetKeysFromConfig(filename string) Keys {
	var keys Keys
	config := utils.GetParameterConfig(filename)

	key, err := base64.StdEncoding.DecodeString(config.PermutatorKey)
	if err != nil {
		fmt.Println("Error decoding Permutation Key:", err)
		panic(err)
	}
	keys.PermKey = key

	tweak, err := base64.StdEncoding.DecodeString(config.PermutatorTweak)
	if err != nil {
		fmt.Println("Error decoding Permutation Tweak:", err)
		panic(err)
	}

	keys.PermTweak = tweak

	encKey, err := base64.StdEncoding.DecodeString(config.EncyrptorKey)
	if err != nil {
		fmt.Println("Error decoding Encryption Key:", err)
		panic(err)
	}

	keys.EncKey = encKey
	return keys
}
