//go:build tesseract_pure

package tesswrap

import (
	"errors"
	"io"
	"sync"
	"unsafe"

	"github.com/ebitengine/purego"
	"github.com/johbar/text-extraction-service/v2/internal/unix"
)

var (
	TessVersion       func() *byte
	TessBaseAPICreate func() uintptr
	TessBaseAPIDelete func(handle uintptr)
	TessBaseAPIInit3  func(baseApiHandle uintptr, datapath unsafe.Pointer, lang *byte) int
	// This returns a vector, essentially a slice. purego can't deal with that
	// TessBaseAPIGetAvailableLanguagesAsVector func(baseApiHandle uintptr) *uintptr
	/*
		Close down tesseract and free up all memory. End() is equivalent to destructing and reconstructing
		your TessBaseAPI. Once End() has been used, none of the other API functions may be used other than Init.
	*/
	TessBaseAPIEnd            func(handle uintptr)
	TessBaseAPISetImage2      func(handle uintptr, pix uintptr)
	TessBaseAPIGetUTF8Text    func(handle uintptr) *byte
	TessBaseAPISetPageSegMode func(handle uintptr, mode uint32)
	/*
		Free up recognition results and any stored image data,
		without actually freeing any recognition data that would be time-consuming to reload.
		Afterwards, you must call SetImage or TesseractRect before doing any Recognize or Get* operation.
	*/
	TessBaseAPIClear func(handle uintptr)

	pixReadMem  func(data *byte, lenght uint64) uintptr
	pixFreeData func(data uintptr)
	free        func(*byte)
	lock        sync.Mutex
	handle      uintptr
)

func init() {
	lib, err := purego.Dlopen("libtesseract.so", purego.RTLD_LAZY)
	if err != nil {
		Initialized = false
		return
	}
	purego.RegisterLibFunc(&TessBaseAPICreate, lib, "TessBaseAPICreate")
	purego.RegisterLibFunc(&TessBaseAPIDelete, lib, "TessBaseAPIDelete")
	purego.RegisterLibFunc(&TessBaseAPIInit3, lib, "TessBaseAPIInit3")
	purego.RegisterLibFunc(&TessBaseAPIEnd, lib, "TessBaseAPIEnd")

	purego.RegisterLibFunc(&TessVersion, lib, "TessVersion")
	purego.RegisterLibFunc(&TessBaseAPISetImage2, lib, "TessBaseAPISetImage2")
	purego.RegisterLibFunc(&TessBaseAPIGetUTF8Text, lib, "TessBaseAPIGetUTF8Text")
	purego.RegisterLibFunc(&free, lib, "free")
	purego.RegisterLibFunc(&pixReadMem, lib, "pixReadMem")
	purego.RegisterLibFunc(&pixFreeData, lib, "pixFreeData")
	purego.RegisterLibFunc(&TessBaseAPISetPageSegMode, lib, "TessBaseAPISetPageSegMode")
	purego.RegisterLibFunc(&TessBaseAPIClear, lib, "TessBaseAPIClear")

	Version = getVersion()
	Initialized = true
}

func initLib() {
	handle = TessBaseAPICreate()
	lang, _ := unix.BytePtrFromString(Languages)
	if ret := TessBaseAPIInit3(handle, unsafe.Pointer(nil), lang); ret != 0 {
		Initialized = false
	}
}

func getVersion() string {
	return unix.BytePtrToString(TessVersion())
}

func ImageReaderToText(r io.Reader) (string, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return "", err
	}
	return ImageBytesToText(data)
}

func ImageBytesToText(data []byte) (string, error) {
	lock.Lock()
	defer lock.Unlock()
	if handle == 0 {
		// start the tesseract lib if it hasn't been already
		initLib()
	}
	if !Initialized {
		return "", errors.New("tesseract could not be initialized")
	}
	defer TessBaseAPIClear(handle)

	TessBaseAPISetPageSegMode(handle, 1) //PSM_AUTO_OSD
	_pix := pixReadMem(&data[0], uint64(len(data)))
	if _pix == 0 {
		return "", errors.New("not an image")
	}
	defer pixFreeData(_pix)
	TessBaseAPISetImage2(handle, _pix)
	text := TessBaseAPIGetUTF8Text(handle)
	result := unix.BytePtrToString(text)
	free(text)
	return result, nil
}

func ImageReaderToTextWriter(r io.Reader, w io.Writer) error {
	txt, err := ImageReaderToText(r)
	if err != nil {
		return err
	}
	w.Write([]byte(txt))
	return nil
}

func IsTesseractConfigOk() (ok bool, reason string) {
	return true, ""
}
