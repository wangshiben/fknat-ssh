package service

import "testing"

func TestSystemdArgQuotesPathsAndEscapesSpecifiers(t *testing.T) {
	got, err := systemdArg(`/opt/FKNAT SSH/fknat-ssh%prod`)
	if err != nil {
		t.Fatal(err)
	}
	if got != `"/opt/FKNAT SSH/fknat-ssh%%prod"` {
		t.Fatalf("argument=%q", got)
	}
}

func TestSystemdArgRejectsNewline(t *testing.T) {
	if _, err := systemdArg("/tmp/fknat-ssh\nExecStart=/bin/false"); err == nil {
		t.Fatal("newline was accepted")
	}
}
