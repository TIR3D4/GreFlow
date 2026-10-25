package main

import (
	"encoding/json"
	"fmt"
	"github.com/TIR3D4/GreFlow/core/manager"
	"github.com/TIR3D4/GreFlow/internal/providers/gre"
	"os"
)

func structured(m manager.Manager, action string) error {
	switch action {
	case "tunnels":
		v, e := gre.Discover(m)
		if e != nil {
			return e
		}
		return json.NewEncoder(os.Stdout).Encode(v)
	case "status", "test":
		v, e := gre.Observe(m, action == "test")
		if e != nil {
			return e
		}
		return json.NewEncoder(os.Stdout).Encode(v)
	default:
		return fmt.Errorf("--json supports tunnels, status and test")
	}
}
