// Command vcsim runs an in-process vCenter simulator (govmomi's simulator
// package) for local development and testing - no real VMware needed. It prints
// the SDK URL (with embedded dev credentials) and serves until interrupted.
//
//	go run ./cmd/vcsim
//	opord vcenter check --url <printed-url> --insecure
package main

import (
	"crypto/tls"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/vmware/govmomi/simulator"
)

func main() {
	model := simulator.VPX()
	defer model.Remove()

	if err := model.Create(); err != nil {
		log.Fatalf("vcsim: creating model: %v", err)
	}

	// Serve over HTTPS (self-signed) so both govmomi (insecure) and the OpenTofu
	// vSphere provider (which requires https) can connect.
	model.Service.TLS = new(tls.Config)
	server := model.Service.NewServer()
	defer server.Close()

	fmt.Println("vcsim ready (simulated vCenter, VPX model: DC0, LocalDS_0, sample VMs)")
	fmt.Printf("URL: %s\n", server.URL.String())
	fmt.Printf("check: opord vcenter check --url %s --insecure\n", server.URL.String())

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop
	fmt.Println("\nvcsim shutting down")
}
