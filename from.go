package main

import (
	"fmt"
	"path/filepath"

	"slices"

	"github.com/defenseunicorns/zarf/src/pkg/layout"
	"github.com/defenseunicorns/zarf/src/pkg/oci"
	"github.com/defenseunicorns/zarf/src/pkg/utils/helpers"
	"github.com/defenseunicorns/zarf/src/types"

	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

func LayersFromRequestedComponents(o *oci.OrasRemote, requestedComponents []string) (layers []ocispec.Descriptor, err error) {
	root, err := o.FetchRoot()
	if err != nil {
		return nil, fmt.Errorf("failed to fetch root: %w", err)
	}

	pkg, err := o.FetchZarfYAML(root)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch zarf.yaml: %w", err)
	}
	images := map[string]bool{}
	tarballFormat := "%s.tar"
	for _, name := range requestedComponents {
		component := helpers.Find(pkg.Components, func(component types.ZarfComponent) bool {
			return component.Name == name
		})
		if component.Name == "" {
			return nil, fmt.Errorf("component %s does not exist in this package", name)
		}
	}
	for _, component := range pkg.Components {
		// If we requested this component, or it is required, we need to pull its images and tarball
		if slices.Contains(requestedComponents, component.Name) || component.Required {
			for _, image := range component.Images {
				images[image] = true
			}
			layers = append(layers, root.Locate(filepath.Join(layout.ComponentsDir, fmt.Sprintf(tarballFormat, component.Name))))
		}
	}
	// Append the sboms.tar layer if it exists
	//
	// Since sboms.tar is not a heavy addition 99% of the time, we'll just always pull it
	sbomsDescriptor := root.Locate(layout.SBOMTar)
	if !oci.IsEmptyDescriptor(sbomsDescriptor) {
		layers = append(layers, sbomsDescriptor)
	}
	if len(images) > 0 {
		// Add the image index and the oci-layout layers
		layers = append(layers, root.Locate(oci.ZarfPackageIndexPath), root.Locate(oci.ZarfPackageLayoutPath))
		index, err := o.FetchImagesIndex(root)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch images index: %w", err)
		}

		fmt.Println()
		l.Info("images in the index:")
		for _, desc := range index.Manifests {
			l.Info(desc.Annotations[ocispec.AnnotationBaseImageName])
		}
		fmt.Println()

		for image := range images {
			l.Infof("checking for image %s", image)

			manifestDescriptor := helpers.Find(index.Manifests, func(layer ocispec.Descriptor) bool {
				return layer.Annotations[ocispec.AnnotationBaseImageName] == image
			})

			// even though these are technically image manifests, we store them as Zarf blobs
			manifestDescriptor.MediaType = oci.ZarfLayerMediaTypeBlob

			manifest, err := o.FetchManifest(manifestDescriptor)
			if err != nil {
				return nil, fmt.Errorf("failed to fetch manifest for %q: %w", image, err)
			}
			// Add the manifest and the manifest config layers
			layers = append(layers, root.Locate(filepath.Join(oci.ZarfPackageImagesBlobsDir, manifestDescriptor.Digest.Encoded())))
			layers = append(layers, root.Locate(filepath.Join(oci.ZarfPackageImagesBlobsDir, manifest.Config.Digest.Encoded())))

			// Add all the layers from the manifest
			for _, layer := range manifest.Layers {
				layerPath := filepath.Join(oci.ZarfPackageImagesBlobsDir, layer.Digest.Encoded())
				layers = append(layers, root.Locate(layerPath))
			}
		}
	}
	return layers, nil
}
