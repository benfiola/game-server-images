package datatransform

import (
	"encoding/json"
	"fmt"

	jsonpatch "github.com/evanphx/json-patch/v5"
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

	jsonPatch, err := jsonpatch.DecodePatch(patchesBytes)
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

func MergeShallow(items ...map[string]any) map[string]any {
	merged := items[0]

	for _, item := range items[1:] {
		patches := []Patch{}
		for key, value := range item {
			op := "add"
			if _, ok := merged[key]; ok {
				op = "replace"
			}

			patch := Patch{
				Op:    op,
				Path:  fmt.Sprintf("/%s", key),
				Value: value,
			}
			patches = append(patches, patch)
		}

		ApplyPatches(merged, patches, &merged)
	}

	return merged
}
