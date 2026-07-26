// Copyright (c) 2026 Jarvis Friends contributors
// SPDX-License-Identifier: MIT

// Command genicon turns a source SVG into a multi-resolution Windows .ico and,
// optionally, the embedded resource objects (.syso) that give a compiled Go
// binary its file/taskbar icon.
//
// It is intentionally a standalone module (see go.mod) so that the SVG
// rasterizer and resource writer never leak into the tui-base library's
// dependency graph. tui-base regenerates its own assets with:
//
//	go generate ./...
//
// Apps built on tui-base brand their own binary without vendoring anything by
// pointing the published tool at their own artwork, for example:
//
//	go run github.com/jarvisfriends/tui-base/tools/genicon@latest \
//	    -svg assets/app.svg -ico assets/app.ico -syso ./cmd/app \
//	    -name "My App" -desc "My App" -version 1.4.0
//
// See docs/branding.md for the full override guide.
package main

import (
	"bytes"
	"encoding/binary"
	"flag"
	"fmt"
	"image"
	"image/png"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"cogentcore.org/core/math32"
	"cogentcore.org/core/svg"

	// Cogent Core resolves its image renderer and text shaper through
	// registration variables; the implementations register themselves via
	// these blank imports (without them, RenderImage nil-panics even for
	// text-free artwork).
	_ "cogentcore.org/core/paint/renderers"
	_ "cogentcore.org/core/text/shaped/shapers"

	"github.com/josephspurrier/goversioninfo"
	xdraw "golang.org/x/image/draw"
)

func main() {
	log.SetFlags(0)

	var (
		svgPath = flag.String("svg", "assets/icon.svg", "source SVG to rasterize")
		icoPath = flag.String("ico", "assets/icon.ico", "output .ico path")
		pngPath = flag.String(
			"png",
			"",
			"also write a square PNG here (empty: skip); handy for Windows Terminal tab icons",
		)
		pngSize = flag.Int("png-size", 256, "size (px) of the -png output")
		sysoDir = flag.String(
			"syso",
			"",
			"directory to write resource_windows_<arch>.syso into (empty: skip)",
		)
		sizesCSV = flag.String("sizes", "256,128,64,48,32,16", "comma-separated icon sizes (px)")
		archCSV  = flag.String(
			"arch",
			"amd64,arm64",
			"comma-separated GOARCH values for .syso output",
		)
		ss = flag.Int(
			"supersample",
			4,
			"render at N× each icon size then downscale (anti-aliasing quality)",
		)
		name = flag.String("name", "TUI Base", "product name recorded in the Windows resource")
		desc = flag.String(
			"desc",
			"",
			"file description recorded in the Windows resource (defaults to -name)",
		)
		version = flag.String(
			"version",
			"0.0.0.0",
			"dotted version (e.g. 1.4.0) recorded in the Windows resource",
		)
		manifest = flag.String(
			"manifest",
			"",
			"optional application manifest (.xml) to embed in the .syso",
		)
	)
	flag.Parse()

	if *desc == "" {
		*desc = *name
	}

	sizes, err := parseInts(*sizesCSV)
	if err != nil {
		log.Fatalf("genicon: -sizes: %v", err)
	}
	if len(sizes) == 0 {
		log.Fatal("genicon: -sizes must list at least one size")
	}
	for _, s := range sizes {
		// The ICO directory stores dimensions in a single byte (0 = 256), so
		// anything outside 1-256 would be silently mis-encoded.
		if s < 1 || s > 256 {
			log.Fatalf("genicon: -sizes: %d is outside the ICO-supported range 1-256", s)
		}
	}

	if *ss < 1 {
		*ss = 1
	}
	imgs := make([]*image.RGBA, len(sizes))
	for i, s := range sizes {
		img, err := renderSVG(*svgPath, s, *ss)
		if err != nil {
			log.Fatalf("genicon: render %s at %dpx: %v", *svgPath, s, err)
		}
		imgs[i] = img
	}

	if err := writeICO(*icoPath, imgs); err != nil {
		log.Fatalf("genicon: write %s: %v", *icoPath, err)
	}
	log.Printf("genicon: wrote %s (%d sizes: %s)", *icoPath, len(sizes), *sizesCSV)

	if *pngPath != "" {
		img, err := renderSVG(*svgPath, *pngSize, *ss)
		if err != nil {
			log.Fatalf("genicon: render %s at %dpx: %v", *svgPath, *pngSize, err)
		}
		if err := writePNG(*pngPath, img); err != nil {
			log.Fatalf("genicon: write %s: %v", *pngPath, err)
		}
		log.Printf("genicon: wrote %s (%dpx)", *pngPath, *pngSize)
	}

	if *sysoDir == "" {
		return
	}
	archs := splitCSV(*archCSV)
	if err := writeSyso(*sysoDir, *icoPath, *manifest, *name, *desc, *version, archs); err != nil {
		log.Fatalf("genicon: write resources: %v", err)
	}
}

// renderSVG rasterizes an SVG file into a size x size RGBA image using
// Cogent Core's pure-Go SVG renderer (the maintained successor lineage of
// the archived srwiley oksvg/rasterx pair — absorbed and maintained
// in-tree, so no srwiley modules appear in the graph). It renders at ss×
// the target size and then downsamples with a Catmull-Rom filter, because
// rasterizing thin strokes directly at 16–48 px aliases badly (the icon
// Windows actually shows in Explorer and the taskbar). Supersampling gives
// the tiny sizes clean, legible edges. Non-square artwork aspect-fits per
// standard SVG preserveAspectRatio semantics.
func renderSVG(path string, size, ss int) (*image.RGBA, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	hi := size * ss
	img, err := svg.SVGToImage(data, math32.Vec2(float32(hi), float32(hi)))
	if err != nil {
		return nil, err
	}

	out := image.NewRGBA(image.Rect(0, 0, size, size))
	if ss == 1 {
		xdraw.Draw(out, out.Bounds(), img, img.Bounds().Min, xdraw.Src)
		return out, nil
	}
	xdraw.CatmullRom.Scale(out, out.Bounds(), img, img.Bounds(), xdraw.Over, nil)
	return out, nil
}

// writeICO packs the rendered images into a PNG-compressed .ico container. The
// ICO directory records a width/height byte of 0 for the 256px entry, which is
// the format's convention for "256".
func writeICO(path string, imgs []*image.RGBA) error {
	blobs := make([][]byte, len(imgs))
	for i, im := range imgs {
		var pb bytes.Buffer
		if err := png.Encode(&pb, im); err != nil {
			return err
		}
		blobs[i] = pb.Bytes()
	}

	var buf bytes.Buffer
	// ICONDIR header.
	_ = binary.Write(&buf, binary.LittleEndian, uint16(0)) // reserved
	_ = binary.Write(&buf, binary.LittleEndian, uint16(1)) // type 1 = icon
	_ = binary.Write(&buf, binary.LittleEndian, uint16(len(imgs)))

	offset := 6 + 16*len(imgs) // header + one directory entry per image
	for i, im := range imgs {
		w := im.Bounds().Dx()
		h := im.Bounds().Dy()
		buf.WriteByte(dimByte(w))
		buf.WriteByte(dimByte(h))
		buf.WriteByte(0)                                                   // palette count
		buf.WriteByte(0)                                                   // reserved
		_ = binary.Write(&buf, binary.LittleEndian, uint16(1))             // color planes
		_ = binary.Write(&buf, binary.LittleEndian, uint16(32))            // bits per pixel
		_ = binary.Write(&buf, binary.LittleEndian, uint32(len(blobs[i]))) // image byte size
		_ = binary.Write(&buf, binary.LittleEndian, uint32(offset))        // image byte offset
		offset += len(blobs[i])
	}
	for _, bl := range blobs {
		buf.Write(bl)
	}

	if dir := filepath.Dir(path); dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}
	return os.WriteFile(path, buf.Bytes(), 0o644)
}

// writePNG encodes a single image as a standalone PNG (used for Windows
// Terminal tab icons, which render better from PNG than from .ico).
func writePNG(path string, img image.Image) error {
	if dir := filepath.Dir(path); dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return err
	}
	return os.WriteFile(path, buf.Bytes(), 0o644)
}

// dimByte encodes an icon dimension for the ICO directory, where 256 is stored
// as 0.
func dimByte(v int) byte {
	if v >= 256 {
		return 0
	}
	return byte(v)
}

// writeSyso emits one resource_windows_<arch>.syso per requested arch. The Go
// linker automatically embeds the matching file when building for that
// GOOS/GOARCH, and ignores it otherwise, so the icon ships in the binary
// without any build-tag plumbing.
func writeSyso(dir, icoPath, manifestPath, name, desc, version string, archs []string) error {
	major, minor, patch, build := parseVersion(version)

	vi := &goversioninfo.VersionInfo{}
	vi.IconPath = icoPath
	vi.ManifestPath = manifestPath
	vi.FixedFileInfo.FileVersion = goversioninfo.FileVersion{
		Major: major,
		Minor: minor,
		Patch: patch,
		Build: build,
	}
	vi.FixedFileInfo.ProductVersion = goversioninfo.FileVersion{
		Major: major,
		Minor: minor,
		Patch: patch,
		Build: build,
	}
	vi.ProductName = name
	vi.FileDescription = desc
	vi.StringFileInfo.FileVersion = version
	vi.StringFileInfo.ProductVersion = version

	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	for _, arch := range archs {
		vi.Build()
		vi.Walk()
		out := filepath.Join(dir, "resource_windows_"+arch+".syso")
		if err := vi.WriteSyso(out, arch); err != nil {
			return fmt.Errorf("%s: %w", arch, err)
		}
		log.Printf("genicon: wrote %s", out)
	}
	return nil
}

// parseVersion turns a dotted version string into the four 16-bit fields the
// Windows resource expects. Missing components default to 0.
func parseVersion(v string) (major, minor, patch, build int) {
	parts := strings.Split(strings.TrimSpace(v), ".")
	get := func(i int) int {
		if i >= len(parts) {
			return 0
		}
		n, err := strconv.Atoi(strings.TrimSpace(parts[i]))
		if err != nil {
			return 0
		}
		return n
	}
	return get(0), get(1), get(2), get(3)
}

func parseInts(csv string) ([]int, error) {
	var out []int
	for _, f := range splitCSV(csv) {
		n, err := strconv.Atoi(f)
		if err != nil {
			return nil, fmt.Errorf("%q is not an integer", f)
		}
		out = append(out, n)
	}
	return out, nil
}

func splitCSV(csv string) []string {
	var out []string
	for f := range strings.SplitSeq(csv, ",") {
		if f = strings.TrimSpace(f); f != "" {
			out = append(out, f)
		}
	}
	return out
}
