package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/remi/gno-utilities/valcontrol/pkg/valcontrol"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) == 0 {
		return runTUI(nil)
	}

	switch args[0] {
	case "tui":
		return runTUI(args[1:])
	case "new":
		return runNew(args[1:])
	case "list":
		return runList(args[1:])
	case "watch":
		return runWatch(args[1:])
	case "state":
		return runState(args[1:])
	case "drop":
		return runDrop(args[1:])
	case "delay":
		return runDelay(args[1:])
	case "clear":
		return runClear(args[1:])
	case "reset":
		return runReset(args[1:])
	case "help", "-h", "--help":
		usage()
		return nil
	default:
		return fmt.Errorf("unknown subcommand %q", args[0])
	}
}

func usage() {
	fmt.Println(`valcontrol [subcommand] [flags]

Subcommands:
  new <count>         Bootstrap a new chain with <count> validators
  tui                 Launch the interactive TUI (default)
  list                List validators and current status
  watch               Continuously refresh validator status table
  state <validator>   Show full JSON state for one validator
  drop <validator> <phase> [--height N] [--round N]
  delay <validator> <phase> <duration> [--height N] [--round N]
  clear <validator> [phase]
  reset <validator>           Clean reset (wipes db, wal, validator state, restores genesis)
  reset --safe <validator>    Safe reset (wipes only db and wal, preserves validator state)

Flags for new:
  --scenario-lib <path>   Path to val-scenarios/lib/scenario.sh
                          (default: ../../gno/misc/val-scenarios/lib/scenario.sh)
  --name <name>           Scenario name (default: valcontrol-<N>-validators)
  --controllable-signer   Attach a controllable signer sidecar to each validator

Flags shared by most commands:
  --inventory <path>  Path to inventory.json
  --timeout <dur>     HTTP timeout (default 5s)

Environment:
  INVENTORY_PATH       Path to inventory.json (overridden by --inventory)
  VALCONTROL_INVENTORY  Deprecated fallback for inventory.json
  WORK_ROOT             Root directory for scenario data (default: /tmp/gno-val-tests)`)
}

func runList(args []string) error {
	inv, client, _, err := common(args)
	if err != nil {
		return err
	}

	fmt.Printf("scenario: %s\n", inv.Scenario)
	fmt.Printf("inventory: %s\n\n", resolvedInventoryPath(args))
	fmt.Printf("%-10s %-8s %-12s %-10s %-12s %s\n", "VALIDATOR", "HEIGHT", "CATCHINGUP", "CONTROL", "MONIKER", "RULES")
	for _, v := range inv.Validators {
		snap := client.Snapshot(v)
		height := "error"
		catchingUp := "-"
		moniker := "-"
		if snap.RPC != nil {
			height = snap.RPC.Result.SyncInfo.LatestBlockHeight
			catchingUp = fmt.Sprintf("%v", snap.RPC.Result.SyncInfo.CatchingUp)
			moniker = snap.RPC.Result.NodeInfo.Moniker
		}
		control := "no"
		if v.ControlURL != nil {
			control = "yes"
		}
		rules := valcontrol.FormatRules(snap.Signer)
		fmt.Printf("%-10s %-8s %-12s %-10s %-12s %s\n", v.Name, height, catchingUp, control, moniker, rules)
	}

	return nil
}

func runWatch(args []string) error {
	fs := flag.NewFlagSet("watch", flag.ContinueOnError)
	invPath := fs.String("inventory", defaultInventoryPath(), "inventory path")
	timeout := fs.Duration("timeout", 5*time.Second, "HTTP timeout")
	interval := fs.Duration("interval", 2*time.Second, "refresh interval")
	fs.SetOutput(os.Stderr)
	if err := fs.Parse(args); err != nil {
		return err
	}

	inv, err := valcontrol.LoadInventory(*invPath)
	if err != nil {
		return err
	}
	client := valcontrol.NewClient(*timeout)

	for {
		fmt.Print("\033[2J\033[H")
		fmt.Printf("scenario: %s\n", inv.Scenario)
		fmt.Printf("inventory: %s\n", *invPath)
		fmt.Printf("time: %s\n\n", time.Now().Format(time.RFC3339))
		fmt.Printf("%-10s %-8s %-12s %-10s %-12s %s\n", "VALIDATOR", "HEIGHT", "CATCHINGUP", "CONTROL", "MONIKER", "RULES")
		for _, v := range inv.Validators {
			snap := client.Snapshot(v)
			height := "error"
			catchingUp := "-"
			moniker := "-"
			if snap.RPC != nil {
				height = snap.RPC.Result.SyncInfo.LatestBlockHeight
				catchingUp = fmt.Sprintf("%v", snap.RPC.Result.SyncInfo.CatchingUp)
				moniker = snap.RPC.Result.NodeInfo.Moniker
			}
			control := "no"
			if v.ControlURL != nil {
				control = "yes"
			}
			fmt.Printf("%-10s %-8s %-12s %-10s %-12s %s\n", v.Name, height, catchingUp, control, moniker, valcontrol.FormatRules(snap.Signer))
		}
		time.Sleep(*interval)
	}
}

func runState(args []string) error {
	inv, client, rest, err := common(args)
	if err != nil {
		return err
	}
	if len(rest) != 1 {
		return errors.New("usage: valcontrol state [--inventory path] <validator>")
	}

	validator, err := inv.FindValidator(rest[0])
	if err != nil {
		return err
	}

	snap := client.Snapshot(*validator)
	return printJSON(snap)
}

func runDrop(args []string) error {
	inv, client, rest, err := common(args)
	if err != nil {
		return err
	}
	validator, phase, _, height, round, err := parseRuleArgs(rest, false)
	if err != nil {
		return err
	}

	v, err := inv.FindValidator(validator)
	if err != nil {
		return err
	}
	if v.ControlURL == nil {
		return fmt.Errorf("validator %s does not expose a control URL", validator)
	}

	if err := client.PutRule(*v.ControlURL, phase, "drop", height, round, ""); err != nil {
		return err
	}
	fmt.Printf("configured drop on %s %s\n", validator, phase)
	return nil
}

func runDelay(args []string) error {
	inv, client, rest, err := common(args)
	if err != nil {
		return err
	}
	validator, phase, delay, height, round, err := parseRuleArgs(rest, true)
	if err != nil {
		return err
	}

	v, err := inv.FindValidator(validator)
	if err != nil {
		return err
	}
	if v.ControlURL == nil {
		return fmt.Errorf("validator %s does not expose a control URL", validator)
	}

	if err := client.PutRule(*v.ControlURL, phase, "delay", height, round, delay); err != nil {
		return err
	}
	fmt.Printf("configured delay on %s %s (%s)\n", validator, phase, delay)
	return nil
}

func runClear(args []string) error {
	fs := flag.NewFlagSet("clear", flag.ContinueOnError)
	invPath := fs.String("inventory", defaultInventoryPath(), "inventory path")
	timeout := fs.Duration("timeout", 5*time.Second, "HTTP timeout")
	scenarioLib := fs.String("scenario-lib", defaultScenarioLibPath(), "path to val-scenarios/lib/scenario.sh")
	fs.SetOutput(os.Stderr)
	if err := fs.Parse(args); err != nil {
		return err
	}

	inv, err := valcontrol.LoadInventory(*invPath)
	if err != nil {
		return err
	}
	client := valcontrol.NewClient(*timeout)
	rest := fs.Args()
	if len(rest) < 1 || len(rest) > 2 {
		return errors.New("usage: valcontrol clear [--inventory path] [--scenario-lib <path>] <validator> [phase]")
	}

	v, err := inv.FindValidator(rest[0])
	if err != nil {
		return err
	}
	if v.ControlURL == nil {
		return fmt.Errorf("validator %s does not expose a control URL", rest[0])
	}

	state, err := client.GetSignerState(*v.ControlURL)
	if err != nil {
		state = nil
	}

	if len(rest) == 2 {
		restart := shouldRestartAfterClear(state, rest[1])
		if err := client.ClearRule(*v.ControlURL, rest[1]); err != nil {
			return err
		}
		if restart {
			if err := restartValidator(inv, *scenarioLib, rest[0]); err != nil {
				return err
			}
			fmt.Printf("cleared %s on %s and restarted validator\n", rest[1], rest[0])
			return nil
		}
		fmt.Printf("cleared %s on %s\n", rest[1], rest[0])
		return nil
	}

	restart := shouldRestartAfterClear(state, "")
	if err := client.Reset(*v.ControlURL); err != nil {
		return err
	}
	if restart {
		if err := restartValidator(inv, *scenarioLib, rest[0]); err != nil {
			return err
		}
		fmt.Printf("cleared all rules on %s and restarted validator\n", rest[0])
		return nil
	}
	fmt.Printf("cleared all rules on %s\n", rest[0])
	return nil
}

func runReset(args []string) error {
	fs := flag.NewFlagSet("reset", flag.ContinueOnError)
	invPath := fs.String("inventory", defaultInventoryPath(), "inventory path")
	scenarioLib := fs.String("scenario-lib", defaultScenarioLibPath(), "path to val-scenarios/lib/scenario.sh")
	safe := fs.Bool("safe", false, "safe reset: wipe db/wal only, preserve validator state")
	fs.SetOutput(os.Stderr)
	if err := fs.Parse(args); err != nil {
		return err
	}
	rest := fs.Args()
	if len(rest) != 1 {
		return errors.New("usage: valcontrol reset [--inventory path] [--scenario-lib path] [--safe] <validator>")
	}

	inv, err := valcontrol.LoadInventory(*invPath)
	if err != nil {
		return err
	}
	if _, err := inv.FindValidator(rest[0]); err != nil {
		return err
	}

	libPath, err := resolveScenarioLib(*scenarioLib)
	if err != nil {
		return err
	}

	fn := "reset_validator"
	if *safe {
		fn = "safe_reset_validator"
	}
	script := buildScenarioFuncScript(libPath, inv, fmt.Sprintf("%s %q", fn, rest[0]))
	if out, err := runBashScript(script); err != nil {
		return fmt.Errorf("reset %s: %w\n%s", rest[0], err, out)
	}

	label := "clean reset"
	if *safe {
		label = "safe reset"
	}
	fmt.Printf("%s %s done\n", label, rest[0])
	return nil
}

func common(args []string) (*valcontrol.Inventory, *valcontrol.Client, []string, error) {
	fs := flag.NewFlagSet("common", flag.ContinueOnError)
	invPath := fs.String("inventory", defaultInventoryPath(), "inventory path")
	timeout := fs.Duration("timeout", 5*time.Second, "HTTP timeout")
	fs.SetOutput(os.Stderr)
	if err := fs.Parse(args); err != nil {
		return nil, nil, nil, err
	}

	inv, err := valcontrol.LoadInventory(*invPath)
	if err != nil {
		return nil, nil, nil, err
	}
	return inv, valcontrol.NewClient(*timeout), fs.Args(), nil
}

func parseRuleArgs(args []string, wantDelay bool) (validator, phase, delay string, height *int64, round *int, err error) {
	fs := flag.NewFlagSet("rule", flag.ContinueOnError)
	heightFlag := fs.Int64("height", -1, "match only this height")
	roundFlag := fs.Int("round", -1, "match only this round")
	fs.SetOutput(os.Stderr)
	if err = fs.Parse(args); err != nil {
		return
	}

	rest := fs.Args()
	need := 2
	if wantDelay {
		need = 3
	}
	if len(rest) != need {
		if wantDelay {
			err = errors.New("usage: valcontrol delay [--inventory path] <validator> <phase> <duration> [--height N] [--round N]")
		} else {
			err = errors.New("usage: valcontrol drop [--inventory path] <validator> <phase> [--height N] [--round N]")
		}
		return
	}

	validator = rest[0]
	phase = strings.ToLower(rest[1])
	if wantDelay {
		delay = rest[2]
	}
	if *heightFlag >= 0 {
		height = heightFlag
	}
	if *roundFlag >= 0 {
		round = roundFlag
	}
	return
}

func defaultInventoryPath() string {
	if env := os.Getenv("INVENTORY_PATH"); env != "" {
		return env
	}
	if env := os.Getenv("VALCONTROL_INVENTORY"); env != "" {
		return env
	}
	return "inventory.json"
}

func resolvedInventoryPath(args []string) string {
	for idx := 0; idx < len(args)-1; idx++ {
		if args[idx] == "--inventory" {
			return args[idx+1]
		}
	}
	path := defaultInventoryPath()
	if abs, err := filepath.Abs(path); err == nil {
		return abs
	}
	return path
}

func printJSON(v any) error {
	bz, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	fmt.Println(string(bz))
	return nil
}
