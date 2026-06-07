package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/pnsrc/ttgo/internal/admin"
)

func main() {
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
		fmt.Fprintf(os.Stderr, "unknown command %q\nUsage: ttadmin [setup|manage|migrate]\n", mode)
		os.Exit(1)
	}

	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}
