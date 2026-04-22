package valcontrol

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
)

var ErrValidatorNotFound = errors.New("validator not found")

type Inventory struct {
	Scenario    string      `json:"scenario"`
	WorkDir     string      `json:"work_dir"`
	ComposeFile string      `json:"compose_file"`
	Validators  []Validator `json:"validators"`
}

type Validator struct {
	Name               string  `json:"name"`
	RPCURL             string  `json:"rpc_url"`
	ControlURL         *string `json:"control_url"`
	Service            string  `json:"service"`
	SignerService      string  `json:"signer_service"`
	ControllableSigner bool    `json:"controllable_signer"`
	Address            string  `json:"address"`
	PubKey             string  `json:"pub_key"`
}

func LoadInventory(path string) (*Inventory, error) {
	bz, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read inventory: %w", err)
	}

	var inv Inventory
	if err := json.Unmarshal(bz, &inv); err != nil {
		return nil, fmt.Errorf("decode inventory: %w", err)
	}

	return &inv, nil
}

func (i *Inventory) FindValidator(name string) (*Validator, error) {
	for idx := range i.Validators {
		if i.Validators[idx].Name == name {
			return &i.Validators[idx], nil
		}
	}

	return nil, fmt.Errorf("%w: %s", ErrValidatorNotFound, name)
}
