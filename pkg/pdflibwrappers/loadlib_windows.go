package pdflibwrappers

import (
	"errors"
	"syscall"
)

// TryLoadLib tries to load a shared object/dynamically linked library
// from various paths and returns a handle or 0 and an error.
func TryLoadLib(paths ...string) (uintptr, string, error) {
	var lib syscall.Handle
	var liberr, err error
	for _, path := range paths {
		lib, liberr = syscall.LoadLibrary(path)
		err = errors.Join(liberr, err)
		if lib != 0 {
			return uintptr(lib), path, nil
		}
	}
	return 0, "", err
}