package systemd

import (
	"context"
	"fmt"
	"os/exec"

	"github.com/pkg/errors"
)

func Enable(ctx context.Context, id string) error {
	return Command(ctx, "enable", serviceName(id))
}

func Start(ctx context.Context, id string) error {
	return Command(ctx, "start", serviceName(id))
}

func Stop(ctx context.Context, id string) error {
	return Command(ctx, "stop", serviceName(id))
}

func Disable(ctx context.Context, id string) error {
	return Command(ctx, "disable", serviceName(id))
}

// Command runs a systemd command
func Command(ctx context.Context, args ...string) error {
	out, err := exec.CommandContext(ctx, "systemctl", args...).CombinedOutput()
	if err != nil {
		return errors.Wrap(err, string(out))
	}
	return nil
}

func serviceName(id string) string {
	return fmt.Sprintf("boss-v2@%s.service", id)
}
