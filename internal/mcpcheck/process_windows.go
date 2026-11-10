//go:build windows

package mcpcheck

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

func startProcess(cmd *exec.Cmd) (func() error, error) {
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: windows.CREATE_NO_WINDOW | windows.CREATE_SUSPENDED}
	ext := strings.ToLower(filepath.Ext(cmd.Path))
	if ext == ".cmd" || ext == ".bat" {
		var quoted []string
		for _, arg := range append([]string{cmd.Path}, cmd.Args[1:]...) {
			if strings.ContainsAny(arg, "\"%\r\n") {
				return nil, fmt.Errorf("批处理参数包含不支持的字符，请改用对应的可执行文件（例如 node.exe）")
			}
			quoted = append(quoted, `"`+arg+`"`)
		}
		cmd.Path = filepath.Join(os.Getenv("SystemRoot"), "System32", "cmd.exe")
		cmd.SysProcAttr.CmdLine = `"` + cmd.Path + `" /D /V:OFF /S /C "` + strings.Join(quoted, " ") + `"`
	}
	job, err := windows.CreateJobObject(nil, nil)
	if err != nil {
		return nil, err
	}
	limits := windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION{}
	limits.BasicLimitInformation.LimitFlags = windows.JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE
	_, err = windows.SetInformationJobObject(job, windows.JobObjectExtendedLimitInformation, uintptr(unsafe.Pointer(&limits)), uint32(unsafe.Sizeof(limits)))
	if err != nil {
		windows.CloseHandle(job)
		return nil, err
	}
	if err = cmd.Start(); err != nil {
		windows.CloseHandle(job)
		return nil, err
	}
	fail := func(err error) (func() error, error) {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		windows.CloseHandle(job)
		return nil, err
	}
	process, err := windows.OpenProcess(windows.PROCESS_SET_QUOTA|windows.PROCESS_TERMINATE|windows.PROCESS_SUSPEND_RESUME, false, uint32(cmd.Process.Pid))
	if err != nil {
		return fail(err)
	}
	defer windows.CloseHandle(process)
	if err = windows.AssignProcessToJobObject(job, process); err != nil {
		return fail(err)
	}
	// Assign the suspended process before it can spawn children; all descendants
	// inherit the job and are terminated when the last job handle closes.
	resume := windows.NewLazySystemDLL("ntdll.dll").NewProc("NtResumeProcess")
	if err = resume.Find(); err != nil {
		return fail(err)
	}
	status, _, _ := resume.Call(uintptr(process))
	if status != 0 {
		return fail(fmt.Errorf("NtResumeProcess: 0x%x", status))
	}
	return func() error { return windows.CloseHandle(job) }, nil
}
