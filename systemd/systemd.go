package systemd

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/pkg/errors"
)

// Install installs the needed systemd files to run containers
// as proxy to containerd
func Install() error {
	path := filepath.Join(Root, serviceName(""))
	// don't re-install it if it already exists
	data, err := ioutil.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return writeService(path)
		}
		return err
	}
	// check the hashes of the files just to be safe
	current := getHash([]byte(service))
	if getHash(data) != current {
		return writeService(path)
	}
	return nil
}

// Remove the boss unit file
func Remove() error {
	path := filepath.Join(Root, serviceName(""))
	return os.Remove(path)
}

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
	return fmt.Sprintf("boss-v%d@%s.service", Version, id)
}
