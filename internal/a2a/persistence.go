// Package a2a provides Agent-to-Agent protocol support.
package a2a

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
)

// A2ATaskPersistence defines the interface for task persistence.
type A2ATaskPersistence interface {
	SaveTask(task *A2aTask) error
	LoadTask(id string) (*A2aTask, error)
	LoadAllTasks() ([]*A2aTask, error)
	DeleteTask(id string) error
}

// FilePersistence implements file-based task persistence.
type FilePersistence struct {
	mu      sync.RWMutex
	dataDir string
}

// NewFilePersistence creates a new file-based persistence.
func NewFilePersistence(dataDir string) (*FilePersistence, error) {
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return nil, err
	}
	return &FilePersistence{
		dataDir: dataDir,
	}, nil
}

// taskFilePath returns the file path for a task.
func (fp *FilePersistence) taskFilePath(id string) string {
	return filepath.Join(fp.dataDir, id+".json")
}

// SaveTask saves a task to disk.
func (fp *FilePersistence) SaveTask(task *A2aTask) error {
	fp.mu.Lock()
	defer fp.mu.Unlock()

	data, err := json.MarshalIndent(task, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(fp.taskFilePath(task.ID), data, 0644)
}

// LoadTask loads a task from disk.
func (fp *FilePersistence) LoadTask(id string) (*A2aTask, error) {
	fp.mu.RLock()
	defer fp.mu.RUnlock()

	data, err := os.ReadFile(fp.taskFilePath(id))
	if err != nil {
		return nil, err
	}

	var task A2aTask
	if err := json.Unmarshal(data, &task); err != nil {
		return nil, err
	}

	return &task, nil
}

// LoadAllTasks loads all tasks from disk.
func (fp *FilePersistence) LoadAllTasks() ([]*A2aTask, error) {
	fp.mu.RLock()
	defer fp.mu.RUnlock()

	entries, err := os.ReadDir(fp.dataDir)
	if err != nil {
		return nil, err
	}

	var tasks []*A2aTask
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if filepath.Ext(entry.Name()) != ".json" {
			continue
		}

		id := entry.Name()[:len(entry.Name())-5]
		task, err := fp.LoadTask(id)
		if err != nil {
			continue
		}
		tasks = append(tasks, task)
	}

	return tasks, nil
}

// DeleteTask deletes a task from disk.
func (fp *FilePersistence) DeleteTask(id string) error {
	fp.mu.Lock()
	defer fp.mu.Unlock()

	return os.Remove(fp.taskFilePath(id))
}

// PersistentA2ATaskStore is a task store with persistence support.
type PersistentA2ATaskStore struct {
	*A2ATaskStore
	persistence A2ATaskPersistence
}

// NewPersistentA2ATaskStore creates a new persistent task store.
func NewPersistentA2ATaskStore(maxTasks int, persistence A2ATaskPersistence) (*PersistentA2ATaskStore, error) {
	store := &PersistentA2ATaskStore{
		A2ATaskStore: NewA2ATaskStore(maxTasks),
		persistence:  persistence,
	}

	if persistence != nil {
		tasks, err := persistence.LoadAllTasks()
		if err == nil {
			store.mu.Lock()
			for _, task := range tasks {
				if len(store.tasks) < store.maxTasks {
					store.tasks[task.ID] = task
				}
			}
			store.mu.Unlock()
		}
	}

	return store, nil
}

// CreateTask creates a new task and saves it.
func (s *PersistentA2ATaskStore) CreateTask(agentID string, agentName string, messages []A2aMessage, sessionID *string) *A2aTask {
	task := s.A2ATaskStore.CreateTask(agentID, agentName, messages, sessionID)
	if s.persistence != nil {
		s.persistence.SaveTask(task)
	}
	return task
}

// AddTask adds an existing task and saves it.
func (s *PersistentA2ATaskStore) AddTask(task *A2aTask) {
	s.A2ATaskStore.AddTask(task)
	if s.persistence != nil {
		s.persistence.SaveTask(task)
	}
}

// UpdateTaskStatus updates a task's status and saves it.
func (s *PersistentA2ATaskStore) UpdateTaskStatus(id string, status A2aTaskStatus) bool {
	ok := s.A2ATaskStore.UpdateTaskStatus(id, status)
	if ok && s.persistence != nil {
		if task, ok := s.GetTask(id); ok {
			s.persistence.SaveTask(task)
		}
	}
	return ok
}

// CompleteTask completes a task and saves it.
func (s *PersistentA2ATaskStore) CompleteTask(id string, response A2aMessage, artifacts []A2aArtifact) {
	s.A2ATaskStore.CompleteTask(id, response, artifacts)
	if s.persistence != nil {
		if task, ok := s.GetTask(id); ok {
			s.persistence.SaveTask(task)
		}
	}
}

// FailTask fails a task and saves it.
func (s *PersistentA2ATaskStore) FailTask(id string, errorMessage A2aMessage) {
	s.A2ATaskStore.FailTask(id, errorMessage)
	if s.persistence != nil {
		if task, ok := s.GetTask(id); ok {
			s.persistence.SaveTask(task)
		}
	}
}

// CancelTask cancels a task and saves it.
func (s *PersistentA2ATaskStore) CancelTask(id string) bool {
	ok := s.A2ATaskStore.CancelTask(id)
	if ok && s.persistence != nil {
		if task, ok := s.GetTask(id); ok {
			s.persistence.SaveTask(task)
		}
	}
	return ok
}
