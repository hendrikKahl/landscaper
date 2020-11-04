// SPDX-FileCopyrightText: 2020 SAP SE or an SAP affiliate company and Gardener contributors.
//
// SPDX-License-Identifier: Apache-2.0

package componentdescriptor

import (
	"context"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	cdv2 "github.com/gardener/component-spec/bindings-go/apis/v2"
	"github.com/gardener/component-spec/bindings-go/codec"
	"github.com/go-logr/logr"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/gardener/landscaper/cmd/landscaper-cli/app/constants"
	componentsregistry "github.com/gardener/landscaper/pkg/landscaper/registry/components"
	"github.com/gardener/landscaper/pkg/landscaper/registry/components/cdutils"
	"github.com/gardener/landscaper/pkg/logger"
	"github.com/gardener/landscaper/pkg/utils/oci"
	"github.com/gardener/landscaper/pkg/utils/oci/cache"
)

type pushOptions struct {
	// baseUrl is the oci registry where the component is stored.
	baseUrl string
	// componentName is the unique name of the component in the registry.
	componentName string
	// version is the component version in the oci registry.
	version string
	// componentPath is the path to the directory containing the definition.
	componentPath string

	// ref is the oci artifact uri reference to the uploaded component descriptor
	ref string
	// cd is the effective component descriptor
	cd *cdv2.ComponentDescriptor
	// cacheDir defines the oci cache directory
	cacheDir string
}

// NewPushCommand creates a new definition command to push definitions
func NewPushCommand(ctx context.Context) *cobra.Command {
	opts := &pushOptions{}
	cmd := &cobra.Command{
		Use:  "push",
		Args: cobra.RangeArgs(1, 4),
		Example: `landscapercli cd push [path to component descriptor]
- The cli will read all necessary parameters from the component descriptor.

landscapercli cd push [baseurl] [componentname] [version] [path to component descriptor]
- The cli will add the baseurl as repository context and validate the name and version.
`,
		Short: "command to interact with a component descriptor stored an oci registry",
		Run: func(cmd *cobra.Command, args []string) {
			if err := opts.Complete(args); err != nil {
				fmt.Println(err.Error())
				os.Exit(1)
			}

			if err := opts.run(ctx, logger.Log); err != nil {
				fmt.Println(err.Error())
				os.Exit(1)
			}

			fmt.Printf("Successfully uploaded %s\n", opts.ref)
		},
	}

	opts.AddFlags(cmd.Flags())

	return cmd
}

func (o *pushOptions) run(ctx context.Context, log logr.Logger) error {
	cache, err := cache.NewCache(log, cache.WithBasePath(o.cacheDir))
	if err != nil {
		return err
	}

	data, err := ioutil.ReadFile(o.componentPath)
	if err != nil {
		return err
	}

	defManifest, err := cdutils.BuildNewManifest(cache, data)
	if err != nil {
		return err
	}

	ociClient, err := oci.NewClient(log, oci.WithCache{Cache: cache}, oci.WithKnownMediaType(componentsregistry.ComponentDescriptorMediaType))
	if err != nil {
		return err
	}

	return ociClient.PushManifest(ctx, o.ref, defManifest)
}

func (o *pushOptions) Complete(args []string) error {
	switch len(args) {
	case 1:
		o.componentPath = args[0]
	case 4:
		o.baseUrl = args[0]
		o.componentName = args[1]
		o.version = args[2]
		o.componentPath = args[3]
	}

	landscaperCliHomeDir, err := constants.LandscaperCliHomeDir()
	if err != nil {
		return err
	}
	o.cacheDir = filepath.Join(landscaperCliHomeDir, "components")
	if err := os.MkdirAll(o.cacheDir, os.ModePerm); err != nil {
		return fmt.Errorf("unable to create cache directory %s: %w", o.cacheDir, err)
	}

	if err := o.Validate(); err != nil {
		return err
	}

	data, err := ioutil.ReadFile(o.componentPath)
	if err != nil {
		return err
	}
	o.cd = &cdv2.ComponentDescriptor{}
	if err := codec.Decode(data, o.cd); err != nil {
		return err
	}

	if len(o.componentName) != 0 {
		if o.cd.Name != o.componentName {
			return fmt.Errorf("name in component descriptor '%s' does not match the given name '%s'", o.cd.Name, o.componentName)
		}
		if o.cd.Version != o.version {
			return fmt.Errorf("version in component descriptor '%s' does not match the given version '%s'", o.cd.Version, o.version)
		}
	}

	if len(o.baseUrl) != 0 {
		o.cd.RepositoryContexts = append(o.cd.RepositoryContexts, cdv2.RepositoryContext{
			Type:    cdv2.OCIRegistryType,
			BaseURL: o.baseUrl,
		})
	}

	repoCtx := o.cd.GetEffectiveRepositoryContext()
	obj := cdv2.ObjectMeta{
		Name:    o.cd.Name,
		Version: o.cd.Version,
	}
	o.ref, err = componentsregistry.OCIRef(repoCtx, obj)
	if err != nil {
		return fmt.Errorf("invalid component reference: %w", err)
	}
	return nil
}

// Validate validates push options
func (o *pushOptions) Validate() error {
	if len(o.componentPath) == 0 {
		return errors.New("a path to the component descriptor must be defined")
	}

	if len(o.cacheDir) == 0 {
		return errors.New("a oci cache directory must be defined")
	}

	// todo: validate references exist
	return nil
}

func (o *pushOptions) AddFlags(_ *pflag.FlagSet) {}
