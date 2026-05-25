//go:build windows

package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"runtime/debug"
	"sync"
	"syscall"
	"time"
	"unsafe"
)

const (
	WS_EX_LAYERED     = 0x00080000
	WS_EX_TOPMOST     = 0x00000008
	WS_EX_TOOLWINDOW  = 0x00000080
	WS_POPUP          = 0x80000000
	SW_SHOW           = 5
	ULW_ALPHA         = 0x00000002
	BI_RGB            = 0
	DIB_RGB_COLORS    = 0
	CS_DBLCLKS        = 0x0008
	VK_LBUTTON        = 0x01
	WM_DESTROY        = 0x0002
	WM_CLOSE          = 0x0010
	WM_CANCELMODE     = 0x001F
	WM_LBUTTONDOWN    = 0x0201
	WM_LBUTTONUP      = 0x0202
	WM_LBUTTONDBLCLK  = 0x0203
	WM_RBUTTONDOWN    = 0x0204
	WM_CAPTURECHANGED = 0x0215
	AC_SRC_OVER       = 0x00
	AC_SRC_ALPHA      = 0x01
	MB_OK             = 0x00000000
	MB_ICONERROR      = 0x00000010
)

type point struct{ X, Y int32 }
type size struct{ CX, CY int32 }
type blendFunction struct{ BlendOp, BlendFlags, SourceConstantAlpha, AlphaFormat byte }
type bitmapInfoHeader struct {
	Size          uint32
	Width         int32
	Height        int32
	Planes        uint16
	BitCount      uint16
	Compression   uint32
	SizeImage     uint32
	XPelsPerMeter int32
	YPelsPerMeter int32
	ClrUsed       uint32
	ClrImportant  uint32
}
type bitmapInfo struct {
	Header bitmapInfoHeader
	Colors [1]uint32
}

var (
	user32               = syscall.NewLazyDLL("user32.dll")
	gdi32                = syscall.NewLazyDLL("gdi32.dll")
	kernel32             = syscall.NewLazyDLL("kernel32.dll")
	pRegisterClassExW    = user32.NewProc("RegisterClassExW")
	pCreateWindowExW     = user32.NewProc("CreateWindowExW")
	pDestroyWindow       = user32.NewProc("DestroyWindow")
	pDefWindowProcW      = user32.NewProc("DefWindowProcW")
	pMessageBoxW         = user32.NewProc("MessageBoxW")
	pShowWindow          = user32.NewProc("ShowWindow")
	pPostQuitMessage     = user32.NewProc("PostQuitMessage")
	pGetMessageW         = user32.NewProc("GetMessageW")
	pTranslateMessage    = user32.NewProc("TranslateMessage")
	pDispatchMessageW    = user32.NewProc("DispatchMessageW")
	pGetSystemMetrics    = user32.NewProc("GetSystemMetrics")
	pGetCursorPos        = user32.NewProc("GetCursorPos")
	pGetAsyncKeyState    = user32.NewProc("GetAsyncKeyState")
	pUpdateLayeredWindow = user32.NewProc("UpdateLayeredWindow")
	pGetDC               = user32.NewProc("GetDC")
	pReleaseDC           = user32.NewProc("ReleaseDC")
	pCreateCompatibleDC  = gdi32.NewProc("CreateCompatibleDC")
	pCreateDIBSection    = gdi32.NewProc("CreateDIBSection")
	pSelectObject        = gdi32.NewProc("SelectObject")
	pDeleteObject        = gdi32.NewProc("DeleteObject")
	pDeleteDC            = gdi32.NewProc("DeleteDC")
	pGetModuleHandleW    = kernel32.NewProc("GetModuleHandleW")
)

type wndClassEx struct {
	Size       uint32
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
	IconSm     uintptr
}
type msg struct {
	Hwnd    uintptr
	Message uint32
	WParam  uintptr
	LParam  uintptr
	Time    uint32
	Pt      point
}

type WindowPet struct {
	HWND        uintptr
	Pet         *Pet
	Bitmap      uintptr
	DC          uintptr
	OldBitmap   uintptr
	Bits        uintptr
	Drag        bool
	PendingLeft bool
	DragOffX    int32
	DragOffY    int32
	DownScreenX int32
	DownScreenY int32
	DownAt      time.Time
	Scale       float64
	FrameW      int
	FrameH      int
}

type InputEventKind int

const (
	InputLeftDown InputEventKind = iota + 1
	InputLeftUp
	InputLeftDouble
	InputRightDown
	InputCancel
)

type InputEvent struct {
	Kind   InputEventKind
	HWND   uintptr
	X      int32
	Y      int32
	Reason string
}

type App struct {
	Profile      Profile
	Pets         []*WindowPet
	Inputs       chan InputEvent
	Mu           sync.RWMutex
	ScreenW      int
	ScreenH      int
	ClickCommand string
	RightCommand string
}

var app *App

func main() {
	debug.SetGCPercent(25)
	defer func() {
		if r := recover(); r != nil {
			fatalExit(2, "fatal panic in main: %v\n%s", r, debug.Stack())
		}
	}()
	rand.Seed(time.Now().UnixNano())
	profilePath := flag.String("profile", "", "optional profile json; empty means auto-discover selected assets/pets")
	assetsPath := flag.String("assets", "..\\assets", "assets root containing pet.json and pets/*")
	petSelect := flag.String("pet", "", "comma-separated pet ids to run, e.g. pet5; omit to run first discovered pet; use all to run every discovered pet")
	petsOverride := flag.Int("count", 0, "override instance count for first selected pet")
	scaleOverride := flag.Float64("scale", 0, "optional temporary scale override; default scale is read from pet.json")
	catalog := flag.Bool("catalog", false, "print loaded pet/animation catalog")
	clickCommand := flag.String("click-cmd", "", "optional command run on left click")
	rightCommand := flag.String("right-cmd", "", "optional command run on right click")
	flag.Parse()

	initLog()
	log.Printf("startup args profile=%q assets=%q pet=%q count=%d scale=%.2f catalog=%v click_cmd=%v right_cmd=%v", *profilePath, *assetsPath, *petSelect, *petsOverride, *scaleOverride, *catalog, *clickCommand != "", *rightCommand != "")
	profile, profileBase, err := loadRuntimeProfile(*profilePath, *assetsPath, *petSelect)
	if err != nil {
		fatalExit(1, "load runtime profile failed: %v", err)
	}
	app = &App{Profile: profile, Inputs: make(chan InputEvent, 256), ScreenW: int(getSystemMetrics(0)), ScreenH: int(getSystemMetrics(1)), ClickCommand: *clickCommand, RightCommand: *rightCommand}
	log.Printf("screen size=%dx%d selected_groups=%d", app.ScreenW, app.ScreenH, len(profile.ActivePets))
	if *catalog {
		if err := app.printCatalogOnly(profileBase, *assetsPath); err != nil {
			fatalExit(1, "catalog failed: %v", err)
		}
		return
	}
	if err := app.createWindows(profileBase, *assetsPath, *petsOverride, *scaleOverride, false); err != nil {
		fatalExit(1, "create windows failed: %v", err)
	}
	go app.loop()
	messageLoop()
}

func (a *App) printCatalogOnly(profileBase string, assetsPath string) error {
	profileDir := profileBase
	defaultManifestPath := filepath.Join(assetsPath, "pet.json")
	for _, active := range a.Profile.ActivePets {
		if !active.Enabled {
			continue
		}
		manifestPath := active.Manifest
		if manifestPath == "" {
			manifestPath = filepath.Join(assetsPath, "pets", active.PetID, "pet.json")
		} else if !filepath.IsAbs(manifestPath) {
			manifestPath = filepath.Clean(filepath.Join(profileDir, manifestPath))
		}
		var manifest PetManifest
		var err error
		if active.Manifest == "" {
			manifest, err = LoadPetManifestMerged(defaultManifestPath, filepath.Dir(manifestPath))
		} else {
			manifest, err = LoadPetManifest(manifestPath)
		}
		if err != nil {
			return err
		}
		printCatalog(manifest)
	}
	return nil
}

func (a *App) createWindows(profileBase string, assetsPath string, petsOverride int, scaleOverride float64, catalog bool) error {
	cls, err := syscall.UTF16PtrFromString("DesktopPetLiteGoWindow")
	if err != nil {
		return err
	}
	inst, _, _ := pGetModuleHandleW.Call(0)
	wc := wndClassEx{Size: uint32(unsafe.Sizeof(wndClassEx{})), Style: CS_DBLCLKS, WndProc: syscall.NewCallback(wndProc), Instance: inst, ClassName: cls}
	pRegisterClassExW.Call(uintptr(unsafe.Pointer(&wc)))
	profileDir := profileBase
	defaultManifestPath := filepath.Join(assetsPath, "pet.json")
	for groupIndex, active := range a.Profile.ActivePets {
		if !active.Enabled {
			continue
		}
		manifestPath := active.Manifest
		if manifestPath == "" {
			manifestPath = filepath.Join(assetsPath, "pets", active.PetID, "pet.json")
		} else if !filepath.IsAbs(manifestPath) {
			manifestPath = filepath.Clean(filepath.Join(profileDir, manifestPath))
		}
		var manifest PetManifest
		var err error
		if active.Manifest == "" {
			manifest, err = LoadPetManifestMerged(defaultManifestPath, filepath.Dir(manifestPath))
		} else {
			manifest, err = LoadPetManifest(manifestPath)
		}
		if err != nil {
			return err
		}
		log.Printf("load pet manifest id=%s name=%q dir=%s animations=%d default=%s", manifest.ID, manifest.Name, manifest.BaseDir, len(manifest.Animations), manifest.DefaultAnimation)
		store, err := LoadSpriteStore(manifest)
		if err != nil {
			return err
		}
		manifest = store.Manifest
		log.Printf("loaded sprite store pet=%s animations=%d", manifest.ID, len(store.Animations))
		count := active.Count
		if groupIndex == 0 && petsOverride > 0 {
			count = petsOverride
		}
		if count <= 0 {
			count = 1
		}
		scale := manifest.Scale
		if active.Scale > 0 {
			scale = active.Scale
		}
		if scaleOverride > 0 {
			scale = scaleOverride
		}
		if scale <= 0 {
			scale = 0.5
		}
		frameW := max(32, int(float64(manifest.FrameWidth)*scale))
		frameH := max(32, int(float64(manifest.FrameHeight)*scale))
		if catalog {
			printCatalog(manifest)
		}
		for i := 0; i < count; i++ {
			hwnd, _, err := pCreateWindowExW.Call(WS_EX_LAYERED|WS_EX_TOPMOST|WS_EX_TOOLWINDOW, uintptr(unsafe.Pointer(cls)), uintptr(unsafe.Pointer(cls)), WS_POPUP, 0, 0, uintptr(frameW), uintptr(frameH), 0, 0, inst, 0)
			if hwnd == 0 {
				return err
			}
			pShowWindow.Call(hwnd, SW_SHOW)
			name := active.Name
			if name == "" {
				name = manifest.Name
			}
			pet := NewPet(fmt.Sprintf("%s-%d", active.ID, i+1), name, manifest, store, a.ScreenW, a.ScreenH, frameW, frameH)
			wp := &WindowPet{HWND: hwnd, Pet: pet, Scale: scale, FrameW: frameW, FrameH: frameH}
			if err := wp.initBitmap(frameW, frameH); err != nil {
				pDestroyWindow.Call(hwnd)
				return err
			}
			a.Pets = append(a.Pets, wp)
			log.Printf("created window hwnd=%d pet=%s instance=%s frame=%dx%d scale=%.2f start=(%.0f,%.0f)", hwnd, manifest.ID, pet.InstanceID, frameW, frameH, scale, pet.X, pet.Y)
		}
	}
	if len(a.Pets) == 0 {
		return fmt.Errorf("no pet windows created; profile groups=%d assets=%q", len(a.Profile.ActivePets), assetsPath)
	}
	return nil
}

func initLog() {
	log.SetFlags(log.LstdFlags | log.Lmicroseconds | log.Lshortfile)
	exePath, err := os.Executable()
	logPath := "pet-lite.log"
	if err == nil && exePath != "" {
		logPath = filepath.Join(filepath.Dir(exePath), "pet-lite.log")
	}
	f, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err == nil {
		log.SetOutput(io.MultiWriter(os.Stderr, f))
		log.Printf("log opened path=%s", logPath)
		return
	}
	log.SetOutput(os.Stderr)
	log.Printf("failed to open log file %s: %v", logPath, err)
}

func fatalExit(code int, format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	log.Print(msg)
	showErrorDialog("Desktop Pet Lite", msg)
	os.Exit(code)
}

func showErrorDialog(title string, message string) {
	if message == "" {
		return
	}
	titlePtr, titleErr := syscall.UTF16PtrFromString(title)
	messagePtr, messageErr := syscall.UTF16PtrFromString(message)
	if titleErr != nil || messageErr != nil {
		return
	}
	pMessageBoxW.Call(0, uintptr(unsafe.Pointer(messagePtr)), uintptr(unsafe.Pointer(titlePtr)), MB_OK|MB_ICONERROR)
}

func (wp *WindowPet) initBitmap(w, h int) error {
	sdc, _, err := pGetDC.Call(0)
	if sdc == 0 {
		return fmt.Errorf("GetDC(0) failed: %w", err)
	}
	dc, _, err := pCreateCompatibleDC.Call(sdc)
	if dc == 0 {
		pReleaseDC.Call(0, sdc)
		return fmt.Errorf("CreateCompatibleDC failed: %w", err)
	}
	var bits uintptr
	bi := bitmapInfo{}
	bi.Header.Size = uint32(unsafe.Sizeof(bitmapInfoHeader{}))
	bi.Header.Width = int32(w)
	bi.Header.Height = -int32(h)
	bi.Header.Planes = 1
	bi.Header.BitCount = 32
	bi.Header.Compression = BI_RGB
	hbm, _, err := pCreateDIBSection.Call(dc, uintptr(unsafe.Pointer(&bi)), DIB_RGB_COLORS, uintptr(unsafe.Pointer(&bits)), 0, 0)
	pReleaseDC.Call(0, sdc)
	if hbm == 0 {
		pDeleteDC.Call(dc)
		return fmt.Errorf("CreateDIBSection failed: %w", err)
	}
	old, _, err := pSelectObject.Call(dc, hbm)
	if old == 0 {
		pDeleteObject.Call(hbm)
		pDeleteDC.Call(dc)
		return err
	}
	wp.DC, wp.Bitmap, wp.OldBitmap, wp.Bits = dc, hbm, old, bits
	return nil
}

func (wp *WindowPet) Destroy() {
	if wp == nil {
		return
	}
	if wp.DC != 0 && wp.OldBitmap != 0 {
		pSelectObject.Call(wp.DC, wp.OldBitmap)
		wp.OldBitmap = 0
	}
	if wp.Bitmap != 0 {
		pDeleteObject.Call(wp.Bitmap)
		wp.Bitmap = 0
	}
	if wp.DC != 0 {
		pDeleteDC.Call(wp.DC)
		wp.DC = 0
	}
	wp.Bits = 0
}

func (a *App) findPet(hwnd uintptr) *WindowPet {
	a.Mu.RLock()
	defer a.Mu.RUnlock()
	for _, p := range a.Pets {
		if p.HWND == hwnd {
			return p
		}
	}
	return nil
}
func (a *App) runHook(command string) {
	if command != "" {
		go func() {
			_ = exec.Command("powershell", "-NoProfile", "-ExecutionPolicy", "Bypass", "-Command", command).Start()
		}()
	}
}

func emitInput(ev InputEvent) {
	if app == nil || app.Inputs == nil {
		return
	}
	select {
	case app.Inputs <- ev:
	default:
		log.Printf("input queue full; dropped event kind=%d hwnd=%d reason=%s", ev.Kind, ev.HWND, ev.Reason)
	}
}

// processInputEvents is intentionally executed by the render goroutine, not by wndProc.
// This keeps all pet state mutations on one thread and avoids crashes when several pet windows receive clicks quickly.
func (a *App) processInputEvents() {
	for i := 0; i < 64; i++ {
		select {
		case ev := <-a.Inputs:
			a.handleInputEvent(ev)
		default:
			return
		}
	}
}

func (a *App) handleInputEvent(ev InputEvent) {
	p := a.findPet(ev.HWND)
	if p == nil || p.Pet == nil {
		log.Printf("input ignored: no pet for hwnd=%d kind=%d reason=%s", ev.HWND, ev.Kind, ev.Reason)
		return
	}
	switch ev.Kind {
	case InputLeftDown:
		a.cancelOtherLeftInputs(p, "new_left_down")
		a.beginPendingLeft(p, ev.X, ev.Y, ev.Reason)
	case InputLeftUp:
		if p.Drag {
			a.endDrag(p, "left_up", false)
		} else if p.PendingLeft {
			a.endPendingLeft(p, "left_up_click", false)
			p.Pet.TriggerAction("left_click")
			a.runHook(a.ClickCommand)
		}
	case InputLeftDouble:
		log.Printf("double click hwnd=%d pet=%s; cancel pending/drag and play left_click", ev.HWND, p.Pet.InstanceID)
		a.cancelLeftInput(p, "double_click", false)
		p.Pet.TriggerAction("left_click")
		a.runHook(a.ClickCommand)
	case InputRightDown:
		log.Printf("right click hwnd=%d pet=%s", ev.HWND, p.Pet.InstanceID)
		a.cancelLeftInput(p, "right_click", false)
		p.Pet.TriggerAction("right_click")
		a.runHook(a.RightCommand)
	case InputCancel:
		a.cancelLeftInput(p, ev.Reason, false)
	}
}

func (a *App) loop() {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("panic in render loop: %v\n%s", r, debug.Stack())
		}
	}()
	last := time.Now()
	ticker := time.NewTicker(time.Second / 60)
	defer ticker.Stop()
	for range ticker.C {
		a.processInputEvents()
		now := time.Now()
		dt := now.Sub(last).Seconds()
		if dt > 0.05 {
			dt = 0.05
		}
		last = now
		a.Mu.RLock()
		petsSnapshot := append([]*WindowPet(nil), a.Pets...)
		a.Mu.RUnlock()
		for _, wp := range petsSnapshot {
			// Click/drag is handled without Win32 mouse capture.
			// SetCapture/ReleaseCapture caused WM_CAPTURECHANGED re-entrancy and app exits when several pet windows were active.
			if wp.PendingLeft && !wp.Drag {
				if !isLeftButtonDown() {
					log.Printf("pending click safety release: left button is not down hwnd=%d pet=%s", wp.HWND, wp.Pet.InstanceID)
					a.endPendingLeft(wp, "safety_left_button_up", false)
					wp.Pet.Update(dt, a.ScreenW, wp.FrameW)
					a.drawPet(wp)
					continue
				}
				var pt point
				pGetCursorPos.Call(uintptr(unsafe.Pointer(&pt)))
				dx := absInt(int(pt.X - wp.DownScreenX))
				dy := absInt(int(pt.Y - wp.DownScreenY))
				if dx+dy >= 10 || time.Since(wp.DownAt) >= 180*time.Millisecond {
					a.beginDrag(wp, wp.DragOffX, wp.DragOffY, "pending_promoted")
				}
			}
			if wp.Drag {
				if !isLeftButtonDown() {
					log.Printf("drag safety release: left button is not down hwnd=%d pet=%s", wp.HWND, wp.Pet.InstanceID)
					a.endDrag(wp, "safety_left_button_up", false)
					wp.Pet.Update(dt, a.ScreenW, wp.FrameW)
					a.drawPet(wp)
					continue
				}
				var pt point
				pGetCursorPos.Call(uintptr(unsafe.Pointer(&pt)))
				wp.Pet.X = float64(int(pt.X) - int(wp.DragOffX))
				wp.Pet.Y = float64(int(pt.Y) - int(wp.DragOffY))
				if wp.Pet.X < 0 {
					wp.Pet.X = 0
				}
				if wp.Pet.Y < 0 {
					wp.Pet.Y = 0
				}
				if wp.Pet.X > float64(a.ScreenW-wp.FrameW) {
					wp.Pet.X = float64(a.ScreenW - wp.FrameW)
				}
				if wp.Pet.Y > float64(a.ScreenH-wp.FrameH) {
					wp.Pet.Y = float64(a.ScreenH - wp.FrameH)
				}
				wp.Pet.UpdateDragEmotion()
				wp.Pet.advanceFrame(dt)
			} else {
				wp.Pet.Update(dt, a.ScreenW, wp.FrameW)
			}
			a.drawPet(wp)
		}
	}
}

func (a *App) drawPet(wp *WindowPet) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("draw panic pet=%s anim=%s frame=%d err=%v", wp.Pet.InstanceID, wp.Pet.Animation, wp.Pet.Frame, r)
		}
	}()
	strip, srcX, srcY, fw, fh, err := wp.Pet.Store.FrameRect(wp.Pet.Animation, wp.Pet.Frame)
	if err != nil {
		log.Printf("frame lookup failed pet=%s anim=%s frame=%d err=%v", wp.Pet.InstanceID, wp.Pet.Animation, wp.Pet.Frame, err)
		wp.Pet.setAnimation(wp.Pet.Manifest.DefaultAnimation)
		return
	}
	stride := wp.FrameW * 4
	buf := unsafe.Slice((*byte)(unsafe.Pointer(wp.Bits)), wp.FrameH*stride)
	for y := 0; y < wp.FrameH; y++ {
		for x := 0; x < wp.FrameW; x++ {
			sx := srcX + int(float64(x)/wp.Scale)
			sy := srcY + int(float64(y)/wp.Scale)
			if sx >= srcX+fw {
				sx = srcX + fw - 1
			}
			if sy >= srcY+fh {
				sy = srcY + fh - 1
			}
			if shouldFlip(wp.Pet.Manifest.Animations[wp.Pet.Animation].NativeFacing, wp.Pet.Facing) {
				sx = srcX + fw - 1 - (sx - srcX)
			}
			si := strip.Image.PixOffset(sx, sy)
			di := y*stride + x*4
			r := strip.Image.Pix[si]
			g := strip.Image.Pix[si+1]
			b := strip.Image.Pix[si+2]
			al := strip.Image.Pix[si+3]
			buf[di], buf[di+1], buf[di+2], buf[di+3] = b, g, r, al
		}
	}
	screen, _, _ := pGetDC.Call(0)
	ptDst := point{int32(wp.Pet.X), int32(wp.Pet.Y)}
	sz := size{int32(wp.FrameW), int32(wp.FrameH)}
	ptSrc := point{0, 0}
	blend := blendFunction{AC_SRC_OVER, 0, 255, AC_SRC_ALPHA}
	pUpdateLayeredWindow.Call(wp.HWND, screen, uintptr(unsafe.Pointer(&ptDst)), uintptr(unsafe.Pointer(&sz)), wp.DC, uintptr(unsafe.Pointer(&ptSrc)), 0, uintptr(unsafe.Pointer(&blend)), ULW_ALPHA)
	pReleaseDC.Call(0, screen)
}

func wndProc(hwnd uintptr, msg uint32, wParam, lParam uintptr) uintptr {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("panic in wndProc msg=%d hwnd=%d err=%v\n%s", msg, hwnd, r, debug.Stack())
		}
	}()
	switch msg {
	case WM_LBUTTONDOWN:
		mx := int32(int16(lParam & 0xffff))
		my := int32(int16((lParam >> 16) & 0xffff))
		emitInput(InputEvent{Kind: InputLeftDown, HWND: hwnd, X: mx, Y: my, Reason: "left_down"})
		return 0
	case WM_LBUTTONDBLCLK:
		emitInput(InputEvent{Kind: InputLeftDouble, HWND: hwnd, Reason: "double_click"})
		return 0
	case WM_LBUTTONUP:
		emitInput(InputEvent{Kind: InputLeftUp, HWND: hwnd, Reason: "left_up"})
		return 0
	case WM_RBUTTONDOWN:
		emitInput(InputEvent{Kind: InputRightDown, HWND: hwnd, Reason: "right_click"})
		return 0
	case WM_CAPTURECHANGED:
		emitInput(InputEvent{Kind: InputCancel, HWND: hwnd, Reason: "capture_changed"})
		return 0
	case WM_CANCELMODE:
		emitInput(InputEvent{Kind: InputCancel, HWND: hwnd, Reason: "cancel_mode"})
		return 0
	case WM_CLOSE:
		// Desktop-pet windows do not have a close affordance. During rapid drag/double-click
		// across multiple layered windows Windows can still deliver WM_CLOSE via shell/system
		// paths; treating it with DefWindowProc destroys the pet and can quit the whole app.
		log.Printf("WM_CLOSE ignored hwnd=%d", hwnd)
		return 0
	case WM_DESTROY:
		log.Printf("WM_DESTROY hwnd=%d", hwnd)
		if app != nil {
			remaining := app.removePetWindow(hwnd)
			if remaining == 0 {
				log.Printf("all pet windows destroyed; posting quit")
				pPostQuitMessage.Call(0)
			}
		} else {
			pPostQuitMessage.Call(0)
		}
		return 0
	}
	ret, _, _ := pDefWindowProcW.Call(hwnd, uintptr(msg), wParam, lParam)
	return ret
}

func (a *App) beginPendingLeft(wp *WindowPet, mx, my int32, reason string) {
	if wp == nil || wp.Pet == nil {
		return
	}
	if wp.PendingLeft || wp.Drag {
		log.Printf("beginPendingLeft resets previous input hwnd=%d pet=%s reason=%s pending=%v drag=%v", wp.HWND, wp.Pet.InstanceID, reason, wp.PendingLeft, wp.Drag)
	}
	var pt point
	pGetCursorPos.Call(uintptr(unsafe.Pointer(&pt)))
	wp.PendingLeft = true
	wp.Drag = false
	wp.DragOffX = mx
	wp.DragOffY = my
	wp.DownScreenX = pt.X
	wp.DownScreenY = pt.Y
	wp.DownAt = time.Now()
	log.Printf("pending left begin hwnd=%d pet=%s reason=%s offset=(%d,%d) screen=(%d,%d) pos=(%.0f,%.0f)", wp.HWND, wp.Pet.InstanceID, reason, mx, my, pt.X, pt.Y, wp.Pet.X, wp.Pet.Y)
}

func (a *App) endPendingLeft(wp *WindowPet, reason string, releaseCapture bool) {
	if wp == nil || wp.Pet == nil || !wp.PendingLeft {
		return
	}
	wp.PendingLeft = false
	log.Printf("pending left end hwnd=%d pet=%s reason=%s pos=(%.0f,%.0f)", wp.HWND, wp.Pet.InstanceID, reason, wp.Pet.X, wp.Pet.Y)
}

func (a *App) cancelLeftInput(wp *WindowPet, reason string, releaseCapture bool) {
	if wp == nil || wp.Pet == nil {
		return
	}
	if wp.Drag {
		a.endDrag(wp, reason, releaseCapture)
		return
	}
	if wp.PendingLeft {
		a.endPendingLeft(wp, reason, releaseCapture)
	}
}

func (a *App) cancelOtherLeftInputs(active *WindowPet, reason string) {
	if active == nil {
		return
	}
	a.Mu.RLock()
	petsSnapshot := append([]*WindowPet(nil), a.Pets...)
	a.Mu.RUnlock()
	for _, wp := range petsSnapshot {
		if wp == nil || wp == active || wp.HWND == active.HWND || wp.Pet == nil {
			continue
		}
		if wp.PendingLeft || wp.Drag || wp.Pet.DragMode {
			log.Printf("cancel stale left input hwnd=%d pet=%s active_hwnd=%d reason=%s", wp.HWND, wp.Pet.InstanceID, active.HWND, reason)
			a.cancelLeftInput(wp, reason, false)
		}
	}
}

func (a *App) beginDrag(wp *WindowPet, mx, my int32, reason string) {
	if wp == nil || wp.Pet == nil {
		return
	}
	if wp.Drag {
		log.Printf("beginDrag ignored because already dragging hwnd=%d pet=%s reason=%s", wp.HWND, wp.Pet.InstanceID, reason)
		return
	}
	wp.PendingLeft = false
	wp.Drag = true
	wp.DragOffX = mx
	wp.DragOffY = my
	wp.Pet.StartDrag()
	log.Printf("drag begin hwnd=%d pet=%s reason=%s offset=(%d,%d) pos=(%.0f,%.0f)", wp.HWND, wp.Pet.InstanceID, reason, mx, my, wp.Pet.X, wp.Pet.Y)
}

func (a *App) endDrag(wp *WindowPet, reason string, releaseCapture bool) {
	if wp == nil || wp.Pet == nil {
		return
	}
	if !wp.Drag && !wp.Pet.DragMode {
		return
	}
	wp.Drag = false
	wp.PendingLeft = false
	wp.Pet.EndDrag()
	log.Printf("drag end hwnd=%d pet=%s reason=%s pos=(%.0f,%.0f)", wp.HWND, wp.Pet.InstanceID, reason, wp.Pet.X, wp.Pet.Y)
}

func (a *App) removePetWindow(hwnd uintptr) int {
	a.Mu.Lock()
	defer a.Mu.Unlock()
	for i, p := range a.Pets {
		if p.HWND == hwnd {
			log.Printf("remove window hwnd=%d pet=%s", hwnd, p.Pet.InstanceID)
			p.Destroy()
			a.Pets = append(a.Pets[:i], a.Pets[i+1:]...)
			return len(a.Pets)
		}
	}
	return len(a.Pets)
}

func isLeftButtonDown() bool {
	r, _, _ := pGetAsyncKeyState.Call(VK_LBUTTON)
	return int16(r&0xffff) < 0
}

func absInt(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

func shouldFlip(nativeFacing string, facing int) bool {
	if nativeFacing == "left" {
		return facing > 0
	}
	return facing < 0
}

func messageLoop() {
	var m msg
	log.Printf("message loop start")
	for {
		r, _, _ := pGetMessageW.Call(uintptr(unsafe.Pointer(&m)), 0, 0, 0)
		if int32(r) <= 0 {
			log.Printf("message loop stop code=%d", int32(r))
			break
		}
		pTranslateMessage.Call(uintptr(unsafe.Pointer(&m)))
		pDispatchMessageW.Call(uintptr(unsafe.Pointer(&m)))
	}
}
func getSystemMetrics(i int) int32 { r, _, _ := pGetSystemMetrics.Call(uintptr(i)); return int32(r) }
