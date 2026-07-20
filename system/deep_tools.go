package system

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"runtime"
	"strings"
	"time"
)

type DeepToolResult struct {
	SchemaVersion string `json:"schema_version"`
	Status        string `json:"status"`
	Target        string `json:"target,omitempty"`
	DurationMS    int64  `json:"duration_ms"`
	Output        string `json:"output,omitempty"`
	Error         string `json:"error,omitempty"`
}

type deepCommandRunner func(context.Context, string, ...string) ([]byte, error)

func RunSMARTSelfTest(ctx context.Context, device string) DeepToolResult {
	return runSMARTSelfTest(ctx, strings.TrimSpace(device), runDeepCommand)
}

func runSMARTSelfTest(ctx context.Context, device string, runner deepCommandRunner) (result DeepToolResult) {
	result = DeepToolResult{SchemaVersion: "goecs.smart/selftest-v1", Status: "skipped", Target: device}
	if device == "" {
		result.Error = "explicit SMART device is not configured"
		return result
	}
	if runtime.GOOS != "linux" {
		result.Status, result.Error = "unsupported", "SMART self-test is supported on Linux only"
		return result
	}
	if !strings.HasPrefix(device, "/dev/") || strings.ContainsAny(strings.TrimPrefix(device, "/dev/"), "/ \\") {
		result.Status, result.Error = "error", "invalid explicit SMART device"
		return result
	}
	started := time.Now()
	defer func() { result.DurationMS = time.Since(started).Milliseconds() }()
	output, err := runner(ctx, "smartctl", "-t", "short", device)
	if err != nil {
		return failedDeepTool(result, ctx, output, err)
	}
	result.Output = boundedDeepOutput(output)
	return pollSMARTSelfTest(ctx, device, result, runner, 2*time.Second)
}

func pollSMARTSelfTest(ctx context.Context, device string, result DeepToolResult, runner deepCommandRunner, interval time.Duration) DeepToolResult {
	if interval <= 0 {
		interval = 2 * time.Second
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			abortCtx, abortCancel := context.WithTimeout(context.Background(), 5*time.Second)
			_, _ = runner(abortCtx, "smartctl", "-X", device)
			abortCancel()
			return failedDeepTool(result, ctx, nil, ctx.Err())
		case <-ticker.C:
			output, err := runner(ctx, "smartctl", "-c", device)
			if err != nil {
				return failedDeepTool(result, ctx, output, err)
			}
			text := strings.ToLower(string(output))
			if strings.Contains(text, "self-test routine in progress") || strings.Contains(text, "self-test execution status") && strings.Contains(text, "% remaining") {
				continue
			}
			result.Status, result.Output = "ok", boundedDeepOutput(output)
			return result
		}
	}
}

func RunGPUCompute(ctx context.Context, device string) DeepToolResult {
	return runGPUCompute(ctx, strings.TrimSpace(device), runDeepCommand)
}

func runGPUCompute(ctx context.Context, device string, runner deepCommandRunner) (result DeepToolResult) {
	result = DeepToolResult{SchemaVersion: "goecs.gpu/compute-v1", Status: "skipped", Target: device}
	if device == "" {
		result.Error = "explicit GPU device is not configured"
		return result
	}
	started := time.Now()
	defer func() { result.DurationMS = time.Since(started).Milliseconds() }()
	output, err := runner(ctx, "clpeak", "--device", device)
	if err != nil {
		return failedDeepTool(result, ctx, output, err)
	}
	result.Status, result.Output = "ok", boundedDeepOutput(output)
	return result
}

func runDeepCommand(ctx context.Context, name string, args ...string) ([]byte, error) {
	if _, err := exec.LookPath(name); err != nil {
		return nil, fmt.Errorf("%s is unavailable: %w", name, err)
	}
	return exec.CommandContext(ctx, name, args...).CombinedOutput()
}

func failedDeepTool(result DeepToolResult, ctx context.Context, output []byte, err error) DeepToolResult {
	result.Output = boundedDeepOutput(output)
	if contextErr := ctx.Err(); contextErr != nil {
		result.Status, result.Error = "canceled", contextErr.Error()
	} else if errors.Is(err, exec.ErrNotFound) || strings.Contains(err.Error(), "is unavailable") {
		result.Status, result.Error = "unavailable", err.Error()
	} else {
		result.Status, result.Error = "error", err.Error()
	}
	return result
}

func boundedDeepOutput(output []byte) string {
	const limit = 16 << 10
	if len(output) > limit {
		output = output[:limit]
	}
	return strings.TrimSpace(string(output))
}
