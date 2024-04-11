package pir

import (
	"encoding/base64"
	"fmt"
	"rme/utils"
)

type Params struct {
	Q         uint64
	K         uint8
	PermKey   []byte
	PermTweak []byte
	EncKey    []byte
	ServerUrl string
}

func GetParamsFromConfig() Params {
	var par Params
	config := utils.GetParameterConfig()
	par.Q = uint64(config.Q)
	par.K = uint8(config.K)
	par.ServerUrl = config.ServerUrl

	key, err := base64.StdEncoding.DecodeString(config.PermutatorKey)
	if err != nil {
		fmt.Println("Error decoding Permutation Key:", err)
		panic(err)
	}
	par.PermKey = key

	tweak, err := base64.StdEncoding.DecodeString(config.PermutatorTweak)
	if err != nil {
		fmt.Println("Error decoding Permutation Tweak:", err)
		panic(err)
	}

	par.PermTweak = tweak

	encKey, err := base64.StdEncoding.DecodeString(config.EncyrptorKey)
	if err != nil {
		fmt.Println("Error decoding Encryption Key:", err)
		panic(err)
	}

	par.EncKey = encKey
	return par
}

func (par Params) UpdateConfig() error {
	config := utils.GetParameterConfig()

	config.PermutatorKey = base64.StdEncoding.EncodeToString(par.PermKey)
	config.PermutatorTweak = base64.StdEncoding.EncodeToString(par.PermTweak)
	config.EncyrptorKey = base64.StdEncoding.EncodeToString(par.EncKey)

	return config.Update()
}
