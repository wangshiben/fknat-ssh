package status

import (
	"path/filepath"
	"testing"
)

func TestWriteRead(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "status.json")
	want := Snapshot{State: "online", Address: "abc", Backend: "127.0.0.1:22"}
	if err := Write(path, want); err != nil {
		t.Fatal(err)
	}
	got, err := Read(path)
	if err != nil {
		t.Fatal(err)
	}
	if got.State != want.State || got.Address != want.Address || got.UpdatedAt.IsZero() {
		t.Fatalf("snapshot=%+v", got)
	}
}
