//go:build linux || darwin

package pdflibwrappers

import (
	"errors"

	"github.com/ebitengine/purego"
)

// CloseLib closes the last lib opened by [TryLoadLib]
var CloseLib func() = func() {}

// TryLoadLib tries to load a shared object/dynamically linked library
// from various paths and returns a handle or 0 and an error.
func TryLoadLib(paths ...string) (uintptr, string, error) {
	var lib uintptr
	var liberr, err error
	for _, path := range paths {
		lib, liberr = purego.Dlopen(path, purego.RTLD_NOW)
		err = errors.Join(liberr, err)
		if lib != 0 {
			return lib, path, nil
		}
		CloseLib = func() { purego.Dlclose(lib) }
	}
	return 0, "", err
}
