package cli

import (
	"context"
	"fmt"
	"os"

	"github.com/jlevesy/sind/pkg/sind"
	"github.com/jlevesy/sind/pkg/store"
	"github.com/spf13/cobra"
)

var (
	managers      int
	workers       int
	networkName   string
	portsMapping  []string
	nodeImageName string

	createCmd = &cobra.Command{
		Use:   "create",
		Short: "Create a new swarm cluster.",
		Run:   runCreate,
	}
)

func init() {
	rootCmd.AddCommand(createCmd)

	createCmd.Flags().IntVarP(&managers, "managers", "m", 1, "Amount of managers in the created cluster.")
	createCmd.Flags().IntVarP(&workers, "workers", "w", 0, "Amount of workers in the created cluster.")
	createCmd.Flags().StringVarP(&networkName, "network_name", "n", "sind_default", "Name of the network to create.")
	createCmd.Flags().StringSliceVarP(&portsMapping, "ports", "p", []string{}, "Ingress network port binding.")
	createCmd.Flags().StringVarP(&nodeImageName, "image", "i", "docker:18.09-dind", "Name of the image to use for the nodes.")
}

func runCreate(cmd *cobra.Command, args []string) {
	fmt.Printf("Creating a new cluster %q with %d managers and %d, workers...\n", clusterName, managers, workers)

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	st, err := store.New()
	if err != nil {
		fmt.Printf("unable to create store: %v\n", err)
		os.Exit(1)
	}

	if err := st.Exists(clusterName); err != nil {
		fmt.Printf("invalid cluster name: %v\n", err)
		os.Exit(1)
	}

	clusterParams := sind.CreateClusterParams{
		Managers:     managers,
		Workers:      workers,
		NetworkName:  networkName,
		ClusterName:  clusterName,
		PortBindings: portsMapping,
		ImageName:    nodeImageName,
	}

	cluster, err := sind.CreateCluster(ctx, clusterParams)
	if err != nil {
		fmt.Printf("unable to setup a swarm cluster: %v\n", err)
		os.Exit(1)
	}

	if err = st.Save(*cluster); err != nil {
		fmt.Printf("unable to save cluster: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Cluster %s successfuly created !\n", clusterName)
}