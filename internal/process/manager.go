package process

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"time"
)

const defaultRestartLimit = 3

type ProcessManager struct {
	reg *ProcessRegistry
}

func NewProcessManager() *ProcessManager {
	return &ProcessManager{reg: NewProcessRegistry()}
}

func (pm *ProcessManager) StartProcess(name string, cmd []string, env map[string]string, restartOnCrash bool) (string, error) {
	if len(cmd) == 0 {
		return "", fmt.Errorf("invalid input to StartProcess: empty cmd")
	}
	id := randomID()
	entry := &ProcessEntry{
		ID:             id,
		Name:           name,
		Cmd:            cmd,
		Env:            env,
		Status:         PSStopped,
		RestartOnCrash: restartOnCrash,
		RestartLimit:   defaultRestartLimit,
	}
	pm.reg.set(entry)
	pm.launch(entry)
	return id, nil
}

func (pm *ProcessManager) StopProcess(id string) error {
	p, ok := pm.reg.get(id)
	if !ok {
		return fmt.Errorf("process not found: %s", id)
	}
	if p.PID > 0 {
		if proc, err := os.FindProcess(p.PID); err == nil {
			_ = proc.Kill()
		}
	}
	p.Status = PSStopped
	p.EndTime = time.Now()
	pm.reg.set(p)
	return nil
}

func (pm *ProcessManager) Status(id string) (ProcessStatus, error) {
	p, ok := pm.reg.get(id)
	if !ok {
		return PSStopped, fmt.Errorf("process not found: %s", id)
	}
	return p.Status, nil
}

func (pm *ProcessManager) ListAll() []ProcessStatus {
	var out []ProcessStatus
	for _, p := range pm.reg.list() {
		out = append(out, p.Status)
	}
	return out
}

// launch starts and monitors a process with restart-on-crash policy
func (pm *ProcessManager) launch(e *ProcessEntry) {
	go func(proc *ProcessEntry) {
		for {
			// build cmd
			c := exec.Command(proc.Cmd[0], proc.Cmd[1:]...)
			if proc.Env != nil {
				env := os.Environ()
				for k, v := range proc.Env {
					env = append(env, k+"="+v)
				}
				c.Env = env
			}
			// start
			if err := c.Start(); err != nil {
				proc.Status = PSStopped
				proc.EndTime = time.Now()
				pm.reg.set(proc)
				return
			}
			proc.PID = c.Process.Pid
			proc.StartTime = time.Now()
			proc.Status = PSRunning
			pm.reg.set(proc)
			// wait
			err := c.Wait()
			proc.EndTime = time.Now()
			if err != nil {
				// crashed
				if proc.RestartOnCrash && proc.RestartCount < proc.RestartLimit {
					proc.RestartCount++
					time.Sleep(50 * time.Millisecond)
					pm.reg.set(proc)
					continue
				}
			}
			proc.Status = PSStopped
			pm.reg.set(proc)
			return
		}
	}(e)
}

func randomID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return hex.EncodeToString(b)
	}
	return hex.EncodeToString(b)
}
