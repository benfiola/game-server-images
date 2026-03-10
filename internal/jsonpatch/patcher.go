package jsonpatch

import (
	"context"
	"encoding/json"
	"errors"
	"os"

	jsonpatchx "github.com/evanphx/json-patch/v5"
)

type Opts struct{}

type JSONPatcher struct{}

func New(opts *Opts) (*JSONPatcher, error) {
	return &JSONPatcher{}, nil
}

type Patch struct {
	Op    string `json:"op"`
	Path  string `json:"path"`
	Value any    `json:"value,omitempty"`
}

type PatchMap map[string][]Patch

func (j *JSONPatcher) ApplyPatches(ctx context.Context, patchMap PatchMap) error {
	for file, patches := range patchMap {
		fileBytes, err := os.ReadFile(file)
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

		patchedBytes, err := jsonPatch.ApplyIndent(fileBytes, "  ")
		if err != nil {
			return err
		}

		perm := os.FileMode(0644)
		stat, err := os.Stat(file)
		if err == nil {
			perm = stat.Mode().Perm()
		} else if !errors.Is(err, os.ErrNotExist) {
			return err
		}

		if err := os.WriteFile(file, patchedBytes, perm); err != nil {
			return err
		}
	}
	return nil
}

func (j *JSONPatcher) ApplyPatchesFromFile(ctx context.Context, file string) error {
	fileBytes, err := os.ReadFile(file)
	if err != nil {
		return err
	}

	patchMap := PatchMap{}
	if err := json.Unmarshal(fileBytes, &patchMap); err != nil {
		return err
	}

	return j.ApplyPatches(ctx, patchMap)
}
