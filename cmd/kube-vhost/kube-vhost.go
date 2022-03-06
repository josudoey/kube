package main

import (
	"log"

	"github.com/josudoey/kube/cmd/kube-vhost/vhostserver"
	"github.com/josudoey/kube/cmd/kube-vhost/vhostshow"
	"github.com/spf13/cobra"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
)

func newCommand() *cobra.Command {
	root := &cobra.Command{
		Use: "vhost",
		CompletionOptions: cobra.CompletionOptions{
			// see https://github.com/spf13/cobra/blob/9054739e08187aab9294b7a773d54c92fabc23d3/completions.go#L599
			DisableDefaultCmd: true,
		},
	}
	root.AddCommand(vhostserver.NewCommand())
	root.AddCommand(vhostshow.NewCommand())
	return root
}

func main() {
	cmd := newCommand()
	if err := cmd.Execute(); err != nil {
		log.Fatal(err)
	}
}
