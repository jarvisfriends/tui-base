// Command releasetag turns gorelease's API-compatibility analysis into a
// release decision for this module.
//
// It runs gorelease (golang.org/x/exp/cmd/gorelease), which compares the
// current source against the latest released tag and suggests the next semantic
// version. From the suggested-vs-current delta it classifies the change:
//
//   - patch — no exported API changed; in apply mode this is tagged
//     automatically.
//   - minor — new exported API was added, or (for a pre-1.0 v0.x module) a
//     backward-incompatible change. Never auto-tagged; a maintainer approves.
//   - major — a backward-incompatible change to a stable (v1+) module. Never
//     auto-tagged; a maintainer approves.
//
// Modes:
//
//   - plan  (pull requests): post/update a PR comment and a job summary saying
//     what will happen on merge. Nothing is tagged.
//   - apply (push to the default branch): create the patch tag automatically;
//     for minor/major, emit a summary explaining why human approval is required
//     and stop without tagging.
//
// The tool is stdlib-only and orchestrates gorelease and git as subprocesses so
// the release logic lives in Go rather than in shell.
package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// commentMarker identifies this tool's PR comment so it is updated in place
// rather than duplicated on every run.
const commentMarker = "<!-- releasetag -->"

// Bump kinds, in increasing severity.
const (
	bumpNone    = "none"
	bumpPatch   = "patch"
	bumpMinor   = "minor"
	bumpMajor   = "major"
	bumpInitial = "initial"
	bumpUnknown = "unknown"
)

func main() {
	mode := flag.String("mode", "plan", `"plan" (report only) or "apply" (create the patch tag)`)
	flag.Parse()
	if err := run(*mode); err != nil {
		fmt.Fprintln(os.Stderr, "releasetag:", err)
		os.Exit(1)
	}
}

func run(mode string) error {
	report, gorErr := runGorelease()
	a := analyze(report)

	writeSummary(a.markdown())

	switch mode {
	case "plan":
		// A preview never blocks the PR: even when gorelease could not produce a
		// suggestion, surface what happened in the comment and job summary.
		upsertPRComment(a.markdown())
		fmt.Printf("plan: %s %s -> %s\n", a.bump, a.base, a.next)
		return nil
	case "apply":
		if a.bump == bumpUnknown {
			return fmt.Errorf("gorelease did not yield a version to release (err=%w):\n%s", gorErr, report)
		}
		return a.apply()
	default:
		return fmt.Errorf("unknown -mode %q (want plan or apply)", mode)
	}
}

// ─── analysis ─────────────────────────────────────────────────────────────────

type analysis struct {
	base   string // latest released tag, e.g. v0.1.12
	next   string // gorelease's suggested next version
	bump   string // one of the bump* constants
	reason string // human-readable justification
	report string // full gorelease output
}

type ver struct{ major, minor, patch int }

var (
	reBase = regexp.MustCompile(`(?m)^Inferred base version:\s*(\S+)`)
	reNext = regexp.MustCompile(`(?m)^Suggested version:\s*(\S+)`)
)

// analyze classifies the release from gorelease's report.
func analyze(report string) analysis {
	a := analysis{report: report}
	if m := reBase.FindStringSubmatch(report); m != nil {
		a.base = m[1]
	} else {
		a.base = latestTag()
	}
	if m := reNext.FindStringSubmatch(report); m != nil {
		a.next = m[1]
	}

	if a.base == "" {
		a.bump = bumpInitial
		if a.next == "" {
			a.next = "v0.1.0"
		}
		a.reason = "No previous release tag was found. The first version has to be chosen and tagged by a maintainer."
		return a
	}

	bv, okB := parseVer(a.base)
	nv, okN := parseVer(a.next)
	if !okB || !okN {
		a.bump = bumpUnknown
		a.reason = "Could not parse the version numbers reported by gorelease; a maintainer should review the release manually."
		return a
	}

	a.bump = classify(bv, nv)
	a.reason = reasonFor(a.bump, a.base, report)
	return a
}

// classify reports the semver component that changed between base and next.
func classify(base, next ver) string {
	switch {
	case next.major > base.major:
		return bumpMajor
	case next.major == base.major && next.minor > base.minor:
		return bumpMinor
	case next.major == base.major && next.minor == base.minor && next.patch > base.patch:
		return bumpPatch
	default:
		return bumpNone
	}
}

// reasonFor explains, in one sentence, why a given bump is required. The "why"
// for minor/major is drawn from whether gorelease found incompatible changes.
func reasonFor(bump, base, report string) string {
	breaking := strings.Contains(strings.ToLower(report), "incompatible changes")
	switch bump {
	case bumpNone:
		return "No changes since " + base + "; nothing to release."
	case bumpPatch:
		return "No exported API changed since " + base + " — only internal changes. This is a backward-compatible patch, so it is tagged automatically."
	case bumpMinor:
		if breaking {
			return "This module is pre-1.0 (v0.x) and makes backward-incompatible API changes. Under semver, breaking changes to a 0.x module bump the minor version — a maintainer must approve it."
		}
		return "New exported API was added (backward-compatible). Adding API bumps the minor version — a maintainer must approve it."
	case bumpMajor:
		return "This makes backward-incompatible API changes to a stable (v1+) module, which requires a new major version — a maintainer must approve it."
	default:
		return "The next version could not be determined automatically; a maintainer should review the release."
	}
}

// ─── reporting ────────────────────────────────────────────────────────────────

func (a analysis) headline() string {
	switch a.bump {
	case bumpPatch:
		return fmt.Sprintf("✅ Auto-patch release: **%s → %s**", a.base, a.next)
	case bumpMinor:
		return fmt.Sprintf("⚠️ Minor bump required — human approval needed: **%s → %s**", a.base, a.next)
	case bumpMajor:
		return fmt.Sprintf("⛔ Major bump required — human approval needed: **%s → %s**", a.base, a.next)
	case bumpNone:
		return "ℹ️ No release needed"
	case bumpInitial:
		return "🔖 Initial release — human approval needed"
	default:
		return "❓ Release needs manual review"
	}
}

// markdown renders the report shown in the PR comment and the job summary.
func (a analysis) markdown() string {
	var b strings.Builder
	fmt.Fprintln(&b, commentMarker)
	fmt.Fprintln(&b, "### Release check")
	fmt.Fprintln(&b)
	fmt.Fprintln(&b, a.headline())
	fmt.Fprintln(&b)
	fmt.Fprintln(&b, a.reason)
	fmt.Fprintln(&b)
	switch a.bump {
	case bumpPatch:
		fmt.Fprintf(&b, "On merge to the default branch, `%s` is created automatically.\n", a.next)
	case bumpMinor, bumpMajor, bumpInitial:
		fmt.Fprintf(&b, "Automatic tagging is **blocked**. A maintainer approves this release by pushing the tag:\n\n```sh\ngit tag %s\ngit push origin %s\n```\n", a.next, a.next)
	}
	if r := strings.TrimSpace(a.report); r != "" {
		fmt.Fprintf(&b, "\n<details><summary>gorelease report</summary>\n\n```\n%s\n```\n</details>\n", r)
	}
	return b.String()
}

// apply performs the merge-time action: tag a patch, or surface a blocked
// minor/major without tagging.
func (a analysis) apply() error {
	switch a.bump {
	case bumpPatch:
		if tagExists(a.next) {
			fmt.Printf("tag %s already exists; nothing to do\n", a.next)
			return nil
		}
		fmt.Printf("API is compatible — creating patch tag %s\n", a.next)
		return createAndPushTag(a.next)
	case bumpNone:
		fmt.Println("no changes to release")
		return nil
	default:
		// minor / major / initial / unknown: never auto-tag. Emit a workflow
		// warning annotation (visible in the run) plus the summary already
		// written, and exit successfully — a pending release is not a failure.
		fmt.Printf("::warning title=Release approval needed::%s bump %s -> %s: %s\n", a.bump, a.base, a.next, a.reason)
		return nil
	}
}

// ─── external commands ────────────────────────────────────────────────────────

// runGorelease runs gorelease in the current module and returns its combined
// output. A non-zero exit (which gorelease returns for major/incompatible
// suggestions) is not treated as fatal here; analyze() reads the report.
func runGorelease() (string, error) {
	var buf bytes.Buffer
	cmd := exec.Command("gorelease")
	cmd.Stdout = &buf
	cmd.Stderr = &buf
	err := cmd.Run()
	return buf.String(), err
}

// latestTag returns the highest vX.Y.Z tag, used as the base when gorelease
// does not report an inferred base version.
func latestTag() string {
	out, err := exec.Command("git", "tag", "--list", "v*", "--sort=-v:refname").Output()
	if err != nil {
		return ""
	}
	sc := bufio.NewScanner(bytes.NewReader(out))
	for sc.Scan() {
		if t := strings.TrimSpace(sc.Text()); t != "" {
			return t
		}
	}
	return ""
}

func tagExists(tag string) bool {
	return exec.Command("git", "rev-parse", "-q", "--verify", "refs/tags/"+tag).Run() == nil
}

func createAndPushTag(tag string) error {
	if out, err := exec.Command("git", "tag", tag).CombinedOutput(); err != nil {
		return fmt.Errorf("git tag %s: %w: %s", tag, err, out)
	}
	if out, err := exec.Command("git", "push", "origin", tag).CombinedOutput(); err != nil {
		return fmt.Errorf("git push %s: %w: %s", tag, err, out)
	}
	fmt.Printf("pushed tag %s\n", tag)
	return nil
}

// parseVer parses "v1.2.3" (ignoring any pre-release/build suffix).
func parseVer(s string) (ver, bool) {
	s = strings.TrimPrefix(strings.TrimSpace(s), "v")
	if i := strings.IndexAny(s, "-+"); i >= 0 {
		s = s[:i]
	}
	parts := strings.Split(s, ".")
	if len(parts) != 3 {
		return ver{}, false
	}
	var (
		v   ver
		err error
	)
	if v.major, err = strconv.Atoi(parts[0]); err != nil {
		return ver{}, false
	}
	if v.minor, err = strconv.Atoi(parts[1]); err != nil {
		return ver{}, false
	}
	if v.patch, err = strconv.Atoi(parts[2]); err != nil {
		return ver{}, false
	}
	return v, true
}

// ─── GitHub surfaces ──────────────────────────────────────────────────────────

// writeSummary appends the report to the job's step summary (or stdout when run
// outside Actions).
func writeSummary(md string) {
	path := os.Getenv("GITHUB_STEP_SUMMARY")
	if path == "" {
		fmt.Println(md)
		return
	}
	//nolint:gosec // GITHUB_STEP_SUMMARY is a GitHub Actions-provided output path.
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0o644)
	if err != nil {
		fmt.Println(md)
		return
	}
	defer func() { _ = f.Close() }()
	_, _ = fmt.Fprintln(f, md)
}

// upsertPRComment posts (or updates) the report as a single PR comment. It is
// best-effort: without a token or PR context — e.g. a fork PR, or the apply run
// on push — the job summary already carries the same report.
func upsertPRComment(md string) {
	token := os.Getenv("GITHUB_TOKEN")
	repo := os.Getenv("GITHUB_REPOSITORY")
	pr := prNumber()
	if token == "" || repo == "" || pr == 0 {
		return
	}
	api := os.Getenv("GITHUB_API_URL")
	if api == "" {
		api = "https://api.github.com"
	}
	list := fmt.Sprintf("%s/repos/%s/issues/%d/comments", api, repo, pr)
	body, _ := json.Marshal(map[string]string{"body": md})
	if id := findComment(list, token); id != 0 {
		ghDo("PATCH", fmt.Sprintf("%s/repos/%s/issues/comments/%d", api, repo, id), token, body)
		return
	}
	ghDo("POST", list, token, body)
}

// prNumber reads the pull-request number from the Actions event payload.
func prNumber() int {
	p := os.Getenv("GITHUB_EVENT_PATH")
	if p == "" {
		return 0
	}
	//nolint:gosec // GITHUB_EVENT_PATH is a GitHub Actions-provided JSON payload path.
	data, err := os.ReadFile(p)
	if err != nil {
		return 0
	}
	var ev struct {
		Number      int `json:"number"`
		PullRequest struct {
			Number int `json:"number"`
		} `json:"pull_request"`
	}
	if json.Unmarshal(data, &ev) != nil {
		return 0
	}
	if ev.PullRequest.Number != 0 {
		return ev.PullRequest.Number
	}
	return ev.Number
}

func findComment(listURL, token string) int64 {
	out, code := ghDo("GET", listURL+"?per_page=100", token, nil)
	if code != http.StatusOK {
		return 0
	}
	var comments []struct {
		ID   int64  `json:"id"`
		Body string `json:"body"`
	}
	if json.Unmarshal(out, &comments) != nil {
		return 0
	}
	for _, c := range comments {
		if strings.Contains(c.Body, commentMarker) {
			return c.ID
		}
	}
	return 0
}

// ghDo performs a GitHub REST call and returns the body and status code. Any
// transport error yields status 0, which callers treat as "skip".
func ghDo(method, url, token string, body []byte) (responseBody []byte, statusCode int) {
	var r io.Reader
	if body != nil {
		r = bytes.NewReader(body)
	}
	//nolint:gosec // The URL is built from GitHub Actions environment values for the GitHub API endpoint.
	req, err := http.NewRequest(method, url, r)
	if err != nil {
		return nil, 0
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	client := &http.Client{Timeout: 15 * time.Second}
	//nolint:gosec // GitHub API calls are limited to the trusted API host resolved above.
	resp, err := client.Do(req)
	if err != nil {
		return nil, 0
	}
	defer func() { _ = resp.Body.Close() }()
	out, _ := io.ReadAll(resp.Body)
	return out, resp.StatusCode
}
