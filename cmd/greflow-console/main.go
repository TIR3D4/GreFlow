package main

import (
	"context"
	"flag"
	"fmt"
	"github.com/TIR3D4/GreFlow/internal/console"
	app "github.com/TIR3D4/GreFlow/web/app"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"time"
	_ "time/tzdata"
)

var version = "dev"

func main() {
	if e := run(); e != nil {
		log.Fatal(e)
	}
}
func run() error {
	showVersion := flag.Bool("version", false, "print console version")
	dir := flag.String("data", "data", "private data directory")
	listen := flag.String("listen", "127.0.0.1:8787", "listen address")
	origin := flag.String("origin", "", "exact public origin, HTTPS for non-loopback")
	key := flag.String("key-file", "", "master key outside data directory (Windows DPAPI wrapped)")
	agentListen := flag.String("agent-listen", "", "separate native TLS listener for outbound agents")
	agentURL := flag.String("agent-url", "", "public HTTPS URL of native agent listener")
	certFile := flag.String("tls-cert", "", "native TLS certificate")
	tlsKey := flag.String("tls-key", "", "native TLS private key")
	flag.Parse()
	if *showVersion {
		fmt.Println("GreFlow Console " + version)
		return nil
	}
	if *origin == "" {
		*origin = "http://" + *listen
	}
	host, _, e := net.SplitHostPort(*listen)
	if e != nil {
		return e
	}
	ip := net.ParseIP(host)
	if (ip == nil || !ip.IsLoopback()) && !strings.HasPrefix(*origin, "https://") {
		return fmt.Errorf("non-loopback deployment requires HTTPS public origin and reverse proxy")
	}
	abs, e := filepath.Abs(*dir)
	if e != nil {
		return e
	}
	if *key == "" {
		configDir, e := os.UserConfigDir()
		if e != nil {
			return e
		}
		*key = filepath.Join(configDir, "GreFlow", "console-master.key")
	}
	s, e := console.OpenStore(abs, *key)
	if e != nil {
		return e
	}
	defer s.DB.Close()
	a := console.New(s, *origin)
	a.BuildVersion = version
	a.AgentURL = *agentURL
	if e = a.Bootstrap(); e != nil {
		return e
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()
	if *agentListen != "" {
		if *agentURL == "" || *certFile == "" || *tlsKey == "" {
			return fmt.Errorf("agent listener requires agent-url, tls-cert and tls-key")
		}
		tlsConfig, e := a.AgentTLS()
		if e != nil {
			return e
		}
		agentServer := &http.Server{Addr: *agentListen, Handler: a.AgentHandler(), TLSConfig: tlsConfig, ReadHeaderTimeout: 10 * time.Second, ReadTimeout: 30 * time.Second, WriteTimeout: 30 * time.Second}
		go func() {
			if e := agentServer.ListenAndServeTLS(*certFile, *tlsKey); e != nil && e != http.ErrServerClosed {
				log.Printf("agent listener failed: %v", e)
				cancel()
			}
		}()
		go func() { <-ctx.Done(); agentServer.Close() }()
	}
	server := &http.Server{Addr: *listen, Handler: a.Handler(http.FileServer(http.FS(app.Files))), ReadHeaderTimeout: 10 * time.Second, ReadTimeout: 30 * time.Second, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 32768}
	go func() {
		if e := a.Run(ctx); e != nil {
			log.Printf("operation worker stopped: %v", e)
			cancel()
		}
	}()
	go func() {
		<-ctx.Done()
		shutdown, done := context.WithTimeout(context.Background(), 5*time.Second)
		defer done()
		server.Shutdown(shutdown)
	}()
	fmt.Printf("GreFlow Console: %s\nData: %s\nFirst run: choose your panel password in the browser.\n", *origin, abs)
	e = server.ListenAndServe()
	if e == http.ErrServerClosed {
		return nil
	}
	return e
}
