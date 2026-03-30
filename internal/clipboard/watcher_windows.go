//go:build windows

package clipboard

import (
	"context"
	"errors"
	"fmt"
	"runtime"
	"sync"
	"syscall"
	"time"
	"unsafe"
)

const (
	cfUnicodeText     = 13
	errClassExists    = 1410
	wmClose           = 0x0010
	wmDestroy         = 0x0002
	wmClipboardUpdate = 0x031D
	hwndMessage       = ^uintptr(2)
)

var (
	user32                         = syscall.NewLazyDLL("user32.dll")
	kernel32                       = syscall.NewLazyDLL("kernel32.dll")
	procAddClipboardFormatListener = user32.NewProc("AddClipboardFormatListener")
	procCloseClipboard             = user32.NewProc("CloseClipboard")
	procCreateWindowExW            = user32.NewProc("CreateWindowExW")
	procDefWindowProcW             = user32.NewProc("DefWindowProcW")
	procDestroyWindow              = user32.NewProc("DestroyWindow")
	procDispatchMessageW           = user32.NewProc("DispatchMessageW")
	procGetClipboardData           = user32.NewProc("GetClipboardData")
	procGetMessageW                = user32.NewProc("GetMessageW")
	procIsClipboardFormatAvailable = user32.NewProc("IsClipboardFormatAvailable")
	procOpenClipboard              = user32.NewProc("OpenClipboard")
	procPostMessageW               = user32.NewProc("PostMessageW")
	procPostQuitMessage            = user32.NewProc("PostQuitMessage")
	procRegisterClassW             = user32.NewProc("RegisterClassW")
	procRemoveClipboardListener    = user32.NewProc("RemoveClipboardFormatListener")
	procTranslateMessage           = user32.NewProc("TranslateMessage")
	procGetModuleHandleW           = kernel32.NewProc("GetModuleHandleW")
	procGlobalLock                 = kernel32.NewProc("GlobalLock")
	procGlobalUnlock               = kernel32.NewProc("GlobalUnlock")
)

var (
	windowProcPtr   = syscall.NewCallback(windowProc)
	watcherRegistry sync.Map
	watcherClassMu  sync.Mutex
	watcherClassSet bool
	registerClassW  = func(wc *wndClass) (uintptr, uintptr, error) {
		return procRegisterClassW.Call(uintptr(unsafe.Pointer(wc)))
	}
)

const watcherWindowClassName = "TailclipClipboardWatcher"

type point struct {
	X int32
	Y int32
}

type msg struct {
	HWnd     uintptr
	Message  uint32
	WParam   uintptr
	LParam   uintptr
	Time     uint32
	Pt       point
	LPrivate uint32
}

type wndClass struct {
	Style      uint32
	WndProc    uintptr
	ClsExtra   int32
	WndExtra   int32
	Instance   uintptr
	Icon       uintptr
	Cursor     uintptr
	Background uintptr
	MenuName   *uint16
	ClassName  *uint16
}

type TextChange struct {
	Text       string
	DetectedAt time.Time
}

type Watcher struct {
	events  chan TextChange
	notify  chan struct{}
	errs    chan error
	started chan struct{}

	mu   sync.Mutex
	hwnd uintptr

	startOnce sync.Once
}

func NewWatcher() *Watcher {
	return &Watcher{
		events:  make(chan TextChange, 16),
		notify:  make(chan struct{}, 1),
		errs:    make(chan error, 1),
		started: make(chan struct{}),
	}
}

func (w *Watcher) Next(ctx context.Context) (TextChange, error) {
	w.startOnce.Do(func() {
		go w.run(ctx)
		go w.processNotifications(ctx)
	})

	select {
	case <-ctx.Done():
		return TextChange{}, ctx.Err()
	case err := <-w.errs:
		return TextChange{}, err
	case change := <-w.events:
		return change, nil
	}
}

func (w *Watcher) run(ctx context.Context) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	className := syscall.StringToUTF16Ptr(watcherWindowClassName)

	instance, _, err := procGetModuleHandleW.Call(0)
	if instance == 0 {
		w.sendError(fmt.Errorf("GetModuleHandleW failed: %w", err))
		close(w.started)
		return
	}

	if err := ensureWatcherWindowClass(instance); err != nil {
		w.sendError(err)
		close(w.started)
		return
	}

	hwnd, _, err := procCreateWindowExW.Call(
		0,
		uintptr(unsafe.Pointer(className)),
		uintptr(unsafe.Pointer(className)),
		0,
		0,
		0,
		0,
		0,
		hwndMessage,
		0,
		instance,
		0,
	)
	if hwnd == 0 {
		w.sendError(fmt.Errorf("CreateWindowExW failed: %w", err))
		close(w.started)
		return
	}

	w.mu.Lock()
	w.hwnd = hwnd
	w.mu.Unlock()
	watcherRegistry.Store(hwnd, w)
	close(w.started)
	defer watcherRegistry.Delete(hwnd)

	ok, _, err := procAddClipboardFormatListener.Call(hwnd)
	if ok == 0 {
		procDestroyWindow.Call(hwnd)
		w.sendError(fmt.Errorf("AddClipboardFormatListener failed: %w", err))
		return
	}
	defer procRemoveClipboardListener.Call(hwnd)

	go func() {
		<-ctx.Done()
		<-w.started
		if hwnd != 0 {
			procPostMessageW.Call(hwnd, wmClose, 0, 0)
		}
	}()

	var message msg
	for {
		ret, _, err := procGetMessageW.Call(uintptr(unsafe.Pointer(&message)), 0, 0, 0)
		switch int32(ret) {
		case -1:
			w.sendError(fmt.Errorf("GetMessageW failed: %w", err))
			return
		case 0:
			return
		}

		procTranslateMessage.Call(uintptr(unsafe.Pointer(&message)))
		procDispatchMessageW.Call(uintptr(unsafe.Pointer(&message)))
	}
}

func (w *Watcher) processNotifications(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-w.notify:
			text, err := readText()
			if err != nil || text == "" {
				continue
			}

			change := TextChange{
				Text:       text,
				DetectedAt: time.Now().UTC(),
			}

			select {
			case w.events <- change:
			case <-ctx.Done():
				return
			}
		}
	}
}

func (w *Watcher) onClipboardUpdate() {
	select {
	case w.notify <- struct{}{}:
	default:
	}
}

func (w *Watcher) sendError(err error) {
	select {
	case w.errs <- err:
	default:
	}
}

func ensureWatcherWindowClass(instance uintptr) error {
	watcherClassMu.Lock()
	defer watcherClassMu.Unlock()

	if watcherClassSet {
		return nil
	}

	className := syscall.StringToUTF16Ptr(watcherWindowClassName)
	wc := wndClass{
		WndProc:   windowProcPtr,
		Instance:  instance,
		ClassName: className,
	}

	atom, _, err := registerClassW(&wc)
	if atom == 0 && !errors.Is(err, syscall.Errno(errClassExists)) {
		return fmt.Errorf("RegisterClassW failed: %w", err)
	}

	watcherClassSet = true
	return nil
}

func readText() (string, error) {
	if err := openClipboardWithRetry(); err != nil {
		return "", err
	}
	defer procCloseClipboard.Call()

	available, _, _ := procIsClipboardFormatAvailable.Call(cfUnicodeText)
	if available == 0 {
		return "", nil
	}

	handle, _, err := procGetClipboardData.Call(cfUnicodeText)
	if handle == 0 {
		return "", fmt.Errorf("GetClipboardData failed: %w", err)
	}

	ptr, _, err := procGlobalLock.Call(handle)
	if ptr == 0 {
		return "", fmt.Errorf("GlobalLock failed: %w", err)
	}
	defer procGlobalUnlock.Call(handle)

	text := windowsUTF16PtrToString((*uint16)(unsafe.Pointer(ptr)))
	return text, nil
}

func openClipboardWithRetry() error {
	var lastErr error
	for range 5 {
		r1, _, err := procOpenClipboard.Call(0)
		if r1 != 0 {
			return nil
		}

		lastErr = err
		time.Sleep(20 * time.Millisecond)
	}

	return fmt.Errorf("OpenClipboard failed: %w", lastErr)
}

func windowsUTF16PtrToString(ptr *uint16) string {
	if ptr == nil {
		return ""
	}

	buf := make([]uint16, 0, 256)
	for i := 0; ; i++ {
		ch := *(*uint16)(unsafe.Pointer(uintptr(unsafe.Pointer(ptr)) + uintptr(i)*unsafe.Sizeof(*ptr)))
		if ch == 0 {
			break
		}
		buf = append(buf, ch)
	}

	return syscall.UTF16ToString(buf)
}

func windowProc(hwnd uintptr, msg uint32, wParam, lParam uintptr) uintptr {
	if value, ok := watcherRegistry.Load(hwnd); ok {
		watcher := value.(*Watcher)
		switch msg {
		case wmClipboardUpdate:
			watcher.onClipboardUpdate()
			return 0
		case wmDestroy:
			procPostQuitMessage.Call(0)
			return 0
		case wmClose:
			procDestroyWindow.Call(hwnd)
			return 0
		}
	}

	ret, _, _ := procDefWindowProcW.Call(hwnd, uintptr(msg), wParam, lParam)
	return ret
}
