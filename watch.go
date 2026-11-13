package main

// The kernel tells the store what changed, through Linux's inotify, so a
// request never has to look. The descriptor is non-blocking and is drained
// at the start of a request. The kernel queues an event before the call
// that caused it returns, so a request made after a write has finished
// always hears of it. This file is Linux only, like the server.

import (
	"bytes"
	"errors"
	"fmt"
	"syscall"
	"unsafe"
)

// A change is one event: the watch it came from, the name in that folder
// ("" for the folder itself), and what happened.
type change struct {
	wd   int32
	name string
	mask uint32
}

type watcher struct {
	fd  int
	buf [64 * 1024]byte // one read's events; drain is called under the store's syncMu
}

// The events that can change what a file holds or whether it is there. A
// rename into the folder is how many editors and git save a file.
const watchMask = syscall.IN_CREATE | syscall.IN_CLOSE_WRITE | syscall.IN_MODIFY | syscall.IN_ATTRIB |
	syscall.IN_MOVED_FROM | syscall.IN_MOVED_TO | syscall.IN_DELETE |
	syscall.IN_DELETE_SELF | syscall.IN_MOVE_SELF | syscall.IN_ONLYDIR

// The events that say the watched folder itself is gone or moved.
const selfMask = syscall.IN_DELETE_SELF | syscall.IN_MOVE_SELF | syscall.IN_IGNORED

func newWatcher() (*watcher, error) {
	fd, err := syscall.InotifyInit1(syscall.IN_NONBLOCK | syscall.IN_CLOEXEC)
	if err != nil {
		return nil, fmt.Errorf("inotify: %w", err)
	}
	return &watcher{fd: fd}, nil
}

// add watches a folder. Watching a folder already watched gives its watch
// again.
func (w *watcher) add(dir string) (int32, error) {
	wd, err := syscall.InotifyAddWatch(w.fd, dir, watchMask)
	if err != nil {
		return -1, fmt.Errorf("watching %s: %w", dir, err)
	}
	return int32(wd), nil
}

func (w *watcher) remove(wd int32) {
	syscall.InotifyRmWatch(w.fd, uint32(wd))
}

func (w *watcher) close() error { return syscall.Close(w.fd) }

// drain gives every change queued since the last drain. overflow says the
// kernel's queue filled and some changes were dropped, so the caller must
// read everything again.
func (w *watcher) drain() (changes []change, overflow bool, err error) {
	for {
		n, err := syscall.Read(w.fd, w.buf[:])
		if errors.Is(err, syscall.EAGAIN) {
			return changes, overflow, nil
		}
		if errors.Is(err, syscall.EINTR) {
			continue
		}
		if err != nil {
			return changes, overflow, fmt.Errorf("inotify read: %w", err)
		}
		for off := 0; off+syscall.SizeofInotifyEvent <= n; {
			ev := (*syscall.InotifyEvent)(unsafe.Pointer(&w.buf[off]))
			start := off + syscall.SizeofInotifyEvent
			off = start + int(ev.Len)
			if ev.Mask&syscall.IN_Q_OVERFLOW != 0 {
				overflow = true
				continue
			}
			name := bytes.TrimRight(w.buf[start:off], "\x00")
			changes = append(changes, change{wd: ev.Wd, name: string(name), mask: ev.Mask})
		}
	}
}
