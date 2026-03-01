package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"
)

const (
	releaseBaseLatest = "https://github.com/ercadev/dirigent-releases/releases/latest/download"
	releaseBaseTagFmt = "https://github.com/ercadev/dirigent-releases/releases/download/%s"
)

type versionSnapshot struct {
	CurrentVersion string `json:"currentVersion"`
	LatestVersion  string `json:"latestVersion"`
}

type versionLookup func() (versionSnapshot, error)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	var err error
	switch os.Args[1] {
	case "setup":
		err = runSetup(os.Args[2:])
	case "upgrade":
		err = runUpgrade(os.Args[2:])
	case "doctor":
		err = runDoctor(os.Args[2:])
	case "-h", "--help", "help":
		printUsage()
		return
	default:
		err = fmt.Errorf("unknown command %q", os.Args[1])
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println("Lotsen CLI")
	fmt.Println("")
	fmt.Println("Usage:")
	fmt.Println("  lotsen setup [flags]")
	fmt.Println("  lotsen upgrade [flags]")
	fmt.Println("  lotsen doctor [flags]")
	fmt.Println("")
	fmt.Println("Run `lotsen <command> --help` for command-specific options.")
}

func runSetup(args []string) error {
	fs := flag.NewFlagSet("setup", flag.ContinueOnError)
	fs.SetOutput(io.Discard)

	interactive := fs.Bool("interactive", false, "Force interactive setup")
	nonInteractive := fs.Bool("non-interactive", false, "Disable prompts")
	yes := fs.Bool("yes", false, "Skip confirmation prompts")
	profile := fs.String("profile", "", "Security profile: strict, standard, off")
	proxyHardeningProfile := fs.String("proxy-hardening-profile", "", "Proxy hardening profile: strict, standard, off")
	version := fs.String("version", "latest", "Lotsen version to install")
	dashboardExpose := fs.Bool("dashboard-expose", false, "Expose dashboard via proxy")
	dashboardDomain := fs.String("dashboard-domain", "", "Dashboard domain")
	dashboardUser := fs.String("dashboard-user", "", "Dashboard basic auth username")
	dashboardPasswordStdin := fs.Bool("dashboard-password-stdin", false, "Read dashboard password from stdin")

	if err := fs.Parse(args); err != nil {
		return setupUsage(err)
	}

	isTTY := stdinIsTTY()
	if *interactive && !isTTY {
		return errors.New("--interactive requires a TTY; run this directly in a terminal")
	}

	if !*interactive && !*nonInteractive && isTTY {
		*interactive = true
	}

	selectedProfile := strings.TrimSpace(*profile)
	if selectedProfile == "" {
		if *interactive {
			selectedProfile = promptProfileWithStrictDefault()
		} else {
			selectedProfile = "standard"
		}
	}

	switch selectedProfile {
	case "strict", "standard", "off":
	default:
		return fmt.Errorf("invalid --profile %q (expected strict, standard, or off)", selectedProfile)
	}

	selectedProxyHardeningProfile := strings.TrimSpace(*proxyHardeningProfile)
	if selectedProxyHardeningProfile == "" {
		selectedProxyHardeningProfile = strings.TrimSpace(os.Getenv("LOTSEN_PROXY_HARDENING_PROFILE"))
	}
	if selectedProxyHardeningProfile == "" {
		selectedProxyHardeningProfile = selectedProfile
	}

	switch selectedProxyHardeningProfile {
	case "strict", "standard", "off":
	default:
		return fmt.Errorf("invalid --proxy-hardening-profile %q (expected strict, standard, or off)", selectedProxyHardeningProfile)
	}

	dashboardPassword := ""
	if *dashboardPasswordStdin {
		data, err := io.ReadAll(os.Stdin)
		if err != nil {
			return fmt.Errorf("read password from stdin: %w", err)
		}
		dashboardPassword = strings.TrimSpace(string(data))
	}

	if *dashboardExpose {
		if strings.TrimSpace(*dashboardDomain) == "" {
			return errors.New("--dashboard-expose requires --dashboard-domain")
		}
		if strings.TrimSpace(*dashboardUser) == "" {
			return errors.New("--dashboard-expose requires --dashboard-user")
		}
		if !*dashboardPasswordStdin && !*interactive {
			return errors.New("--dashboard-expose in non-interactive mode requires --dashboard-password-stdin")
		}
	}

	if *interactive && !*yes {
		confirmed, err := promptYesNo("Apply setup now? [Y/n]: ", true)
		if err != nil {
			return err
		}
		if !confirmed {
			return errors.New("setup cancelled")
		}
	}

	env := append(os.Environ(),
		"LOTSEN_VERSION="+*version,
		"LOTSEN_SECURITY_PROFILE="+selectedProfile,
		"LOTSEN_PROXY_HARDENING_PROFILE="+selectedProxyHardeningProfile,
	)
	if *nonInteractive {
		env = append(env, "LOTSEN_NON_INTERACTIVE=1")
	}
	if *dashboardExpose {
		env = append(env,
			"LOTSEN_DASHBOARD_DOMAIN="+strings.TrimSpace(*dashboardDomain),
			"LOTSEN_DASHBOARD_USER="+strings.TrimSpace(*dashboardUser),
			"LOTSEN_DASHBOARD_PASSWORD="+dashboardPassword,
		)
	}

	url := releaseScriptURL(*version, "setup.sh")
	fmt.Printf("--> Running setup from %s\n", url)
	return runRemoteScript(url, env)
}

func setupUsage(parseErr error) error {
	b := &strings.Builder{}
	fmt.Fprintln(b, parseErr)
	fmt.Fprintln(b, "")
	fmt.Fprintln(b, "Usage: lotsen setup [flags]")
	fmt.Fprintln(b, "")
	fmt.Fprintln(b, "Flags:")
	fmt.Fprintln(b, "  --interactive             Force interactive setup")
	fmt.Fprintln(b, "  --non-interactive         Disable prompts")
	fmt.Fprintln(b, "  --yes                     Skip confirmation prompts")
	fmt.Fprintln(b, "  --profile <name>          Security profile: strict, standard, off")
	fmt.Fprintln(b, "  --proxy-hardening-profile <name> Proxy hardening profile: strict, standard, off")
	fmt.Fprintln(b, "  --version <value>         Version to install (default: latest)")
	fmt.Fprintln(b, "  --dashboard-expose        Configure dashboard domain and basic auth")
	fmt.Fprintln(b, "  --dashboard-domain <fqdn> Dashboard domain")
	fmt.Fprintln(b, "  --dashboard-user <name>   Dashboard basic auth username")
	fmt.Fprintln(b, "  --dashboard-password-stdin Read dashboard password from stdin")
	return errors.New(strings.TrimSpace(b.String()))
}

func runUpgrade(args []string) error {
	fs := flag.NewFlagSet("upgrade", flag.ContinueOnError)
	fs.SetOutput(io.Discard)

	target := fs.String("to", "latest", "Upgrade target version")
	nonInteractive := fs.Bool("non-interactive", false, "Disable prompts")
	yes := fs.Bool("yes", false, "Skip confirmation prompts")

	if err := fs.Parse(args); err != nil {
		return fmt.Errorf("%w\n\nUsage: lotsen upgrade [--to latest|vX.Y.Z] [--non-interactive] [--yes]", err)
	}

	currentVersion, targetVersion := determineUpgradeVersions(*target, fetchLocalVersionSnapshot)
	fmt.Printf("--> Upgrading Lotsen from %s to %s\n", currentVersion, targetVersion)

	effectiveNonInteractive := *nonInteractive || !stdinIsTTY()
	if !effectiveNonInteractive && !*yes {
		confirmed, err := promptYesNo("Proceed with upgrade? [Y/n]: ", true)
		if err != nil {
			return err
		}
		if !confirmed {
			return errors.New("upgrade cancelled")
		}
	}

	env := append(os.Environ(),
		"LOTSEN_VERSION="+*target,
		"LOTSEN_UPGRADE=1",
	)
	if effectiveNonInteractive {
		env = append(env, "LOTSEN_NON_INTERACTIVE=1")
	}
	if *yes {
		env = append(env, "LOTSEN_YES=1")
	}

	url := releaseScriptURL(*target, "setup.sh")
	fmt.Printf("--> Running upgrade from %s\n", url)
	return runRemoteScript(url, env)
}

func determineUpgradeVersions(target string, lookup versionLookup) (string, string) {
	from := "unknown"
	to := strings.TrimSpace(target)
	if to == "" {
		to = "latest"
	}

	if lookup == nil {
		return from, to
	}

	snapshot, err := lookup()
	if err != nil {
		return from, to
	}

	if strings.TrimSpace(snapshot.CurrentVersion) != "" {
		from = strings.TrimSpace(snapshot.CurrentVersion)
	}
	if to == "latest" && strings.TrimSpace(snapshot.LatestVersion) != "" {
		to = strings.TrimSpace(snapshot.LatestVersion)
	}

	return from, to
}

func fetchLocalVersionSnapshot() (versionSnapshot, error) {
	client := &http.Client{Timeout: 3 * time.Second}
	req, err := http.NewRequest(http.MethodGet, "http://127.0.0.1:8080/api/version", nil)
	if err != nil {
		return versionSnapshot{}, err
	}

	resp, err := client.Do(req)
	if err != nil {
		return versionSnapshot{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return versionSnapshot{}, fmt.Errorf("version endpoint status: %d", resp.StatusCode)
	}

	var snapshot versionSnapshot
	if err := json.NewDecoder(resp.Body).Decode(&snapshot); err != nil {
		return versionSnapshot{}, err
	}

	return snapshot, nil
}

func runDoctor(args []string) error {
	fs := flag.NewFlagSet("doctor", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	jsonOutput := fs.Bool("json", false, "Print JSON output")
	if err := fs.Parse(args); err != nil {
		return fmt.Errorf("%w\n\nUsage: lotsen doctor [--json]", err)
	}

	status := map[string]string{
		"binary": "ok",
	}
	if _, err := os.Stat("/usr/local/bin/lotsen-api"); err != nil {
		status["api_binary"] = "missing"
	} else {
		status["api_binary"] = "ok"
	}

	if *jsonOutput {
		fmt.Printf("{\"binary\":%q,\"api_binary\":%q}\n", status["binary"], status["api_binary"])
		if status["api_binary"] == "ok" {
			return nil
		}
		return errors.New("doctor found issues")
	}

	fmt.Println("Lotsen doctor")
	fmt.Printf("- CLI binary: %s\n", status["binary"])
	fmt.Printf("- API binary: %s\n", status["api_binary"])
	if status["api_binary"] != "ok" {
		return errors.New("doctor found issues")
	}
	return nil
}

func stdinIsTTY() bool {
	info, err := os.Stdin.Stat()
	if err != nil {
		return false
	}
	return (info.Mode() & os.ModeCharDevice) != 0
}

func promptProfileWithStrictDefault() string {
	fmt.Println("Security profile")
	fmt.Println("  1) strict (recommended)")
	fmt.Println("  2) standard")
	fmt.Println("  3) off")
	choice, err := promptLine("Choose profile [1]: ")
	if err != nil {
		return "strict"
	}
	switch strings.TrimSpace(choice) {
	case "", "1", "strict":
		return "strict"
	case "2", "standard":
		return "standard"
	case "3", "off":
		return "off"
	default:
		return "strict"
	}
}

func promptYesNo(prompt string, defaultYes bool) (bool, error) {
	line, err := promptLine(prompt)
	if err != nil {
		return false, err
	}
	v := strings.ToLower(strings.TrimSpace(line))
	if v == "" {
		return defaultYes, nil
	}
	if v == "y" || v == "yes" {
		return true, nil
	}
	if v == "n" || v == "no" {
		return false, nil
	}
	return defaultYes, nil
}

func promptLine(prompt string) (string, error) {
	tty, err := os.OpenFile("/dev/tty", os.O_RDWR, 0)
	if err != nil {
		return "", fmt.Errorf("open /dev/tty: %w", err)
	}
	defer tty.Close()

	if _, err := fmt.Fprint(tty, prompt); err != nil {
		return "", err
	}
	reader := bufio.NewReader(tty)
	line, err := reader.ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return "", err
	}
	return strings.TrimRight(line, "\r\n"), nil
}

func releaseScriptURL(version string, name string) string {
	version = strings.TrimSpace(version)
	if version == "" || version == "latest" {
		return releaseBaseLatest + "/" + name
	}
	return fmt.Sprintf(releaseBaseTagFmt, version) + "/" + name
}

func runRemoteScript(scriptURL string, env []string) error {
	resp, err := http.Get(scriptURL)
	if err != nil {
		return fmt.Errorf("download script: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download script status: %d", resp.StatusCode)
	}

	tmp, err := os.CreateTemp("", "lotsen-setup-*.sh")
	if err != nil {
		return fmt.Errorf("create temp script: %w", err)
	}

	if _, err := io.Copy(tmp, resp.Body); err != nil {
		tmp.Close()
		_ = os.Remove(tmp.Name())
		return fmt.Errorf("write temp script: %w", err)
	}
	if err := tmp.Chmod(0o700); err != nil {
		tmp.Close()
		_ = os.Remove(tmp.Name())
		return fmt.Errorf("chmod temp script: %w", err)
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmp.Name())
		return fmt.Errorf("close temp script: %w", err)
	}
	defer os.Remove(tmp.Name())

	cmd := exec.Command("bash", tmp.Name())
	cmd.Env = append(env, "LOTSEN_SETUP_STARTED_AT="+time.Now().UTC().Format(time.RFC3339))
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("run setup script: %w", err)
	}
	return nil
}
