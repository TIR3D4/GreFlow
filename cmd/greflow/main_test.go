package main

import (
	"testing"
)

func TestTunnelSelection(t *testing.T) {
	for _, args := range [][]string{{"--tunnel", "exit2", "add", "tcp", "3212", "443"}, {"add", "tcp", "3212", "443", "--tunnel=exit2"}} {
		name, rest, e := selection(args)
		if e != nil || name != "exit2" || len(rest) != 4 {
			t.Fatal(name, rest, e)
		}
	}
	for _, args := range [][]string{{"--tunnel"}, {"--tunnel", "../x", "status"}, {"--tunnel=", "status"}, {"--tunnel", "a", "--tunnel", "b"}} {
		if _, _, e := selection(args); e == nil {
			t.Fatal(args)
		}
	}
	name, _, e := selection([]string{"--tunnel", "default", "status"})
	if e != nil || name != "" {
		t.Fatal(name, e)
	}
}
