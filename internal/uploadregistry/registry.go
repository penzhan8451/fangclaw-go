package uploadregistry

import (
	"path/filepath"
	"sync"
)

type UploadMeta struct {
	Filename    string
	ContentType string
	FilePath    string
}

var (
	registry = make(map[string]UploadMeta)
	mu       sync.RWMutex
)

func Register(fileID string, meta UploadMeta) {
	mu.Lock()
	defer mu.Unlock()
	registry[fileID] = meta
}

func Get(fileID string) (UploadMeta, bool) {
	mu.RLock()
	defer mu.RUnlock()
	meta, ok := registry[fileID]
	return meta, ok
}

func FindByFilename(filename string) (UploadMeta, bool) {
	mu.RLock()
	defer mu.RUnlock()
	for _, meta := range registry {
		if meta.Filename == filename {
			return meta, true
		}
	}
	return UploadMeta{}, false
}

func FindByBasename(filename string) (UploadMeta, bool) {
	basename := filepath.Base(filename)
	mu.RLock()
	defer mu.RUnlock()
	for _, meta := range registry {
		if filepath.Base(meta.Filename) == basename {
			return meta, true
		}
	}
	return UploadMeta{}, false
}
