package decodemsgbytes

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"regexp"
	"strings"

	_ "github.com/gnolang/gno/gno.land/pkg/sdk/vm"
	"github.com/gnolang/gno/tm2/pkg/amino"
	_ "github.com/gnolang/gno/tm2/pkg/bft/consensus"
	"github.com/gnolang/gno/tm2/pkg/bft/mempool"
	"github.com/gnolang/gno/tm2/pkg/std"

	_ "github.com/gnolang/gno/tm2/pkg/crypto/ed25519"
	_ "github.com/gnolang/gno/tm2/pkg/crypto/multisig"
	_ "github.com/gnolang/gno/tm2/pkg/crypto/secp256k1"
)

var msgBytesRE = regexp.MustCompile(`(?i)"msgBytes"\s*:\s*"([0-9a-f]+)"`)

type Result struct {
	RawHex string
	Outer  any
	Tx     *std.Tx
}

func ExtractHex(input string) (string, error) {
	if matches := msgBytesRE.FindStringSubmatch(input); len(matches) == 2 {
		return matches[1], nil
	}

	trimmed := strings.TrimSpace(input)
	trimmed = strings.Trim(trimmed, `"'`)
	trimmed = strings.ReplaceAll(trimmed, " ", "")
	trimmed = strings.ReplaceAll(trimmed, "\n", "")
	trimmed = strings.ReplaceAll(trimmed, "\t", "")

	if trimmed == "" {
		return "", errors.New("no hex payload found")
	}
	if len(trimmed)%2 != 0 {
		return "", errors.New("hex payload has odd length")
	}
	for _, r := range trimmed {
		if !strings.ContainsRune("0123456789abcdefABCDEF", r) {
			return "", errors.New("input is not raw hex and does not contain a msgBytes field")
		}
	}

	return trimmed, nil
}

func DecodeInput(input string) (*Result, error) {
	hexStr, err := ExtractHex(input)
	if err != nil {
		return nil, err
	}

	return DecodeHex(hexStr)
}

func DecodeHex(hexStr string) (*Result, error) {
	raw, err := hex.DecodeString(hexStr)
	if err != nil {
		return nil, err
	}

	return DecodeBytes(hexStr, raw)
}

func DecodeBytes(rawHex string, raw []byte) (*Result, error) {
	var outer any
	if err := amino.Unmarshal(raw, &outer); err != nil {
		return nil, err
	}

	res := &Result{
		RawHex: rawHex,
		Outer:  outer,
	}

	txMsg, ok := outer.(*mempool.TxMessage)
	if !ok {
		return res, nil
	}

	var tx std.Tx
	if err := amino.Unmarshal(txMsg.Tx, &tx); err != nil {
		return nil, err
	}

	res.Tx = &tx
	return res, nil
}

func AminoJSON(v any) ([]byte, error) {
	return amino.MarshalJSON(v)
}

func PrettyAminoJSON(v any) ([]byte, error) {
	bz, err := AminoJSON(v)
	if err != nil {
		return nil, err
	}

	var out any
	if err := json.Unmarshal(bz, &out); err != nil {
		return bz, nil
	}

	pretty, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		return bz, nil
	}

	return pretty, nil
}
