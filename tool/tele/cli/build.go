/*
Copyright 2018 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package cli

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/gravitational/gravity/lib/builder"
	"github.com/gravitational/gravity/lib/loc"
	"github.com/gravitational/gravity/tool/common"

	"github.com/buger/goterm"
	"github.com/gravitational/trace"
)

func buildClusterImage(ctx context.Context, params BuildParameters) error {
	clusterBuilder, err := builder.NewClusterBuilder(params.BuilderConfig())
	if err != nil {
		return trace.Wrap(err)
	}
	defer clusterBuilder.Close()
	return clusterBuilder.Build(ctx, builder.ClusterRequest{
		SourcePath:    params.SourcePath,
		OutputPath:    params.OutPath,
		Overwrite:     params.Overwrite,
		BaseImage:     params.BaseImage,
		Vendor:        params.Vendor,
		From:          params.UpgradeFrom,
		SkipBaseCheck: params.SkipBaseCheck,
	})
}

func diffClusterImage(ctx context.Context, params BuildParameters) error {
	new, err := builder.InspectCluster(ctx, params.SourcePath, params.Vendor)
	if err != nil {
		return trace.Wrap(err)
	}
	old, err := builder.InspectImage(ctx, params.UpgradeFrom)
	if err != nil {
		return trace.Wrap(err)
	}
	printDiff(old, new)
	return nil
}

func buildApplicationImage(ctx context.Context, params BuildParameters) error {
	appBuilder, err := builder.NewApplicationBuilder(params.BuilderConfig())
	if err != nil {
		return trace.Wrap(err)
	}
	defer appBuilder.Close()
	return appBuilder.Build(ctx, builder.ApplicationRequest{
		ChartPath:  params.SourcePath,
		OutputPath: params.OutPath,
		Overwrite:  params.Overwrite,
		Vendor:     params.Vendor,
		From:       params.UpgradeFrom,
	})
}

func diffApplicationImage(ctx context.Context, params BuildParameters) error {
	new, err := builder.InspectChart(ctx, params.SourcePath, params.Vendor)
	if err != nil {
		return trace.Wrap(err)
	}
	old, err := builder.InspectImage(ctx, params.UpgradeFrom)
	if err != nil {
		return trace.Wrap(err)
	}
	printDiff(old, new)
	return nil
}

func printDiff(oldImage, newImage *builder.InspectResponse) {
	diffResults := loc.DiffDockerImages(oldImage.Images, newImage.Images)
	t := goterm.NewTable(0, 10, 5, ' ', 0)
	common.PrintTableHeader(t, []string{"",
		oldImage.Manifest.Locator().Description(),
		newImage.Manifest.Locator().Description()})
	for _, diff := range diffResults {
		var oldTags, newTags []string
		for _, tag := range diff.Tags {
			if tag.Left {
				if !tag.Right {
					oldTags = append(oldTags, fmt.Sprintf("%v (removed)", tag.Tag))
				} else {
					oldTags = append(oldTags, tag.Tag)
				}
			}
			if tag.Right {
				if !tag.Left {
					newTags = append(newTags, fmt.Sprintf("%v (added)", tag.Tag))
				} else {
					newTags = append(newTags, tag.Tag)
				}
			}
		}
		fmt.Fprintf(t, "%v\t%v\t%v\n", diff.Repository,
			strings.Join(oldTags, ", "),
			strings.Join(newTags, ", "))
	}
	io.WriteString(os.Stdout, t.String())
}
