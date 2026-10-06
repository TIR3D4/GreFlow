// Package system executes bounded argv commands without invoking a shell.
package system

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

type Runner interface {
	Run(string, ...string) (string, error)
}
type Exec struct{}

func (Exec) Run(name string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	b, e := exec.CommandContext(ctx, name, args...).CombinedOutput()
	if e != nil {
		return string(b), fmt.Errorf("%s %s: %w: %s", name, strings.Join(args, " "), e, strings.TrimSpace(string(b)))
	}
	return string(b), nil
}
