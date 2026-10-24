package health

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"net"
	"strings"
	"time"
)

func CheckSSHBanner(ctx context.Context, target string) (string, error) {
	connection, err := (&net.Dialer{Timeout: 3 * time.Second}).DialContext(ctx, "tcp", target)
	if err != nil {
		return "", fmt.Errorf("ssh backend unreachable: %w", err)
	}
	defer connection.Close()
	_ = connection.SetReadDeadline(time.Now().Add(3 * time.Second))
	banner, err := bufio.NewReader(connection).ReadString('\n')
	if err != nil {
		return "", fmt.Errorf("ssh banner read failed: %w", err)
	}
	banner = strings.TrimSpace(banner)
	if !strings.HasPrefix(banner, "SSH-") {
		return banner, errors.New("ssh backend returned an invalid banner")
	}
	return banner, nil
}
