package main

import (
	"exec"
	"fmt"
	"log"
	"os"
	"strings"
	"time"
)

type Shell struct {
	running  bool
	tick     <-chan int64
	Interval int64
	Command  string
	Label    string
	Path     string
	Args     string
}

func NewShell(tick int64, label, command, args, path string) *Shell {
	g := &Shell{
		tick:     time.Tick(tick),
		Interval: tick,
		Label:    label,
		Command:  command,
		Args:     args,
		Path:     path,
	}
	return g
}

func (v *Shell) Tick() <-chan int64 {
	return v.tick
}

func (v *Shell) Run() os.Error {
	v.running = true
	defer func() { v.running = false }()
	cmd := exec.Command(v.Command, strings.Fields(v.Args)...)
	if cmd.Args[0] == "nil" {
		cmd.Args = nil
	}
	cmd.Dir = v.Path
	output, err := cmd.Output()
	if err != nil {
		log.Print(err.String() + string(output))
		return os.NewError(err.String() + string(output))
	}
	if string(output) != "" {
		lines := strings.Split(string(output), "\n", -1)
		if !alerts.Contains(lines[0]) {
			alerts.PushFront(NewNotice(v.Label, lines[0]))
		}
	}
	return nil
}

func (v *Shell) String() string {
	return fmt.Sprintf("%-10.10s %-20.20s in %-20.20s status %-20.20v", v.Label, v.Command, v.Path, v.running)
}

func (v *Shell) IsRunning() bool {
	return v.running
}
