package utils

import (
	"sync"
	"testing"
	"time"
)

func TestRunSafeProbeNormal(t *testing.T) {
	var wg sync.WaitGroup
	called := false
	wg.Add(1)
	go runSafeProbe(&wg, func() {
		called = true
	})

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("runSafeProbe did not complete")
	}

	if !called {
		t.Fatal("expected wrapped function to run")
	}
}

func TestRunSafeProbeRecoverPanic(t *testing.T) {
	var wg sync.WaitGroup
	wg.Add(1)
	go runSafeProbe(&wg, func() {
		panic("boom")
	})

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("runSafeProbe did not recover panic")
	}
}
