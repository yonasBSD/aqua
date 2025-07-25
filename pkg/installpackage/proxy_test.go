package installpackage_test

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/aquaproj/aqua/v2/pkg/checksum"
	"github.com/aquaproj/aqua/v2/pkg/config"
	"github.com/aquaproj/aqua/v2/pkg/cosign"
	"github.com/aquaproj/aqua/v2/pkg/download"
	"github.com/aquaproj/aqua/v2/pkg/ghattestation"
	"github.com/aquaproj/aqua/v2/pkg/installpackage"
	"github.com/aquaproj/aqua/v2/pkg/minisign"
	"github.com/aquaproj/aqua/v2/pkg/runtime"
	"github.com/aquaproj/aqua/v2/pkg/slsa"
	"github.com/aquaproj/aqua/v2/pkg/testutil"
	"github.com/aquaproj/aqua/v2/pkg/unarchive"
	"github.com/aquaproj/aqua/v2/pkg/vacuum"
	"github.com/sirupsen/logrus"
)

func Test_installer_InstallProxy(t *testing.T) {
	t.Parallel()
	data := []struct {
		name     string
		files    map[string]string
		param    *config.Param
		rt       *runtime.Runtime
		executor installpackage.Executor
		links    map[string]string
		isErr    bool
	}{
		{
			name: "file already exists",
			rt: &runtime.Runtime{
				GOOS:   "linux",
				GOARCH: "amd64",
			},
			param: &config.Param{
				RootDir:        "/home/foo/.local/share/aquaproj-aqua",
				PWD:            "/home/foo/workspace",
				ConfigFilePath: "aqua.yaml",
				MaxParallelism: 5,
			},
			files: map[string]string{
				fmt.Sprintf("/home/foo/.local/share/aquaproj-aqua/internal/pkgs/github_release/github.com/aquaproj/aqua-proxy/%s/aqua-proxy_linux_amd64.tar.gz/aqua-proxy", installpackage.ProxyVersion): "",
			},
		},
	}
	logE := logrus.NewEntry(logrus.New())
	for _, d := range data {
		t.Run(d.name, func(t *testing.T) {
			t.Parallel()
			ctx := t.Context()
			fs, err := testutil.NewFs(d.files)
			if err != nil {
				t.Fatal(err)
			}
			linker := installpackage.NewMockLinker(fs)
			for dest, src := range d.links {
				if err := linker.Symlink(dest, src); err != nil {
					t.Fatal(err)
				}
			}
			downloader := download.NewDownloader(nil, download.NewHTTPDownloader(logE, http.DefaultClient))
			vacuumMock := vacuum.NewMock(d.param.RootDir, nil, nil)
			ctrl := installpackage.New(d.param, downloader, d.rt, fs, linker, nil, &checksum.Calculator{}, unarchive.New(d.executor, fs), &cosign.MockVerifier{}, &slsa.MockVerifier{}, &minisign.MockVerifier{}, &ghattestation.MockVerifier{}, &installpackage.MockGoInstallInstaller{}, &installpackage.MockGoBuildInstaller{}, &installpackage.MockCargoPackageInstaller{}, vacuumMock)
			if err := ctrl.InstallProxy(ctx, logE); err != nil {
				if d.isErr {
					return
				}
				t.Fatal(err)
			}
			if d.isErr {
				t.Fatal("error must be returned")
			}
		})
	}
}
