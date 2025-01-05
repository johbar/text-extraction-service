package pdflibwrappers

import (
	"errors"

	"github.com/ebitengine/purego"
)

// TryLoadLib tries to load a shared object/dynamically linked library
// from various paths and returns a handle or 0 and an error.
func TryLoadLib(paths ...string) (uintptr, string, error) {
	var lib uintptr
	var liberr, err error
	for _, path := range paths {
		lib, liberr = purego.Dlopen(path, purego.RTLD_NOW|purego.RTLD_GLOBAL)
		err = errors.Join(liberr, err)
		if lib != 0 {
			return lib, path, nil
		}
	}
	return 0, "", err
}
