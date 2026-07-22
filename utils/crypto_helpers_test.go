package utils

import (
	"sync"
	"testing"
)

func TestEncryptorRejectsNonpositiveModulus(t *testing.T) {
	key := make([]byte, 16)
	if _, err := NewEncryptor(key, 0); err == nil {
		t.Fatal("NewEncryptor accepted modulus zero")
	}
}

func TestPermutatorSupportsConcurrentQueries(t *testing.T) {
	key := make([]byte, 16)
	tweak := make([]byte, 8)
	permutator, err := NewPermutator(257, key, tweak)
	if err != nil {
		t.Fatalf("NewPermutator returned error: %v", err)
	}

	want := make([]uint64, 64)
	for i := range want {
		want[i], err = permutator.Permute(uint64(i))
		if err != nil {
			t.Fatalf("Permute(%d) returned error: %v", i, err)
		}
	}

	var wait sync.WaitGroup
	errors := make(chan int, len(want))
	for i := range want {
		wait.Add(1)
		go func(index int) {
			defer wait.Done()
			got, err := permutator.Permute(uint64(index))
			if err != nil || got != want[index] {
				errors <- index
			}
		}(i)
	}
	wait.Wait()
	close(errors)
	for index := range errors {
		t.Errorf("concurrent Permute(%d) did not match its sequential result", index)
	}
}
