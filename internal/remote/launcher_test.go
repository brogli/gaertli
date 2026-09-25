package remote

import (
	"errors"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"github.com/brogli/nebegit/internal/core"
	"github.com/brogli/nebegit/internal/core/coretest"
)

func TestQA_PORT_02_ADeclinedConsentSetsUpNoCommand(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	dc := NewDistroCore("Ubuntu-22.04", "1.0.0", `C:\nebegit.exe`, localRunner, &fakeConsent{accept: false}, nil, slog.Default())

	_, _, err := dc.Ensure(t.Context())

	coretest.AssertCode(t, err, core.DeployRefused)
	for _, path := range []string{".cache/nebegit/1.0.0/gui-path", ".local/bin/nebegit"} {
		if _, statErr := os.Lstat(filepath.Join(os.Getenv("HOME"), path)); !errors.Is(statErr, fs.ErrNotExist) {
			t.Errorf("%s exists after a declined consent", path)
		}
	}
}
