package kube

import (
	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	kubeflag "k8s.io/component-base/cli/flag"
	kubeutil "k8s.io/kubectl/pkg/cmd/util"
)

// see https://github.com/kubernetes/kubectl/blob/a66725a1e1f21f6d34f0bd5ad187b0814d452ecc/pkg/cmd/cmd.go#L221
func GetFactory() kubeutil.Factory {
	warningsAsErrors := false
	// Parent command to which all subcommands are added.
	cmds := &cobra.Command{}
	flags := cmds.PersistentFlags()
	flags.SetNormalizeFunc(kubeflag.WarnWordSepNormalizeFunc) // Warn for "_" flags

	// Normalize all flags that are coming from other packages or pre-configurations
	// a.k.a. change all "_" to "-". e.g. glog package
	flags.SetNormalizeFunc(kubeflag.WordSepNormalizeFunc)

	flags.BoolVar(&warningsAsErrors, "warnings-as-errors", warningsAsErrors, "Treat warnings received from the server as errors and exit with a non-zero exit code")

	kubeConfigFlags := genericclioptions.NewConfigFlags(true).WithDeprecatedPasswordFlag()
	kubeConfigFlags.AddFlags(flags)
	matchVersionKubeConfigFlags := kubeutil.NewMatchVersionFlags(kubeConfigFlags)
	matchVersionKubeConfigFlags.AddFlags(cmds.PersistentFlags())
	// Updates hooks to add kubectl command headers: SIG CLI KEP 859.

	return kubeutil.NewFactory(matchVersionKubeConfigFlags)
}
