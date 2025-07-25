package install

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/aquaproj/aqua/v2/pkg/checksum"
	"github.com/aquaproj/aqua/v2/pkg/config"
	finder "github.com/aquaproj/aqua/v2/pkg/config-finder"
	"github.com/aquaproj/aqua/v2/pkg/config/aqua"
	"github.com/aquaproj/aqua/v2/pkg/installpackage"
	"github.com/aquaproj/aqua/v2/pkg/osfile"
	"github.com/aquaproj/aqua/v2/pkg/policy"
	"github.com/sirupsen/logrus"
	"github.com/suzuki-shunsuke/logrus-error/logerr"
)

// Install is a main method of "install" command.
// This method is also called by "cp" command.
func (c *Controller) Install(ctx context.Context, logE *logrus.Entry, param *config.Param) error {
	if param.Dest == "" {
		// Create a "bin" directory and install aqua-proxy in advance.
		// If param.Dest isn't empty, this means this method is called by "copy" command.
		// If the command is "copy", this block is skipped.
		if err := c.mkBinDir(); err != nil {
			return err
		}
		if err := c.packageInstaller.InstallProxy(ctx, logE); err != nil {
			return fmt.Errorf("install aqua-proxy: %w", err)
		}
	}

	policyCfgs, err := c.policyReader.Read(param.PolicyConfigFilePaths)
	if err != nil {
		return fmt.Errorf("read policy files: %w", err)
	}

	globalPolicyPaths := make(map[string]struct{}, len(param.PolicyConfigFilePaths))
	for _, p := range param.PolicyConfigFilePaths {
		globalPolicyPaths[p] = struct{}{}
	}

	for _, cfgFilePath := range c.configFinder.Finds(param.PWD, param.ConfigFilePath) {
		policyCfgs, err := c.policyReader.Append(logE, cfgFilePath, policyCfgs, globalPolicyPaths)
		if err != nil {
			return fmt.Errorf("append policy configs: %w", logerr.WithFields(err, logrus.Fields{
				"config_file_path": cfgFilePath,
			}))
		}
		if err := c.install(ctx, logE, cfgFilePath, policyCfgs, param); err != nil {
			return fmt.Errorf("install packages: %w", logerr.WithFields(err, logrus.Fields{
				"config_file_path": cfgFilePath,
			}))
		}
	}

	return c.installAll(ctx, logE, param, policyCfgs, globalPolicyPaths)
}

func (c *Controller) mkBinDir() error {
	if err := osfile.MkdirAll(c.fs, filepath.Join(c.rootDir, "bin")); err != nil {
		return fmt.Errorf("create the directory: %w", err)
	}
	if c.runtime.IsWindows() {
		if err := c.fs.RemoveAll(filepath.Join(c.rootDir, "bat")); err != nil {
			return fmt.Errorf("remove the bat directory: %w", err)
		}
	}
	return nil
}

func (c *Controller) installAll(ctx context.Context, logE *logrus.Entry, param *config.Param, policyConfigs []*policy.Config, globalPolicyPaths map[string]struct{}) error {
	if !param.All {
		return nil
	}
	for _, cfgFilePath := range param.GlobalConfigFilePaths {
		if _, err := c.fs.Stat(cfgFilePath); err != nil {
			continue
		}
		policyConfigs, err := c.policyReader.Append(logE, cfgFilePath, policyConfigs, globalPolicyPaths)
		if err != nil {
			return fmt.Errorf("append policy configs: %w", logerr.WithFields(err, logrus.Fields{
				"config_file_path": cfgFilePath,
			}))
		}
		if err := c.install(ctx, logE, cfgFilePath, policyConfigs, param); err != nil {
			return fmt.Errorf("install packages: %w", logerr.WithFields(err, logrus.Fields{
				"config_file_path": cfgFilePath,
			}))
		}
	}
	return nil
}

func (c *Controller) install(ctx context.Context, logE *logrus.Entry, cfgFilePath string, policyConfigs []*policy.Config, param *config.Param) error {
	cfg := &aqua.Config{}
	if cfgFilePath == "" {
		return finder.ErrConfigFileNotFound
	}
	if err := c.configReader.Read(logE, cfgFilePath, cfg); err != nil {
		return err //nolint:wrapcheck
	}
	if err := cfg.Validate(); err != nil {
		return fmt.Errorf("validate the configuration: %w", err)
	}

	checksums, updateChecksum, err := checksum.Open(
		logE, c.fs, cfgFilePath, param.ChecksumEnabled(cfg))
	if err != nil {
		return fmt.Errorf("read a checksum JSON: %w", err)
	}
	defer updateChecksum()

	registryContents, err := c.registryInstaller.InstallRegistries(ctx, logE, cfg, cfgFilePath, checksums)
	if err != nil {
		return err //nolint:wrapcheck
	}

	return c.packageInstaller.InstallPackages(ctx, logE, &installpackage.ParamInstallPackages{ //nolint:wrapcheck
		Config:          cfg,
		Registries:      registryContents,
		ConfigFilePath:  cfgFilePath,
		SkipLink:        c.skipLink,
		Tags:            c.tags,
		ExcludedTags:    c.excludedTags,
		PolicyConfigs:   policyConfigs,
		Checksums:       checksums,
		RequireChecksum: cfg.RequireChecksum(param.EnforceRequireChecksum, param.RequireChecksum),
		DisablePolicy:   param.DisablePolicy,
	})
}
