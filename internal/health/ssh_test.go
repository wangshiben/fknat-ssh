package health

import (
	"context"
	"fmt"
	"net"
	"testing"
)

func TestCheckSSHBanner(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	go func() {
		connection, acceptErr := listener.Accept()
		if acceptErr == nil {
			_, _ = fmt.Fprintln(connection, "SSH-2.0-test")
			_ = connection.Close()
		}
	}()
	banner, err := CheckSSHBanner(context.Background(), listener.Addr().String())
	if err != nil || banner != "SSH-2.0-test" {
		t.Fatalf("banner=%q err=%v", banner, err)
	}
}
