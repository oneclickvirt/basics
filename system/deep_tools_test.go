package system

import (
	"context"
	"errors"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestDeepToolsSkipWithoutExplicitTarget(t *testing.T) {
	if result := RunSMARTSelfTest(context.Background(), ""); result.Status != "skipped" {
		t.Fatalf("SMART status = %q", result.Status)
	}
	if result := RunGPUCompute(context.Background(), ""); result.Status != "skipped" {
		t.Fatalf("GPU status = %q", result.Status)
	}
}

func TestRunSMARTSelfTestRejectsUnsafeDevice(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Linux-only validation")
	}
	called := false
	result := runSMARTSelfTest(context.Background(), "/dev/disk/by-id/unsafe", func(context.Context, string, ...string) ([]byte, error) {
		called = true
		return nil, nil
	})
	if called || result.Status != "error" {
		t.Fatalf("unsafe SMART device executed: called=%t result=%+v", called, result)
	}
}

func TestRunGPUComputeUsesExplicitSelector(t *testing.T) {
	result := runGPUCompute(context.Background(), "fixture", func(_ context.Context, name string, args ...string) ([]byte, error) {
		if name != "clpeak" || len(args) != 2 || args[0] != "--device" || args[1] != "fixture" {
			t.Fatalf("unexpected GPU command: %s %v", name, args)
		}
		time.Sleep(2 * time.Millisecond)
		return []byte("compute result"), nil
	})
	if result.Status != "ok" || result.Output != "compute result" {
		t.Fatalf("unexpected GPU result: %+v", result)
	}
	if result.DurationMS <= 0 {
		t.Fatalf("GPU duration was not recorded: %+v", result)
	}
}

func TestPollSMARTSelfTestCompletesFixture(t *testing.T) {
	result := pollSMARTSelfTest(context.Background(), "/dev/sda", DeepToolResult{SchemaVersion: "goecs.smart/selftest-v1", Target: "/dev/sda"}, func(_ context.Context, name string, args ...string) ([]byte, error) {
		if name != "smartctl" || len(args) != 2 || args[0] != "-c" {
			t.Fatalf("unexpected SMART poll: %s %v", name, args)
		}
		return []byte("Self-test execution status: completed"), nil
	}, time.Millisecond)
	if result.Status != "ok" || !strings.Contains(result.Output, "completed") {
		t.Fatalf("unexpected SMART completion: %+v", result)
	}
}

func TestDeepToolClassifiesUnavailableCommand(t *testing.T) {
	result := runGPUCompute(context.Background(), "fixture", func(context.Context, string, ...string) ([]byte, error) {
		return nil, errors.New("clpeak is unavailable: fixture")
	})
	if result.Status != "unavailable" {
		t.Fatalf("status = %q", result.Status)
	}
}
