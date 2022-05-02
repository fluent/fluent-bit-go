package plugin

import (
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/ory/dockertest/v3"
	dc "github.com/ory/dockertest/v3/docker"
)

func TestPlugin(t *testing.T) {
	pool, err := dockertest.NewPool("")
	wantNoErr(t, err)
	testPlugin(t, pool)
}

func testPlugin(t *testing.T, pool *dockertest.Pool) {
	auths, err := dc.NewAuthConfigurationsFromDockerCfg()
	wantNoErr(t, err)

	pwd, err := os.Getwd()
	wantNoErr(t, err)

	f, err := os.Create(filepath.Join(pwd, "testdata/output.txt"))
	wantNoErr(t, err)

	t.Cleanup(func() { _ = f.Close() })
	t.Cleanup(func() { _ = f.Truncate(0) })

	err = pool.Client.BuildImage(dc.BuildImageOptions{
		Name:         "fluent-bit-go.localhost",
		ContextDir:   filepath.Join(pwd, ".."),
		Dockerfile:   "plugin/testdata/Dockerfile",
		Platform:     "linux/amd64",
		OutputStream: io.Discard,
		Pull:         true,
		AuthConfigs:  *auths,
	})
	wantNoErr(t, err)

	fbit, err := pool.Client.CreateContainer(dc.CreateContainerOptions{
		Config: &dc.Config{
			Image: "fluent-bit-go.localhost",
		},
		HostConfig: &dc.HostConfig{
			AutoRemove: true,
			Mounts: []dc.HostMount{
				{
					Source: f.Name(),
					Target: "/fluent-bit/etc/output.txt",
					Type:   "bind",
				},
			},
		},
	})
	wantNoErr(t, err)

	t.Cleanup(func() {
		_ = pool.Client.RemoveContainer(dc.RemoveContainerOptions{
			ID: fbit.ID,
		})
	})

	go func() {
		if testing.Verbose() {
			_ = pool.Client.Logs(dc.LogsOptions{
				Container:   fbit.ID,
				ErrorStream: os.Stderr,
				Stderr:      true,
				Follow:      true,
			})
		}
	}()

	err = pool.Client.StartContainer(fbit.ID, nil)
	wantNoErr(t, err)

	// fluentbit runs for at least 5 seconds.
	time.Sleep(time.Second * 5)

	err = pool.Client.StopContainer(fbit.ID, 5)
	wantNoErr(t, err)

	contents, err := io.ReadAll(f)
	wantNoErr(t, err)

	lines := strings.Split(string(contents), "\n")

	// after 5 seconds of fluentbit running, there should be at least one record.
	if d := len(lines); d < 1 {
		t.Fatalf("expected at least 1 lines, got %d", d)
	}

	// Input plugin sends:
	//
	//	Message{
	//		Time: time.Now(),
	//		Record: map[string]string{
	//			"message": "hello from go-test-input-plugin",
	//			"foo":     foo,
	//		},
	//	}
	//
	// Output plugin writes to file:
	//
	//	fmt.Fprintf(f, "message=\"got record\" tag=%s time=%s record=%+v\n", msg.Tag(), msg.Time.Format(time.RFC3339), msg.Record)
	re := regexp.MustCompile(`^message="got record" tag=test-input time=[^\s]+ record=map\[foo:bar message:hello from go-test-input-plugin]$`)

	// fluentbit runs for 5 seconds, so at most we could get 5 records if they are collected every one second.
	for _, line := range lines[0:5] {
		if line == "" {
			break
		}

		if !re.MatchString(line) {
			t.Fatalf("line %q does not match regexp %q", line, re)
		}
	}
}

func wantNoErr(t *testing.T, err error) {
	t.Helper()

	if err != nil {
		t.Fatal(err)
	}
}
