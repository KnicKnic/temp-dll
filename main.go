package tempdll

import (
	"bytes"
	"crypto/sha256"
	"encoding/base32"
	"io"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

// A LazyDLL implements access to a single DLL.
// It will delay the load of the DLL until the first
// call to its Handle method or to one of its
// LazyProc's Addr method.
type LazyDLL struct {
	mu   sync.Mutex
	dll  *syscall.DLL // non nil once DLL is loaded
	Name string

	wroteDll  bool
	dllHandle *os.File // a readonly fileHandle to block writes
	fileName  string   // the full path to the dll that is created
	dllData   io.Reader
}

func copyFile(dst string, data io.Reader) error {
	destination, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer destination.Close()
	_, err = io.Copy(destination, data)
	return err
}

func CopyFile(dst string, data io.Reader) string {
	err := copyFile(dst, data)
	if err != nil {
		panic(err)
	}
	return dst
}

func openWithDelete(fileName string) (*syscall.Handle, error) {

	name, e := syscall.UTF16PtrFromString(fileName)
	if e != nil {
		return nil, e
	}
	// sharemode := uint32(syscall.FILE_SHARE_READ)
	var sa *syscall.SecurityAttributes

	// 0x00010000 is delete read it was needed if you use FILE_FLAG_DELETE_ON_CLOSE
	handle, e := syscall.CreateFile(name, syscall.GENERIC_WRITE|syscall.GENERIC_READ|0x00010000, syscall.FILE_SHARE_READ|syscall.FILE_SHARE_DELETE, sa, syscall.OPEN_EXISTING, windows.FILE_FLAG_DELETE_ON_CLOSE, 0)
	return &handle, e
}
func OpenWithDelete(fileName string) *syscall.Handle {
	handle, e := openWithDelete(fileName)
	if e != nil {
		panic(e)
	}
	return handle
}

var fileNameSafeEncoder = base32.StdEncoding.WithPadding(base32.NoPadding)

// Load loads DLL file d.Name into memory. It returns an error if fails.
// Load will not try to load DLL, if it is already loaded into memory.
func (d *LazyDLL) Load() error {
	// Non-racy version of:
	// if d.dll == nil {
	if atomic.LoadPointer((*unsafe.Pointer)(unsafe.Pointer(&d.dll))) == nil {
		d.mu.Lock()
		defer d.mu.Unlock()
		if d.dll == nil {
			if !d.wroteDll {

				contents := bytes.NewBuffer(nil)
				_, e := io.Copy(contents, d.dllData)
				if e != nil {
					return e
				}

				sha := sha256.Sum256(contents.Bytes())
				shaFileName := fileNameSafeEncoder.EncodeToString(sha[:])
				d.fileName = filepath.Join(os.TempDir(), (shaFileName + "-" + d.Name))

				d.dllHandle, e = safeWriteFile(d.fileName, contents.Bytes(), 60, time.Second)
				if e != nil {
					return e
				}

				d.wroteDll = true
			}
			dll, e := syscall.LoadDLL(d.fileName)
			if e != nil {
				return e
			}
			// Non-racy version of:
			// d.dll = dll
			atomic.StorePointer((*unsafe.Pointer)(unsafe.Pointer(&d.dll)), unsafe.Pointer(dll))
		}
	}
	return nil
}

// mustLoad is like Load but panics if search fails.
func (d *LazyDLL) mustLoad() {
	e := d.Load()
	if e != nil {
		panic(e)
	}
}

// Handle returns d's module handle.
func (d *LazyDLL) Handle() uintptr {
	d.mustLoad()
	return uintptr(d.dll.Handle)
}

// NewLazyDLL creates new LazyDLL associated with DLL file.
func NewLazyDLL(dll io.Reader, name string) *LazyDLL {
	return &LazyDLL{Name: name, dllData: dll}
}

// A LazyProc implements access to a procedure inside a LazyDLL.
// It delays the lookup until the Addr, Call, or Find method is called.
type LazyProc struct {
	mu   sync.Mutex
	Name string
	l    *LazyDLL
	proc *syscall.Proc
}

// NewProc returns a LazyProc for accessing the named procedure in the DLL d.
func (d *LazyDLL) NewProc(name string) *LazyProc {
	return &LazyProc{l: d, Name: name}
}

// Find searches DLL for procedure named p.Name. It returns
// an error if search fails. Find will not search procedure,
// if it is already found and loaded into memory.
func (p *LazyProc) Find() error {
	// Non-racy version of:
	// if p.proc == nil {
	if atomic.LoadPointer((*unsafe.Pointer)(unsafe.Pointer(&p.proc))) == nil {
		p.mu.Lock()
		defer p.mu.Unlock()
		if p.proc == nil {
			e := p.l.Load()
			if e != nil {
				return e
			}
			proc, e := p.l.dll.FindProc(p.Name)
			if e != nil {
				return e
			}
			// Non-racy version of:
			// p.proc = proc
			atomic.StorePointer((*unsafe.Pointer)(unsafe.Pointer(&p.proc)), unsafe.Pointer(proc))
		}
	}
	return nil
}

// mustFind is like Find but panics if search fails.
func (p *LazyProc) mustFind() {
	e := p.Find()
	if e != nil {
		panic(e)
	}
}

// Addr returns the address of the procedure represented by p.
// The return value can be passed to Syscall to run the procedure.
func (p *LazyProc) Addr() uintptr {
	p.mustFind()
	return p.proc.Addr()
}

//go:uintptrescapes

// Call executes procedure p with arguments a. See the documentation of
// Proc.Call for more information.
func (p *LazyProc) Call(a ...uintptr) (r1, r2 uintptr, lastErr error) {
	p.mustFind()
	return p.proc.Call(a...)
}
