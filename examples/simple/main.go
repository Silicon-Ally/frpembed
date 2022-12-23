// This example runs a simple 'hello world' file server, and exposes it via
// FRP. See the README.md for usage instructions.
package main

import (
	"context"
	"log"
	"net/http"
	"os"

	"github.com/Silicon-Ally/frpembed"
)

func main() {
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "./examples/simple/index.html")
	})
	ctx := context.Background()

	serverAddr, token, targetDomain := os.Args[1], os.Args[2], os.Args[3]

	go func() {
		proxyOpt := frpembed.WithProxies(frpembed.ProxyConfig{
			Name:           "web-server",
			TargetDomain:   targetDomain,
			UseEncryption:  true,
			UseCompression: true,
			LocalPort:      8080,
		})
		if err := frpembed.Run(ctx, serverAddr, token, proxyOpt); err != nil {
			log.Fatalf("error while running frp client: %v", err)
		}
	}()

	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.Fatalf("error while running frp client: %v", err)
	}
}