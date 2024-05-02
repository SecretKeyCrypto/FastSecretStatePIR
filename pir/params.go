package pir

import (
	"encoding/base64"
	"fmt"
	"rme/utils"
)

type Params struct {
	Q uint64
	K uint8
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
