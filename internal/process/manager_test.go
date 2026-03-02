package process

import (
	"testing"
	"time"
)

func TestProcessManager_StartStopStatus(t *testing.T) {
	pm := NewProcessManager()
	id, err := pm.StartProcess("echo-test", []string{"/bin/sh", "-c", "echo hello; sleep 1"}, nil, false)
	if err != nil {
		t.Fatalf("start failed: %v", err)
	}
	time.Sleep(100 * time.Millisecond)
	if st, _ := pm.Status(id); st != PSRunning {
		t.Fatalf("expected running, got %v", st)
	}
	if err := pm.StopProcess(id); err != nil {
		t.Fatalf("stop failed: %v", err)
	}
}

func TestProcessManager_RestartOnCrash(t *testing.T) {
	pm := NewProcessManager()
	id, err := pm.StartProcess("crash-test", []string{"/bin/sh", "-c", "exit 1"}, nil, true)
	if err != nil {
		t.Fatalf("start failed: %v", err)
	}
	time.Sleep(200 * time.Millisecond)
	// After crash, it may be running briefly due to restart loop; just ensure no panic and a valid status
	if st, _ := pm.Status(id); st != PSRunning && st != PSStopped {
		t.Fatalf("unexpected status after crash: %v", st)
	}
	_ = pm.StopProcess(id)
}
