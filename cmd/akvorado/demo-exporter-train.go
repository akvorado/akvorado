// SPDX-FileCopyrightText: 2026 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

//go:build !akvorado_inlet

package main

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"akvorado/common/clickhousedb"
	"akvorado/common/daemon"
	"akvorado/common/reporter"
	"akvorado/demoexporter/flowmodel"
	"akvorado/demoexporter/flowmodel/extract"
)

// DemoExporterTrainConfiguration represents the configuration file
// for the demo exporter train command.
type DemoExporterTrainConfiguration struct {
	Reporting  reporter.Configuration
	ClickHouse clickhousedb.Configuration
	Training   extract.Configuration `mapstructure:",squash" yaml:",inline"`
	// Output is the path of the model file to write.
	Output string `validate:"required"`
}

// Reset sets the default configuration for the demo exporter train command.
func (c *DemoExporterTrainConfiguration) Reset() {
	*c = DemoExporterTrainConfiguration{
		Reporting:  reporter.DefaultConfiguration(),
		ClickHouse: clickhousedb.DefaultConfiguration(),
		Training:   extract.DefaultConfiguration(),
	}
}

type demoExporterTrainOptions struct {
	ConfigRelatedOptions
	CheckMode bool
	Cache     string
}

// DemoExporterTrainOptions stores the command-line option values for
// the demo exporter train command.
var DemoExporterTrainOptions demoExporterTrainOptions

var demoExporterTrainCmd = &cobra.Command{
	Use:   "train config.yaml",
	Short: "Train the demo exporter flow model",
	Long: `Train the small generative flow model used by the demo exporter from
the flows stored in a ClickHouse database. Customer prefixes are
anonymized.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		config := DemoExporterTrainConfiguration{}
		DemoExporterTrainOptions.Path = args[0]
		if _, err := DemoExporterTrainOptions.Parse(cmd.OutOrStdout(), "demo-exporter-train", &config); err != nil {
			return err
		}

		r, err := reporter.New(config.Reporting)
		if err != nil {
			return fmt.Errorf("unable to initialize reporter: %w", err)
		}
		return demoExporterTrainStart(r, config, DemoExporterTrainOptions)
	},
}

func init() {
	demoExporterCmd.AddCommand(demoExporterTrainCmd)
	demoExporterTrainCmd.Flags().BoolVarP(&DemoExporterTrainOptions.ConfigRelatedOptions.Dump, "dump", "D", false,
		"Dump configuration before starting")
	demoExporterTrainCmd.Flags().BoolVarP(&DemoExporterTrainOptions.CheckMode, "check", "C", false,
		"Check configuration, but does not start")
	demoExporterTrainCmd.Flags().StringVar(&DemoExporterTrainOptions.Cache, "cache", "",
		"File to save extracted data to, or to load it from when it exists")
}

func demoExporterTrainStart(r *reporter.Reporter, config DemoExporterTrainConfiguration, options demoExporterTrainOptions) error {
	if err := config.Training.Check(); err != nil {
		return fmt.Errorf("invalid training configuration: %w", err)
	}
	if _, err := os.Stat(filepath.Dir(config.Output)); err != nil {
		return fmt.Errorf("invalid output directory: %w", err)
	}
	daemonComponent, err := daemon.New(r)
	if err != nil {
		return fmt.Errorf("unable to initialize daemon component: %w", err)
	}
	clickhouseComponent, err := clickhousedb.New(r, config.ClickHouse, clickhousedb.Dependencies{
		Daemon: daemonComponent,
	})
	if err != nil {
		return fmt.Errorf("unable to initialize ClickHouse component: %w", err)
	}
	if options.CheckMode {
		return nil
	}
	defer clickhouseComponent.Close()

	result, err := demoExporterTrainExtract(r, clickhouseComponent, config, options.Cache)
	if err != nil {
		return err
	}
	model := flowmodel.New(result.Metadata, config.Training.Train.Seed)
	r.Info().Int("parameters", model.Parameters()).Msg("training started")
	if err := model.Train(result.Examples, config.Training.Train, func(epoch int, loss float64) {
		r.Info().Msgf("epoch %d/%d: loss %.4f", epoch, config.Training.Train.Epochs, loss)
	}); err != nil {
		return fmt.Errorf("unable to train the model: %w", err)
	}

	if err := model.SaveToPath(config.Output); err != nil {
		return err
	}
	r.Info().Msgf("model written to %s", config.Output)
	return nil
}

// demoExporterTrainExtract returns the training data, from the cache when it
// exists or else from ClickHouse.
func demoExporterTrainExtract(r *reporter.Reporter, conn *clickhousedb.Component, config DemoExporterTrainConfiguration, cache string) (*extract.Result, error) {
	if cache != "" {
		result, err := extract.LoadFromPath(cache)
		if err == nil {
			// Only the model dimensions and the training options apply to
			// cached data.
			result.Metadata.Hyper.EmbeddingDim = config.Training.Model.EmbeddingDim
			result.Metadata.Hyper.HiddenDim = config.Training.Model.HiddenDim
			r.Info().Msgf("training data loaded from %s, other settings are ignored", cache)
			return result, nil
		}
		if !errors.Is(err, fs.ErrNotExist) {
			return nil, fmt.Errorf("unable to load training data: %w", err)
		}
	}
	result, err := extract.Extract(context.Background(), r, conn, config.Training)
	if err != nil {
		return nil, fmt.Errorf("unable to extract training data: %w", err)
	}
	if cache != "" {
		if err := result.SaveToPath(cache); err != nil {
			return nil, fmt.Errorf("unable to save training data: %w", err)
		}
		r.Info().Msgf("training data saved to %s", cache)
	}
	return result, nil
}
