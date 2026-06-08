#!/usr/bin/env bash
set -euo pipefail

mkdir -p profiles

echo "Running benchmarks per-package and writing CPU/memory profiles to profiles/"

# Iterate packages and run benchmarks per-package so cpuprofile/memprofile flags are valid
for pkg in $(go list ./...); do
	pkgname=${pkg//[^a-zA-Z0-9]/_}
	echo "Benchmarking package $pkg ..."
	# Run benchmarks for the single package and write per-package profiles. Ignore failures
	go test -run '^$' -bench . -benchmem "$pkg" -cpuprofile "profiles/cpu-${pkgname}.prof" -memprofile "profiles/mem-${pkgname}.prof" -count=1 || true
done

echo "Profiles written into profiles/ (one per package)."
echo "To view a CPU profile interactively: go tool pprof -http=:8080 profiles/cpu_<pkg>.prof"
