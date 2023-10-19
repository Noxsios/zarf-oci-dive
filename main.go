package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/charmbracelet/log"
	"github.com/defenseunicorns/zarf/src/pkg/oci"
)

var l = log.NewWithOptions(os.Stderr, log.Options{
	ReportTimestamp: false,
})

func JSON(v any) string {
	bytes, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		l.Fatal("failed to marshal json", "err", err)
	}
	return string(bytes)
}

func main() {
	args := os.Args[1:]

	if len(args) != 1 {
		l.Fatal("usage: dive <url>")
	}

	url := args[0]
	if err := dive(url); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func dive(url string) error {
	remote, err := oci.NewOrasRemote(url)
	if err != nil {
		return err
	}

	desc, err := remote.ResolveRoot()
	if err != nil {
		return fmt.Errorf("failed to resolve root: %w", err)
	}

	manifest, err := remote.FetchManifest(desc)
	if err != nil {
		return fmt.Errorf("failed to fetch manifest: %w", err)
	}

	ctx := context.Background()

	configDesc := manifest.Config
	exists, err := remote.Repo().Exists(ctx, configDesc)
	if err != nil {
		return fmt.Errorf("err: %w, config DNE %s", err, JSON(configDesc))
	}
	if !exists {
		return fmt.Errorf("config DNE %s", JSON(configDesc))
	}

	layers := manifest.Layers

	l.Infof("%s@%s has %d layers", url, manifest.Config.Digest, len(layers))

	// for _, layer := range layers {
	// 	exists, err := remote.Repo().Exists(ctx, layer)
	// 	if err != nil {
	// 		return fmt.Errorf("err: %w, layer DNE %s", err, JSON(layer))
	// 	}
	// 	if !exists {
	// 		return fmt.Errorf("layer DNE %s", JSON(layer))
	// 	}
	// 	l.Infof("exists: %s", JSON(layer))
	// }

	requested := []string{}

	// l.Info("all layers exist, now checking LayersFromRequestedComponents returns no errors")

	_, err = LayersFromRequestedComponents(remote, requested)
	if err != nil {
		return fmt.Errorf("failed to get layers from requested components: %w", err)
	}

	return nil
}
