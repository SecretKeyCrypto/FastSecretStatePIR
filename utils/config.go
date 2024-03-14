package utils

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
)

const (
	filename = "../config.json"
)

type Config struct {
	Q                int              `json:"q"`
	K                int              `json:"k"`
	EncyrptorKey     string           `json:"encryptorKey"`
	PermutatorConfig PermutatorConfig `json:"permutatorConfig"`
}

type PermutatorConfig struct {
	Key    string `json:"key"`
	Tweak  string `json:"tweak"`
	MaxNum int    `json:"maxNum"`
	MinNum int    `json:"minNum"`
	Radix  int    `json:"radix"`
	MinLen int    `json:"minLen"`
	MaxLen int    `json:"maxLen"`
}

func WriteConfigToFile(config Config) error {
	jsonData, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(filename, jsonData, 0644)
}

func ReadConfigFromFile(filename string) (Config, error) {
	var config Config

	jsonData, err := os.ReadFile(filename)
	if err != nil {
		return config, err
	}

	err = json.Unmarshal(jsonData, &config)
	return config, err
}

func GetParameterConfig() Config {
	keyLength := 16

	conf, err := ReadConfigFromFile(filename)
	if err != nil {
		fmt.Println("Error reading config from file:", err)
		panic(err)
	}

	if conf.Q == 0 || conf.K == 0 {
		panic("Field Size and degree of the curve is required")
	}

	if conf.PermutatorConfig.Key == "" {
		key, err := GenerateRandomKey(keyLength)

		if err != nil {
			fmt.Println("Error Generating Permuator Key:", err)
			panic(err)
		}

		conf.PermutatorConfig.Key = base64.StdEncoding.EncodeToString(key)
	}

	if conf.PermutatorConfig.Tweak == "" {
		tweak, _ := GenerateRandomKey(keyLength / 2)

		if err != nil {
			fmt.Println("Error Generating Permuator Tweak:", err)
			panic(err)
		}

		conf.PermutatorConfig.Tweak = base64.StdEncoding.EncodeToString(tweak)
	}

	if conf.EncyrptorKey == "" {
		key, err := GenerateRandomKey(keyLength)

		if err != nil {
			fmt.Println("Error Generating Encryptor Key:", err)
			panic(err)
		}

		conf.EncyrptorKey = base64.StdEncoding.EncodeToString(key)
	}

	if err := WriteConfigToFile(conf); err != nil {
		fmt.Println("Error writing config to file:", err)
	}

	return conf
}
