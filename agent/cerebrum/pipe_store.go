package cerebrum

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"gopkg.in/yaml.v3"
)

const defaultReflexFile = "pipes.yaml"

// PipeStore manages reflex pipes — YAML persistence + embedding search.
// Absorbed from github.com/dpopsuev/tubus/registry.
type PipeStore struct {
	path   string
	config PipeConfig
	mu     sync.RWMutex
}

var _ ReflexStore = (*PipeStore)(nil)

func LoadPipeStore(path string) (*PipeStore, error) {
	if path == "" {
		path = defaultReflexPath()
	}
	s := &PipeStore{path: path}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			s.config = PipeConfig{Pipes: make(map[string]Pipe)}
			return s, nil
		}
		return nil, fmt.Errorf("read %s: %w", path, err)
	}

	if err := yaml.Unmarshal(data, &s.config); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	if s.config.Pipes == nil {
		s.config.Pipes = make(map[string]Pipe)
	}
	return s, nil
}

func NewPipeStore() *PipeStore {
	return &PipeStore{
		config: PipeConfig{Pipes: make(map[string]Pipe)},
	}
}

func (s *PipeStore) Match(embedding []float64) (*Pipe, float64) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if len(embedding) == 0 || len(s.config.Pipes) == 0 {
		return nil, 0
	}

	var best *Pipe
	var bestSim float64

	for name := range s.config.Pipes {
		p := s.config.Pipes[name]
		if len(p.Embedding) == 0 {
			continue
		}
		sim := CosineSimilarity(embedding, p.Embedding)
		if sim > bestSim {
			bestSim = sim
			best = &p
		}
	}

	return best, bestSim
}

func (s *PipeStore) Add(pipe Pipe) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.config.Pipes[pipe.Name]; exists {
		return fmt.Errorf("pipe %q already exists", pipe.Name)
	}
	s.config.Pipes[pipe.Name] = pipe
	return nil
}

func (s *PipeStore) Merge(embedding []float64, steps []PipeStep) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	for name := range s.config.Pipes {
		p := s.config.Pipes[name]
		if len(p.Embedding) == 0 {
			continue
		}
		sim := CosineSimilarity(embedding, p.Embedding)
		if sim > 0.9 {
			p.Steps = append(p.Steps, steps...)
			s.config.Pipes[name] = p
			return true
		}
	}
	return false
}

func (s *PipeStore) Prune(minScore float64) int {
	s.mu.Lock()
	defer s.mu.Unlock()

	pruned := 0
	for name := range s.config.Pipes {
		p := s.config.Pipes[name]
		if p.Score() < minScore {
			delete(s.config.Pipes, name)
			pruned++
		}
	}
	return pruned
}

func (s *PipeStore) Save() error {
	if s.path == "" {
		return nil
	}
	s.mu.RLock()
	defer s.mu.RUnlock()

	data, err := yaml.Marshal(&s.config)
	if err != nil {
		return err
	}
	dir := filepath.Dir(s.path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	return os.WriteFile(s.path, data, 0644)
}

func (s *PipeStore) Len() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.config.Pipes)
}

func (s *PipeStore) Get(name string) (Pipe, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	p, ok := s.config.Pipes[name]
	return p, ok
}

func defaultReflexPath() string {
	if _, err := os.Stat(".tako/reflexes"); err == nil {
		return filepath.Join(".tako", "reflexes", defaultReflexFile)
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".tako", "reflexes", defaultReflexFile)
}
