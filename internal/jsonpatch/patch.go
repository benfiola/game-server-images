package jsonpatch

import (
	"encoding/json"

	jsonpatchx "github.com/evanphx/json-patch/v5"
)

type Patch struct {
	Op    string `json:"op"`
	Path  string `json:"path"`
	Value any    `json:"value,omitempty"`
}

type PatchMap map[string][]Patch

func ApplyPatches(original any, patches []Patch, patched any) error {
	originalBytes, err := json.Marshal(original)
	if err != nil {
		return err
	}

	patchesBytes, err := json.Marshal(patches)
	if err != nil {
		return err
	}

	jsonPatch, err := jsonpatchx.DecodePatch(patchesBytes)
	if err != nil {
		return err
	}

	patchedBytes, err := jsonPatch.Apply(originalBytes)
	if err != nil {
		return err
	}

	if err := json.Unmarshal(patchedBytes, patched); err != nil {
		return err
	}

	return nil
}
