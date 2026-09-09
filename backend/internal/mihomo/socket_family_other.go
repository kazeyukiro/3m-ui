//go:build !linux

package mihomo

import (
	"context"
	"fmt"
)

func ipv6OnlySockets(context.Context, byte) (map[uint32]bool, error) {
	return nil, fmt.Errorf("socket diagnostics unavailable")
}
