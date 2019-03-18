package sind

import (
	"archive/tar"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/golang/sync/errgroup"

	"github.com/docker/docker/api/types"
	docker "github.com/docker/docker/client"
)

// Errors.
const (
	ErrImageReferenceNotFound = "image reference not found"
)

// PushImage pushes an image from the host to the cluster.
func (c *Cluster) PushImage(ctx context.Context, refs []string) error {
	hostClient, err := c.Host.Client()
	if err != nil {
		return fmt.Errorf("unable to get host client: %v", err)
	}

	imageContainerPath, archivePath, err := prepareArchive(ctx, hostClient, refs)
	if err != nil {
		return fmt.Errorf("unable to prepare the archive: %v", err)
	}
	defer os.Remove(archivePath)

	containers, err := c.ContainerList(ctx)
	if err != nil {
		return fmt.Errorf("unable to get container list %v", err)
	}

	var errg errgroup.Group
	for _, container := range containers {
		cID := container.ID
		errg.Go(func() error {
			return copyToContainer(ctx, hostClient, archivePath, cID)
		})
	}

	if err = errg.Wait(); err != nil {
		return fmt.Errorf("unable to deploy the image to host: %v", err)
	}

	errg = errgroup.Group{}
	for _, container := range containers {
		cID := container.ID
		errg.Go(func() error {
			return execContainer(
				ctx,
				hostClient,
				cID,
				[]string{
					"docker",
					"load",
					"-i",
					imageContainerPath,
				},
			)
		})
	}

	if err = errg.Wait(); err != nil {
		return fmt.Errorf("unable to load the image on the host: %v", err)
	}

	return nil
}

func copyToContainer(ctx context.Context, client *docker.Client, filePath, containerID string) error {
	file, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("unable to open file to deploy: %v", err)
	}

	defer file.Close()

	if err := client.CopyToContainer(ctx, containerID, "/", file, types.CopyToContainerOptions{}); err != nil {
		return fmt.Errorf("unable to copy the image to container %s: %v", containerID, err)
	}

	return nil
}

func prepareArchive(ctx context.Context, hostClient *docker.Client, refs []string) (string, string, error) {
	imgsFile, err := ioutil.TempFile("", "img_sind")
	if err != nil {
		return "", "", fmt.Errorf("unable to create the image file: %v", err)
	}

	defer func() {
		imgsFile.Close()
		os.Remove(imgsFile.Name())
	}()

	imgReader, err := hostClient.ImageSave(ctx, refs)
	if err != nil {
		return "", "", fmt.Errorf("unable to save the images to disk: %v", err)
	}
	defer imgReader.Close()

	if bytes, err := io.Copy(imgsFile, imgReader); err != nil {
		return "", "", fmt.Errorf("unable to save the images to disk (copied %d): %v", bytes, err)
	}

	if _, err = imgsFile.Seek(0, 0); err != nil {
		return "", "", fmt.Errorf("unable to seek to the begining of the image file: %v", err)
	}

	tarImgsFile, err := ioutil.TempFile("", "tar_img_sind")
	if err != nil {
		return "", "", fmt.Errorf("unable to create the tar file: %v", err)
	}
	defer tarImgsFile.Close()

	imgsFileInfo, err := imgsFile.Stat()
	if err != nil {
		return "", "", fmt.Errorf("unabel to collect images file info: %v", err)
	}

	tarWriter := tar.NewWriter(tarImgsFile)

	err = tarWriter.WriteHeader(
		&tar.Header{
			Typeflag: tar.TypeReg,
			Name:     imgsFileInfo.Name(),
			Size:     imgsFileInfo.Size(),
			Mode:     0664,
		},
	)
	if err != nil {
		return "", "", fmt.Errorf("unable to write tar file header: %v", err)
	}

	bytes, err := io.Copy(tarWriter, imgsFile)
	if err != nil {
		return "", "", fmt.Errorf("unable to tar image files (wrote %d): %v", bytes, err)
	}

	if err = tarWriter.Close(); err != nil {
		return "", "", fmt.Errorf("unable to close the tar writer properly (wrote %d): %v", bytes, err)
	}

	return filepath.Join("/", filepath.Base(imgsFile.Name())), tarImgsFile.Name(), nil
}