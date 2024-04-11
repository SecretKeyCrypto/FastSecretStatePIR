package utils

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
)

const (
	filename    = "../config.json"
	keyLength   = 16
	tweakLength = 8
)

type Config struct {
	Q               int    `json:"q"`
	K               int    `json:"k"`
	EncyrptorKey    string `json:"encryptorKey"`
	PermutatorKey   string `json:"permutatorKey"`
	PermutatorTweak string `json:"permutatorTweak"`
	ServerUrl       string `json:"serverUrl"`
}

type PermutatorConfig struct {
	Key    string `json:"key"`
	Tweak  string `json:"tweak"`
	MaxNum int    `json:"maxNum"`
	Radix  int    `json:"radix"`
	MaxLen int    `json:"maxLen"`
}

func (config *Config) Update() error {
	jsonData, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(filename, jsonData, 0644)
}

func ReadConfigFromFile(filename string) (*Config, error) {
	var config Config

	jsonData, err := os.ReadFile(filename)
	if err != nil {
		return &config, err
	}

	err = json.Unmarshal(jsonData, &config)
	return &config, err
}

func GetParameterConfig() *Config {
	conf, err := ReadConfigFromFile(filename)
	if err != nil {
		fmt.Println("Error reading config from file:", err)
		panic(err)
	}

	if conf.PermutatorKey == "" {
		key, err := GenerateRandomKey(keyLength)

		if err != nil {
			fmt.Println("Error Generating Permuator Key:", err)
			panic(err)
		}

		conf.PermutatorKey = base64.StdEncoding.EncodeToString(key)
	}

	if conf.PermutatorTweak == "" {
		tweak, _ := GenerateRandomKey(tweakLength)

		if err != nil {
			fmt.Println("Error Generating Permuator Tweak:", err)
			panic(err)
		}

		conf.PermutatorTweak = base64.StdEncoding.EncodeToString(tweak)
	}

	if conf.EncyrptorKey == "" {
		key, err := GenerateRandomKey(keyLength)

		if err != nil {
			fmt.Println("Error Generating Encryptor Key:", err)
			panic(err)
		}

		conf.EncyrptorKey = base64.StdEncoding.EncodeToString(key)
	}

	if err := conf.Update(); err != nil {
		fmt.Println("Error writing config to file:", err)
	}

	return conf
}
