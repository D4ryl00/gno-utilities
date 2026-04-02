package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/remi/gno-utilities/decode-msgbytes/pkg/decodemsgbytes"
)

func main() {
	input, err := readInput()
	if err != nil {
		exitf("read input: %v", err)
	}

	result, err := decodemsgbytes.DecodeInput(input)
	if err != nil {
		exitf("decode input: %v", err)
	}

	fmt.Printf("outer type: %T\n\n", result.Outer)
	printPretty("outer amino-json", result.Outer)

	if result.Tx != nil {
		fmt.Printf("\ninner type: %T\n\n", *result.Tx)
		printPretty("inner amino-json", result.Tx)
	}
}

func readInput() (string, error) {
	if len(os.Args) > 1 {
		return strings.Join(os.Args[1:], " "), nil
	}

	bz, err := io.ReadAll(os.Stdin)
	if err != nil {
		return "", err
	}
	if len(strings.TrimSpace(string(bz))) == 0 {
		return "", errors.New("usage: go run ./cmd/decode-msgbytes '<hex-or-full-log-line>'")
	}

	return string(bz), nil
}

func printPretty(label string, value any) {
	bz, err := decodemsgbytes.PrettyAminoJSON(value)
	if err != nil {
		exitf("%s: %v", label, err)
	}

	fmt.Printf("%s:\n%s\n", label, bz)
}

func exitf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}
