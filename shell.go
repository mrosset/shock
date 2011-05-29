package main

import (
	"exec"
	"fmt"
	"os"
	"strings"
	"time"
)

type Shell struct {
	running bool
	tick    <-chan int64
	name    string
	path    string
	args    string
}

func NewShell(tick int64, name, args, path string) *Shell {
	g := &Shell{
		tick: time.Tick(tick),
		name: name,
		args: args,
		path: path,
	}
	return g
}

func (v *Shell) Tick() <-chan int64 {
	return v.tick
}

func (v *Shell) Run() os.Error {
	v.running = true
	defer func() { v.running = false }()
	cmd := exec.Command(v.name, strings.Fields(v.args)...)
	cmd.Dir = v.path
	output, err := cmd.Output()
	if err != nil {
		return os.NewError(err.String() + output)
	}
	if output != "" {
		line := strings.Split(output, "\n", -1)
		msg := fmt.Sprintf("%v %v more", line[0], len(line))
		if !contains(msg) {
			alerts.queue.Push(msg)
		}
	}
	return nil
}

func (v *Shell) String() string {
	return fmt.Sprintf("%s in %s", v.name, v.path)
}

func (v *Shell) IsRunning() bool {
	return v.running
}
