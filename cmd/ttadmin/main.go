package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/pnsrc/ttgo/internal/admin"
)

func main() {
	if len(os.Args) >= 2 && os.Args[1] == "tunnel" {
		runTunnel(os.Args[2:])
		return
	}

	vpnPath := flag.String("vpn", "vpn.toml", "path to vpn.toml")
	hostsPath := flag.String("hosts", "hosts.toml", "path to hosts.toml")
	credsPath := flag.String("creds", "credentials.toml", "path to credentials.toml (file store)")
	flag.Parse()

	mode := "manage"
	if flag.NArg() > 0 {
		mode = flag.Arg(0)
	}

	paths := admin.Paths{
		VPN:   *vpnPath,
		Hosts: *hostsPath,
		Creds: *credsPath,
	}

	var err error
	switch mode {
	case "setup":
		err = admin.RunWizard(paths)
	case "migrate":
		err = admin.RunMigrate(paths)
	case "manage", "":
		err = admin.RunManage(paths)
	default:
		fmt.Fprintf(os.Stderr, "unknown command %q\nUsage: ttadmin [setup|manage|migrate|tunnel]\n", mode)
		os.Exit(1)
	}

	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func runTunnel(args []string) {
	if len(args) == 0 {
		tunnelUsage()
		os.Exit(1)
	}
	sub := args[0]

	fs := flag.NewFlagSet("tunnel "+sub, flag.ExitOnError)
	cfg := admin.TunnelConfig{}
	fs.StringVar(&cfg.Server, "server", "", "endpoint host:port (required)")
	fs.StringVar(&cfg.Hostname, "hostname", "", "SNI/Host header (default: same as server hostname)")
	fs.StringVar(&cfg.Username, "user", "", "TrustTunnel username (required)")
	fs.StringVar(&cfg.Password, "pass", "", "TrustTunnel password (required)")
	fs.StringVar(&cfg.Room, "room", "", "P2P room name shared by both ends (required)")
	fs.BoolVar(&cfg.Insecure, "insecure", false, "skip TLS certificate verification")

	switch sub {
	case "expose":
		fs.StringVar(&cfg.Target, "target", "", "local TCP host:port to forward to (required)")
		fs.IntVar(&cfg.Pool, "pool", 4, "number of pending listen sessions to keep")
	case "reach":
		fs.StringVar(&cfg.Local, "local", "", "local host:port to listen on (required)")
	default:
		tunnelUsage()
		os.Exit(1)
	}

	if err := fs.Parse(args[1:]); err != nil {
		os.Exit(1)
	}
	if cfg.Server == "" || cfg.Username == "" || cfg.Password == "" || cfg.Room == "" {
		fs.Usage()
		os.Exit(1)
	}

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	logfn := func(s string) { log.Println(s) }

	var err error
	switch sub {
	case "expose":
		err = admin.RunExpose(ctx, cfg, logfn)
	case "reach":
		err = admin.RunReach(ctx, cfg, logfn)
	}
	if err != nil && err != context.Canceled {
		fmt.Fprintln(os.Stderr, "tunnel error:", err)
		os.Exit(1)
	}
}

func tunnelUsage() {
	fmt.Fprint(os.Stderr, `Usage: ttadmin tunnel <subcommand> [flags]

Subcommands:
  expose    Make a local service reachable through the server
  reach     Connect to a service exposed by another device

Common flags:
  --server HOST:PORT     TrustTunnel endpoint
  --hostname NAME        SNI / Host header (default: extracted from server)
  --user NAME            TrustTunnel username
  --pass PASSWORD        TrustTunnel password
  --room NAME            shared room name (must match on both sides)
  --insecure             skip TLS verification (self-signed cert)

expose-only:
  --target HOST:PORT     local service to expose (e.g. 127.0.0.1:22)
  --pool N               pending listen sessions (default: 4)

reach-only:
  --local HOST:PORT      local listen address (e.g. 127.0.0.1:2222)

Example: SSH to home PC from anywhere
  # On home PC:
  ttadmin tunnel expose --server my.server:443 --user pnsrc --pass secret \
                        --room ssh --target 127.0.0.1:22

  # On laptop:
  ttadmin tunnel reach  --server my.server:443 --user pnsrc --pass secret \
                        --room ssh --local 127.0.0.1:2222

  ssh -p 2222 localhost   # → reaches home PC via the server
`)
}
