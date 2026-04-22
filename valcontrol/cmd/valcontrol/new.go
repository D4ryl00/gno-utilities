package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

func runNew(args []string) error {
	fs := flag.NewFlagSet("new", flag.ContinueOnError)
	scenarioLib := fs.String("scenario-lib", defaultScenarioLibPath(), "path to val-scenarios/lib/scenario.sh")
	name := fs.String("name", "", "scenario name (default: valcontrol-<N>-validators)")
	controllableSigner := fs.Bool("controllable-signer", true, "add controllable signer to each validator")
	fs.SetOutput(os.Stderr)
	if err := fs.Parse(args); err != nil {
		return err
	}

	rest := fs.Args()
	if len(rest) != 1 {
		return errors.New("usage: valcontrol new [--scenario-lib <path>] [--name <name>] [--controllable-signer] <count>")
	}

	count, err := strconv.Atoi(rest[0])
	if err != nil || count < 1 {
		return fmt.Errorf("count must be a positive integer, got %q", rest[0])
	}

	libPath, err := resolveScenarioLib(*scenarioLib)
	if err != nil {
		return err
	}

	scenarioName := *name
	if scenarioName == "" {
		scenarioName = fmt.Sprintf("valcontrol-%d-validators", count)
	}

	script := buildBootstrapScript(libPath, scenarioName, count, *controllableSigner)

	cmd := exec.Command("bash", "-s")
	cmd.Stdin = strings.NewReader(script)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("bootstrap failed: %w", err)
	}

	inventoryPath := computeInventoryPath(scenarioName)
	fmt.Printf("\ninventory: %s\n", inventoryPath)
	fmt.Printf("run: export INVENTORY_PATH=%s\n", inventoryPath)

	return nil
}

func buildBootstrapScript(libPath, scenarioName string, count int, controllableSigner bool) string {
	var b strings.Builder
	fmt.Fprintln(&b, "#!/usr/bin/env bash")
	fmt.Fprintln(&b, "set -euo pipefail")
	fmt.Fprintln(&b, `LOG_LEVEL="${LOG_LEVEL:-debug}"`)
	fmt.Fprintf(&b, "source %q\n", libPath)
	fmt.Fprintf(&b, "scenario_init %q\n", scenarioName)
	for i := 1; i <= count; i++ {
		if controllableSigner {
			fmt.Fprintf(&b, "gen_validator val%d --controllable-signer\n", i)
		} else {
			fmt.Fprintf(&b, "gen_validator val%d\n", i)
		}
	}
	fmt.Fprintln(&b, "prepare_network")
	fmt.Fprintln(&b, "start_all_nodes")
	return b.String()
}

func computeInventoryPath(scenarioName string) string {
	workRoot := os.Getenv("WORK_ROOT")
	if workRoot == "" {
		workRoot = "/tmp/gno-val-tests"
	}
	return filepath.Join(workRoot, slugifyName(scenarioName), "inventory.json")
}

// slugifyName mirrors scenario.sh's slugify():
// lowercase, replace sequences of non-[a-z0-9] chars with a single dash.
func slugifyName(s string) string {
	s = strings.ToLower(s)
	var b strings.Builder
	inSep := false
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
			inSep = false
		} else {
			if !inSep {
				b.WriteRune('-')
				inSep = true
			}
		}
	}
	return strings.Trim(b.String(), "-")
}

// runBashScript writes script to a temp file and executes it, returning combined output.
func runBashScript(script string) ([]byte, error) {
	f, err := os.CreateTemp("", "valcontrol-*.sh")
	if err != nil {
		return nil, err
	}
	defer os.Remove(f.Name())
	if _, err := f.WriteString(script); err != nil {
		f.Close()
		return nil, err
	}
	f.Close()
	return exec.Command("bash", f.Name()).CombinedOutput()
}

func resolveScenarioLib(path string) (string, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("resolve scenario-lib: %w", err)
	}
	if _, err := os.Stat(abs); err != nil {
		return "", fmt.Errorf("scenario-lib not found at %q: use --scenario-lib to set the path", abs)
	}
	return abs, nil
}

func defaultScenarioLibPath() string {
	return "../../gno/misc/val-scenarios/lib/scenario.sh"
}
