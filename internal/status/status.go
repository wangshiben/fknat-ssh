package status

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

type Snapshot struct {
	State          string    `json:"state"`
	Address        string    `json:"address"`
	Backend        string    `json:"backend"`
	BackendStatus  string    `json:"backend_status"`
	NetworkStatus  string    `json:"network_status"`
	RelayCount     int       `json:"relay_count"`
	ActiveSessions int       `json:"active_sessions"`
	ActiveStreams  int64     `json:"active_streams"`
	AcceptedTotal  uint64    `json:"accepted_total"`
	RejectedTotal  uint64    `json:"rejected_total"`
	StartedAt      time.Time `json:"started_at,omitempty"`
	LastError      string    `json:"last_error,omitempty"`
	UpdatedAt      time.Time `json:"updated_at"`
}

func Write(path string, snapshot Snapshot) error {
	snapshot.UpdatedAt = time.Now().UTC()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(snapshot, "", "  ")
	if err != nil {
		return err
	}
	temporary, err := os.CreateTemp(filepath.Dir(path), ".status-*")
	if err != nil {
		return err
	}
	name := temporary.Name()
	defer os.Remove(name)
	if err := temporary.Chmod(0o600); err != nil {
		_ = temporary.Close()
		return err
	}
	if _, err := temporary.Write(append(data, '\n')); err != nil {
		_ = temporary.Close()
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	return os.Rename(name, path)
}

func Read(path string) (Snapshot, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Snapshot{}, err
	}
	var snapshot Snapshot
	err = json.Unmarshal(data, &snapshot)
	return snapshot, err
}
