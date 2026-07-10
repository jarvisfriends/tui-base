// Command rendertapes renders every demo tape to a gif by running the
// official VHS container (ported from snap/tools/rendertapes): rendering
// inside Docker produces consistent output across hosts, and Windows-native
// vhs hangs (see ROADMAP SP-15).
//
// Tapes run in parallel on a worker pool sized to the CPU count, so large
// tape sets finish fast without making the host unusable. The Docker Go
// client talks to Docker Desktop or Podman alike (Podman's Docker-compatible
// socket / DOCKER_HOST are honored automatically).
//
// Usage, from the tui-base repo root:
//
//	go -C tools/rendertapes run . [-image ghcr.io/charmbracelet/vhs] [-workers N]
//
// This is a standalone module so the Docker client dependency never enters
// tui-base's library graph.
package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/stdcopy"
)

func main() {
	log.SetFlags(0)
	if err := run(); err != nil {
		log.Fatalf("rendertapes: %v", err)
	}
}

// run holds the real body so deferred cleanup (demo binaries, the Docker
// client) executes on error paths — log.Fatal in main would skip defers.
func run() error {
	var (
		imageRef = flag.String("image", "ghcr.io/charmbracelet/vhs", "VHS container image")
		workers  = flag.Int("workers", runtime.NumCPU(), "parallel renders (default: CPU count)")
		repoRoot = flag.String("root", "", "tui-base repo root (default: two levels up from this tool)")
	)
	flag.Parse()

	root := *repoRoot
	if root == "" {
		wd, err := os.Getwd()
		if err != nil {
			return err
		}
		root = filepath.Dir(filepath.Dir(wd)) // tools/rendertapes -> repo root
	}
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return err
	}
	root = absRoot

	tapes, err := findTapes(root)
	if err != nil {
		return fmt.Errorf("scan for *.tape under %s: %w", root, err)
	}
	if len(tapes) == 0 {
		return fmt.Errorf("no *.tape files found under %s", root)
	}

	if buildErr := buildDemoBinaries(root); buildErr != nil {
		return buildErr
	}
	defer cleanDemoBinaries(root)

	ctx := context.Background()
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return fmt.Errorf("docker/podman client: %w", err)
	}
	defer cli.Close() //nolint:errcheck // process exit follows

	if err := ensureImage(ctx, cli, *imageRef); err != nil {
		return fmt.Errorf("pull %s: %w", *imageRef, err)
	}

	jobs := make(chan string)
	var wg sync.WaitGroup
	var mu sync.Mutex
	failures := 0

	for range max(*workers, 1) {
		wg.Go(func() {
			for tape := range jobs {
				rel, _ := filepath.Rel(root, tape)
				rel = filepath.ToSlash(rel)
				log.Printf("==> %s", rel)
				if err := renderTape(ctx, cli, *imageRef, root, rel); err != nil {
					mu.Lock()
					failures++
					mu.Unlock()
					log.Printf("FAIL %s: %v", rel, err)
					continue
				}
				log.Printf("ok   %s", rel)
			}
		})
	}
	for _, t := range tapes {
		jobs <- t
	}
	close(jobs)
	wg.Wait()

	if failures > 0 {
		return fmt.Errorf("%d of %d tape(s) failed", failures, len(tapes))
	}
	log.Printf("rendertapes: %d gif(s) rendered", len(tapes))
	return nil
}

// findTapes walks the repo for every *.tape file (any name, any depth —
// a package can ship several, e.g. charts/sparkline.tape next to
// charts/pie.tape), skipping VCS and tool directories.
func findTapes(root string) ([]string, error) {
	var tapes []string
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			switch d.Name() {
			case ".git", "tools", "node_modules":
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasSuffix(d.Name(), ".tape") {
			tapes = append(tapes, path)
		}
		return nil
	})
	return tapes, err
}

// ensureImage pulls the VHS image when it is not already present.
func ensureImage(ctx context.Context, cli *client.Client, ref string) error {
	if _, _, err := cli.ImageInspectWithRaw(ctx, ref); err == nil {
		return nil
	}
	rc, err := cli.ImagePull(ctx, ref, image.PullOptions{})
	if err != nil {
		return err
	}
	defer rc.Close() //nolint:errcheck // drain-and-close of a pull stream
	_, err = io.Copy(io.Discard, rc)
	return err
}

// renderTape runs one tape in its own container with the repo mounted at
// /vhs (the image's working directory), mirroring
// `docker run --rm -v <root>:/vhs ghcr.io/charmbracelet/vhs <tape>`.
func renderTape(ctx context.Context, cli *client.Client, imageRef, root, relTape string) error {
	created, err := cli.ContainerCreate(ctx,
		&container.Config{
			Image: imageRef,
			Cmd:   []string{relTape},
		},
		&container.HostConfig{
			Binds:      []string{root + ":/vhs"},
			AutoRemove: false, // removed explicitly after logs are collected
		},
		nil, nil, "")
	if err != nil {
		return err
	}
	id := created.ID
	defer func() {
		_ = cli.ContainerRemove(context.WithoutCancel(ctx), id, container.RemoveOptions{Force: true})
	}()

	if err := cli.ContainerStart(ctx, id, container.StartOptions{}); err != nil {
		return err
	}

	waitC, errC := cli.ContainerWait(ctx, id, container.WaitConditionNotRunning)
	select {
	case err := <-errC:
		return err
	case status := <-waitC:
		if status.StatusCode != 0 {
			return fmt.Errorf("vhs exited %d:\n%s", status.StatusCode, containerLogs(ctx, cli, id))
		}
	}
	return nil
}

// containerLogs collects a failed container's output for the error message.
func containerLogs(ctx context.Context, cli *client.Client, id string) string {
	rc, err := cli.ContainerLogs(ctx, id, container.LogsOptions{ShowStdout: true, ShowStderr: true})
	if err != nil {
		return "(logs unavailable: " + err.Error() + ")"
	}
	defer rc.Close() //nolint:errcheck // read-only stream
	var buf writerBuf
	_, _ = stdcopy.StdCopy(&buf, &buf, rc)
	return buf.String()
}

// writerBuf is a minimal strings.Builder-compatible io.Writer.
type writerBuf struct{ b []byte }

func (w *writerBuf) Write(p []byte) (int, error) { w.b = append(w.b, p...); return len(p), nil }
func (w *writerBuf) String() string              { return string(w.b) }

// buildDemoBinaries cross-compiles every example and command for linux/amd64
// into <dir>/demo-bin — the vhs container has no Go toolchain, so the tapes
// run prebuilt binaries (this is also what failed silently before: `go build`
// inside the container left nothing to run).
func buildDemoBinaries(root string) error {
	examples, err := filepath.Glob(filepath.Join(root, "examples", "*"))
	if err != nil {
		return err
	}
	cmds, err := filepath.Glob(filepath.Join(root, "cmd", "*"))
	if err != nil {
		return err
	}
	examples = append(examples, cmds...)
	for _, dir := range examples {
		st, err := os.Stat(dir)
		if err != nil || !st.IsDir() {
			continue
		}
		out := filepath.Join(dir, "demo-bin")
		cmd := exec.Command("go", "build", "-o", out, "./"+filepath.ToSlash(mustRel(root, dir)))
		cmd.Dir = root
		cmd.Env = append(os.Environ(), "GOOS=linux", "GOARCH=amd64", "CGO_ENABLED=0")
		if outb, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("build %s: %w\n%s", dir, err, outb)
		}
		log.Printf("built %s", filepath.ToSlash(mustRel(root, out)))
	}
	return nil
}

// cleanDemoBinaries removes the cross-compiled demo binaries after rendering.
func cleanDemoBinaries(root string) {
	matches, _ := filepath.Glob(filepath.Join(root, "examples", "*", "demo-bin"))
	cmds, _ := filepath.Glob(filepath.Join(root, "cmd", "*", "demo-bin"))
	for _, m := range append(matches, cmds...) {
		_ = os.Remove(m)
	}
}

func mustRel(base, target string) string {
	r, err := filepath.Rel(base, target)
	if err != nil {
		return target
	}
	return r
}
