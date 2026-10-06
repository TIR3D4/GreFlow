// GreFlow manages independent named GRE links. Shells are never used for commands.
package main

import (
	"bufio"
	"flag"
	"fmt"
	"github.com/TIR3D4/GreFlow/core/config"
	"github.com/TIR3D4/GreFlow/core/health"
	"github.com/TIR3D4/GreFlow/core/manager"
	"github.com/TIR3D4/GreFlow/core/system"
	"os"
	"strconv"
	"strings"
)

var version = "0.2.0"

var input = bufio.NewReader(os.Stdin)

func ask(label, def string) (string, error) {
	fmt.Printf("%s [%s]: ", label, def)
	s, e := input.ReadString('\n')
	if e != nil {
		return "", e
	}
	s = strings.TrimSpace(s)
	if s == "" {
		s = def
	}
	return s, nil
}
func main() {
	if e := run(os.Args[1:]); e != nil {
		fmt.Fprintln(os.Stderr, "GreFlow:", e)
		os.Exit(1)
	}
}
func run(args []string) error {
	name, args, e := selection(args)
	if e != nil {
		return e
	}
	if len(args) == 0 {
		return menu(name)
	}
	if args[0] == "version" || args[0] == "--version" {
		fmt.Println("GreFlow v" + version)
		return nil
	}
	if args[0] == "help" || args[0] == "--help" {
		fmt.Println(`GreFlow v0.2.0 — GRE + TCP/UDP forwarding
[--tunnel NAME] COMMAND (omit NAME for the existing default tunnel)
tunnels
setup [--role entry|exit --local IPv4 --remote IPv4 --interface greflow0 --network 10.77.0.0/30 --mtu 1476 --no-persist]
add tcp|udp PUBLIC_PORT_OR_RANGE [DESTINATION_PORT_OR_RANGE]
remove tcp|udp PUBLIC_PORT_OR_RANGE
list | apply | down | repair | restart | status | test | doctor | stats
verify yes|no | tune | logs | rollback BACKUP_DIRECTORY | uninstall [--yes]
GRE is unencrypted. Ports on exit must be allowed with add as well.`)
		return nil
	}
	if os.Geteuid() != 0 {
		return fmt.Errorf("run with sudo/root")
	}
	m := manager.Manager{Root: os.Getenv("GREFLOW_ROOT"), Name: name, R: system.Exec{}}
	// systemctl restart/stop invoke this binary: do not hold our lock across them.
	if args[0] == "restart" {
		if len(args) != 1 {
			return fmt.Errorf("restart takes no arguments")
		}
		_, e := m.R.Run("systemctl", "restart", m.Service())
		return e
	}
	if args[0] == "uninstall" && m.Root == "" {
		if len(args) != 2 || args[1] != "--yes" {
			s, e := ask("Remove tunnel "+m.Label()+"? Type yes", "no")
			if e != nil {
				return e
			}
			if s != "yes" {
				return nil
			}
		}
		if _, e := os.Stat("/etc/systemd/system/" + m.Service()); e == nil {
			if _, e = m.R.Run("systemctl", "stop", m.Service()); e != nil {
				return e
			}
		}
	}
	unlock, e := m.Lock()
	if e != nil {
		return e
	}
	defer unlock()
	switch args[0] {
	case "tunnels":
		instances, e := m.Instances()
		if e != nil {
			return e
		}
		for _, instance := range instances {
			c, e := config.Load(instance.ConfigPath())
			if e != nil {
				return e
			}
			state, e := instance.State()
			if e != nil {
				return e
			}
			fmt.Printf("%s role=%s remote=%s interface=%s network=%s active=%t ports=%d\n", instance.Label(), c.Role, c.Remote, c.Interface, c.Network, state.Active, len(c.Forwards))
		}
		return nil
	case "setup":
		return setup(m, args[1:])
	case "uninstall":
		return m.Uninstall()
	case "down":
		return m.Down()
	case "rollback":
		if len(args) != 2 {
			return fmt.Errorf("rollback BACKUP_DIRECTORY")
		}
		return m.Rollback(args[1])
	case "tune":
		return health.Tune(m)
	case "logs":
		b, e := os.ReadFile(m.Path("/var/log/greflow/events.jsonl"))
		if e == nil {
			fmt.Print(string(b))
		}
		return e
	}
	c, e := config.Load(m.ConfigPath())
	if e != nil {
		return fmt.Errorf("configuration: %w (run greflow setup)", e)
	}
	switch args[0] {
	case "apply":
		return m.Apply(c)
	case "repair":
		if e = m.Apply(c); e != nil {
			return e
		}
		if m.Root == "" {
			return m.Persist()
		}
		return nil
	case "add":
		if len(args) < 3 || len(args) > 4 {
			return fmt.Errorf("add tcp|udp PUBLIC [DESTINATION]")
		}
		dst := args[2]
		if len(args) == 4 {
			dst = args[3]
		}
		f := config.Forward{Protocol: strings.ToLower(args[1]), Public: args[2], Destination: dst}
		for _, g := range c.Forwards {
			if f == g {
				fmt.Println("Forward already configured")
				return nil
			}
		}
		c.Forwards = append(c.Forwards, f)
		return m.Apply(c)
	case "remove":
		if len(args) != 3 {
			return fmt.Errorf("remove tcp|udp PUBLIC")
		}
		found := false
		var fs []config.Forward
		for _, f := range c.Forwards {
			if f.Protocol == args[1] && f.Public == args[2] {
				found = true
			} else {
				fs = append(fs, f)
			}
		}
		if !found {
			return fmt.Errorf("forward not found")
		}
		c.Forwards = fs
		return m.Apply(c)
	case "list":
		for _, f := range c.Forwards {
			fmt.Printf("%s %s -> %s\n", strings.ToUpper(f.Protocol), f.Public, f.Destination)
		}
		return nil
	case "status":
		return health.Status(m, c, false)
	case "test":
		return health.Status(m, c, true)
	case "doctor":
		return health.Doctor(m, c)
	case "stats":
		return health.Stats(m, c)
	case "verify":
		if len(args) != 2 || (args[1] != "yes" && args[1] != "no") {
			return fmt.Errorf("verify yes|no")
		}
		s, e := m.State()
		if e != nil {
			return e
		}
		if !s.Active {
			return fmt.Errorf("tunnel inactive")
		}
		s.Verified = args[1] == "yes"
		return config.Save(m.StatePath(), s)
	default:
		return fmt.Errorf("unknown command %q; use greflow help", args[0])
	}
}
func setup(m manager.Manager, args []string) error {
	c := config.Config{Interface: m.DefaultInterface(), Network: "10.77.0.0/30", MTU: 1476}
	previous, previousErr := config.Load(m.ConfigPath())
	if previousErr == nil {
		c.Interface = previous.Interface
		c.Network = previous.Network
		c.MTU = previous.MTU
	}
	fs := flag.NewFlagSet("setup", flag.ContinueOnError)
	fs.StringVar(&c.Role, "role", "", "entry or exit")
	fs.StringVar(&c.Local, "local", "", "local IPv4")
	fs.StringVar(&c.Remote, "remote", "", "remote IPv4")
	fs.StringVar(&c.Interface, "interface", c.Interface, "GRE interface")
	fs.StringVar(&c.Network, "network", c.Network, "private /30")
	fs.IntVar(&c.MTU, "mtu", c.MTU, "GRE MTU")
	noPersist := fs.Bool("no-persist", false, "skip systemd (test use)")
	if e := fs.Parse(args); e != nil {
		return e
	}
	if fs.NArg() != 0 {
		return fmt.Errorf("unexpected setup arguments")
	}
	networkSet := false
	fs.Visit(func(f *flag.Flag) {
		if f.Name == "network" {
			networkSet = true
		}
	})
	if m.Name != "" && len(args) > 0 && previousErr != nil && !networkSet {
		return fmt.Errorf("new named tunnel requires --network with a unique private /30; use the same subnet on its peer")
	}
	if previousErr != nil && !os.IsNotExist(previousErr) {
		return previousErr
	}
	if len(args) == 0 {
		var e error
		c.Role, e = ask("Role (entry/exit)", "entry")
		if e != nil {
			return e
		}
		c.Local, e = ask("Local public IPv4", "")
		if e != nil {
			return e
		}
		c.Remote, e = ask("Remote public IPv4", "")
		if e != nil {
			return e
		}
		c.Interface, e = ask("GRE interface", c.Interface)
		if e != nil {
			return e
		}
		c.Network, e = ask("Private GRE network", c.Network)
		if e != nil {
			return e
		}
		v, e := ask("MTU", "1476")
		if e != nil {
			return e
		}
		c.MTU, e = strconv.Atoi(v)
		if e != nil {
			return e
		}
	}
	if old, e := config.Load(m.ConfigPath()); e == nil {
		c.Forwards = old.Forwards
	}
	if e := m.Apply(c); e != nil {
		return e
	}
	if !*noPersist {
		if e := m.Persist(); e != nil {
			return fmt.Errorf("tunnel applied, persistence failed: %w; run greflow repair", e)
		}
	}
	fmt.Println("GRE configured. Configure the other server, then add ports on both sides and run greflow test.")
	return nil
}
func selection(args []string) (string, []string, error) {
	var name string
	var rest []string
	seen := false
	for i := 0; i < len(args); i++ {
		a := args[i]
		if a == "--tunnel" || strings.HasPrefix(a, "--tunnel=") {
			if seen {
				return "", nil, fmt.Errorf("--tunnel may appear once")
			}
			seen = true
			if a == "--tunnel" {
				i++
				if i == len(args) {
					return "", nil, fmt.Errorf("--tunnel requires a name")
				}
				name = args[i]
			} else {
				name = strings.TrimPrefix(a, "--tunnel=")
			}
			if name == "" {
				return "", nil, fmt.Errorf("empty tunnel name")
			}
			if name == "default" {
				name = ""
			} else if e := config.ValidName(name); e != nil {
				return "", nil, e
			}
		} else {
			rest = append(rest, a)
		}
	}
	return name, rest, nil
}
func menu(name string) error {
	for {
		fmt.Printf("\nSelected tunnel: %s\n", manager.Manager{Name: name}.Label())
		fmt.Print(`
GreFlow v0.2.0
1. Install / Setup (selected tunnel)
11. List Tunnels
12. Select / Create Named Tunnel
2. Add Port Forward
3. Remove Port Forward
4. List Port Forwards
5. Tunnel Status
6. Test Tunnel
7. Traffic Stats
8. Repair
9. Restart Tunnel
10. Uninstall
0. Exit
`)
		s, e := ask("Choice", "0")
		if e != nil {
			return e
		}
		if s == "0" {
			return nil
		}
		if s == "12" {
			v, e := ask("Tunnel name (default for existing tunnel)", "default")
			if e != nil {
				return e
			}
			if v == "default" {
				name = ""
			} else if e = config.ValidName(v); e != nil {
				fmt.Println(e)
			} else {
				name = v
			}
			continue
		}
		fmt.Printf("Selected tunnel: %s\n", manager.Manager{Name: name}.Label())
		cmd := map[string]string{"1": "setup", "2": "add", "3": "remove", "4": "list", "5": "status", "6": "test", "7": "stats", "8": "repair", "9": "restart", "10": "uninstall", "11": "tunnels"}[s]
		if cmd == "" {
			fmt.Println("Invalid selection")
			continue
		}
		a := []string{cmd}
		if cmd == "add" || cmd == "remove" {
			p, e := ask("Protocol tcp/udp", "tcp")
			if e != nil {
				return e
			}
			port, e := ask("Public port/range", "")
			if e != nil {
				return e
			}
			a = append(a, p, port)
			if cmd == "add" {
				dst, e := ask("Destination port/range", port)
				if e != nil {
					return e
				}
				a = append(a, dst)
			}
		}
		if name != "" {
			a = append([]string{"--tunnel", name}, a...)
		}
		if e = run(a); e != nil {
			fmt.Fprintln(os.Stderr, e)
		}
	}
}
