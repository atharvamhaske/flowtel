package agentprocess

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
)

func PiBinary() (string, error) {
	if bin := os.Getenv("FLOWTEL_PI"); bin != "" {
		return bin, nil
	}
	return exec.LookPath("pi")
}

func (w *World) RunPi(ctx context.Context, prompt string) ([]byte, error) {
	if w == nil || w.Inference == nil {
		return nil, fmt.Errorf("run pi: agent world is not ready")
	}
	bin, err := PiBinary()
	if err != nil {
		return nil, fmt.Errorf("run pi: %w", err)
	}
	agentDir := filepath.Join(w.dataDir, "pi-agent")
	if err := os.MkdirAll(agentDir, 0o700); err != nil {
		return nil, fmt.Errorf("create pi agent dir: %w", err)
	}
	models := []byte(`{"providers":{"flowtel":{"baseUrl":"` + w.Inference.URL() + `/v1","api":"openai-completions","apiKey":"test","compat":{"supportsDeveloperRole":false,"supportsReasoningEffort":false},"models":[{"id":"mock","contextWindow":8192,"maxTokens":256}]}}}`)
	if err := os.WriteFile(filepath.Join(agentDir, "models.json"), models, 0o600); err != nil {
		return nil, fmt.Errorf("write pi models.json: %w", err)
	}
	cmd := exec.CommandContext(ctx, bin,
		"--mode", "json",
		"--no-tools",
		"--no-extensions",
		"--no-skills",
		"--no-context-files",
		"--no-session",
		"--provider", "flowtel",
		"--model", "mock",
		"--api-key", "test",
		"--system-prompt", "Reply with exactly: hi",
		prompt,
	)
	cmd.Dir = w.dataDir
	cmd.Env = append(os.Environ(),
		"HOME="+w.dataDir,
		"PI_CODING_AGENT_DIR="+agentDir,
		"PI_CODING_AGENT_SESSION_DIR="+filepath.Join(w.dataDir, "pi-sessions"),
		"PI_SKIP_VERSION_CHECK=1",
	)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("run pi: %w", err)
	}
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start pi: %w", err)
	}
	output, readErr := io.ReadAll(stdout)
	waitErr := cmd.Wait()
	if readErr != nil {
		return nil, fmt.Errorf("read pi stdout: %w", readErr)
	}
	if waitErr != nil {
		return nil, fmt.Errorf("run pi: %w: %s", waitErr, stderr.String())
	}
	return output, nil
}
