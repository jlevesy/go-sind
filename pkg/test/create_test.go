package test

import (
	"context"
	"fmt"
	"testing"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
	"github.com/jlevesy/sind/pkg/sind"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSindCanCreateACluster(t *testing.T) {
	ctx := context.Background()
	params := sind.CreateClusterParams{
		ClusterName: "test_create",
		NetworkName: "test_create",
		Managers:    3,
		Workers:     4,
	}
	cluster, err := sind.CreateCluster(ctx, params)
	require.NoError(t, err)

	defer func() {
		require.NoError(t, cluster.Delete(ctx))
	}()

	swarmClient, err := cluster.Cluster.Client()
	require.NoError(t, err)

	info, err := swarmClient.Info(ctx)
	require.NoError(t, err)

	require.True(t, info.Swarm.ControlAvailable)

	assert.Equal(t, info.Swarm.Managers, params.Managers)
	assert.Equal(t, info.Swarm.Nodes-info.Swarm.Managers, params.Workers)
}

func TestSindCanCreateAClusterWithCustomImage(t *testing.T) {
	ctx := context.Background()
	params := sind.CreateClusterParams{
		ClusterName: "test_create_custom_image",
		NetworkName: "test_create_custom_image",
		Managers:    3,
		Workers:     4,
		ImageName:   "jlevesy/docker:dind",
		PullImage:   true,
	}
	cluster, err := sind.CreateCluster(ctx, params)
	require.NoError(t, err)

	defer func() {
		require.NoError(t, cluster.Delete(ctx))
	}()

	dockerCli, err := cluster.Host.Client()
	require.NoError(t, err)

	listFilters := filters.NewArgs(filters.Arg("ancestor", params.ImageName), filters.Arg("status", "running"))
	runningContainers, err := dockerCli.ContainerList(ctx, types.ContainerListOptions{Filters: listFilters})
	require.NoError(t, err)

	require.Len(t, runningContainers, params.Managers+params.Workers)
}

func TestSindCanCreateMultipleClusters(t *testing.T) {
	ctx := context.Background()
	for i := 0; i < 3; i++ {
		t.Log("Creating cluster n°", i)
		params := sind.CreateClusterParams{
			ClusterName: fmt.Sprintf("test_create_parallel_%d", i),
			NetworkName: fmt.Sprintf("test_create_parallel_%d", i),
			Managers:    1,
			Workers:     1,
		}
		cluster, err := sind.CreateCluster(ctx, params)
		require.NoError(t, err)

		defer func() {
			require.NoError(t, cluster.Delete(ctx))
		}()
	}
}

func TestSindCanCreateAClusterWithCustomSubnet(t *testing.T) {
	ctx := context.Background()

	params := sind.CreateClusterParams{
		ClusterName:   "test_create_custom_subnet",
		NetworkName:   "test_create_custom_subnet",
		Managers:      3,
		Workers:       4,
		NetworkSubnet: "10.7.0.0/24",
	}
	cluster, err := sind.CreateCluster(ctx, params)
	require.NoError(t, err)

	defer func() {
		require.NoError(t, cluster.Delete(ctx))
	}()

	dockerClient, err := cluster.Host.Client()
	require.NoError(t, err)

	networks, err := dockerClient.NetworkList(
		ctx,
		types.NetworkListOptions{
			Filters: filters.NewArgs(filters.Arg("name", params.NetworkName)),
		},
	)
	require.NoError(t, err)
	require.Len(t, networks, 1)

	net := networks[0]

	require.Len(t, net.IPAM.Config, 1)
	require.Equal(t, net.IPAM.Config[0].Subnet, params.NetworkSubnet)
}
