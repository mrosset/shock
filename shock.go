package main

import (
	"bufio"
	"container/list"
	"container/vector"
	"flag"
	"fmt"
	"http"
	"io"
	"log"
	"net"
	"os"
	"path/filepath"
	"rpc"
	"strings"
	"time"
)

const (
	SECONDS = 1e09
	MINUTES = SECONDS * 60
	HOURS   = MINUTES * 60
	DAYS    = HOURS * 24
)

var (
	isServer   = flag.Bool("s", false, "start in server mode")
	isClient   = flag.Bool("c", true, "start in client mode")
	isReciever = flag.Bool("r", false, "recieve message count")
	isVerbose  = flag.Bool("v", false, "reciver use more verbose output")
	ts         = new(TaskServer)
	con        = &connection{"unix", "/tmp/shock"}
	stop       = make(chan bool)
	alerts     = new(Notify)
	taskConfig = filepath.Join(os.Getenv("HOME"), ".shock/tasks")
	//con        = &connection{"tcp", "localhost:8080"}
	//socket     = "localhost:8080"
)

func init() {
	log.SetFlags(log.Lshortfile)
}

func main() {
	flag.Parse()
	if *isServer {
		lfile, err := os.Create(filepath.Join(os.Getenv("HOME"), ".shock.log"))
		if err != nil {
			log.Fatal(err)
		}
		defer lfile.Close()
		output := io.MultiWriter(lfile, alerts)
		log.SetOutput(output)
		server()
		os.Exit(0)
	}
	if *isReciever {
		recieve()
		os.Exit(0)
	}
	if *isClient {
		client()
	}
}

type connection struct {
	net    string
	socket string
}

func server() {
	if con.net == "unix" {
		os.Remove(con.socket)
	}
	rpc.Register(alerts)
	rpc.HandleHTTP()
	l, e := net.Listen(con.net, con.socket)
	if e != nil {
		log.Fatal(e)
	}
	go http.Serve(l, nil)
	if err := ts.LoadTasks(); err != nil {
		fmt.Print(err)
		log.Fatal(err)
	}
	log.Println("server started")
	ts.Run()
}

func recieve() {
	client, err := rpc.DialHTTP(con.net, con.socket)
	if err != nil {
		log.Fatal(err)
	}
	var message string
	err = client.Call("Notify.Last", *isVerbose, &message)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(message)
	client.Close()
}

func client() {
	if len(flag.Args()) == 0 {
		log.Fatal("no arguments")
	}
	client, err := rpc.DialHTTP(con.net, con.socket)
	if err != nil {
		log.Fatal(err)
	}
	command := flag.Args()[0]
	m := strings.Join(flag.Args()[1:], " ")
	var results []string
	err = client.Call(command, m, &results)
	if err != nil {
		log.Fatal(err)
	}
	if results != nil && len(results) > 0 {
		for _, v := range results {
			fmt.Printf("%v\n", v)
		}
	}
	client.Close()
}

type TaskServer struct {
	list.List
}

func (v *TaskServer) Tasks() []string {
	vector := new(vector.StringVector)
	for e := v.Front(); e != nil; e = e.Next() {
		switch t := e.Value.(type) {
		case Task:
			vector.Push(fmt.Sprintf("%v", t))
		default:
			vector.Push(fmt.Sprintf("%T is unknown", t))
		}
	}
	return vector.Copy()
}

// Pulse all tasks once a second
func (v *TaskServer) Run() {
	tick := time.Tick(1e09)
	for {
		select {
		case <-tick:
			v.Pulse()
		}
	}
}

// If we recieve a Tick from a Task run its Task
func (v *TaskServer) Pulse() {
	for e := v.Front(); e != nil; e = e.Next() {
		switch t := e.Value.(type) {
		case Task:
			select {
			case <-t.Tick():
				if !t.IsRunning() {
					go func() {
						if err := t.Run(); err != nil {
							log.Printf("error running %v %v", t, err)
						}
					}()
				}
			default:
			}
		default:
			log.Printf(fmt.Sprintf("%T is unknown", t))
		}
	}
}

func (v *TaskServer) LoadTasks() os.Error {
	f, err := os.Open(taskConfig)
	if err != nil {
		return err
	}
	defer f.Close()
	buf := bufio.NewReader(f)
	for {
		line, _, err := buf.ReadLine()
		if err == os.EOF {
			break
		}
		if err != nil {
			return err
		}
		if line[0] != '#' {
			t := new(Shell)
			_, err := fmt.Sscanf(string(line), "%d %s %s %s %s\n", &t.Interval, &t.Label, &t.Command, &t.Args, &t.Path)
			if err != nil {
				return err
			}
			t.Interval = t.Interval * MINUTES
			t.tick = time.Tick(t.Interval)
			t.Args = strings.Replace(t.Args, ",", " ", -1)
			go func() {
				err := t.Run()
				if err != nil {
					log.Print(err)
				}
			}()
			v.PushFront(t)
		}
	}
	return nil
}

func (v *TaskServer) SaveTasks() os.Error {
	f, err := os.Create(taskConfig)
	if err != nil {
		return err
	}
	defer f.Close()

	header :=
		`# shell arguements should have spaces escaped with a , if there are no arguements use nil
#
# minutes label command comma,delimited,arguement path
`

	f.Write([]byte(header))
	for e := v.Front(); e != nil; e = e.Next() {
		t := e.Value.(*Shell)
		s := fmt.Sprintf("%03d %s %s %s %s\n", t.Interval/MINUTES, t.Label, t.Command, strings.Replace(t.Args, " ", ",", -1), t.Path)
		f.Write([]byte(s))
	}
	return nil
}

type Notice struct {
	label   string
	message string
	read    bool
}

func NewNotice(label, message string) *Notice {
	return &Notice{label: label, message: message}
}

func (v *Notice) String() string {
	return fmt.Sprintf("%-10.10s %-70.70s...", v.label+":", v.message)
}

// Notify
type Notify struct {
	list.List
}

// This extends list.List to iterate and call a function on each Notice
func (v *Notify) Each(f func(*Notice)) os.Error {
	for e := v.Front(); e != nil; e = e.Next() {
		switch t := e.Value.(type) {
		case *Notice:
			f(t)
		default:
			log.Printf("%T = %v", t, t)
		}
	}
	return nil
}

func (v *Notify) Contains(s string) (contains bool) {
	v.Each(
		func(n *Notice) {
			if n.message == s {
				contains = true
			}
		})
	return
}

func (v *Notify) Write(b []byte) (n int, err os.Error) {
	m := NewNotice("shock", string(b[:len(b)-1]))
	v.PushFront(m)
	return 0, nil
}

func (v *Notify) Total(n string, reply *int) os.Error {
	*reply = v.Len()
	return nil
}

// TODO: this method should walk back and find the last unread message
func (v *Notify) Last(verbose *bool, message *string) os.Error {
	count := 0
	v.Each(func(n *Notice) {
		if !n.read {
			count++
		}
	})

	if v.Len() == 0 && *verbose {
		*message = "shock: 0 messages"
		return nil
	}

	n := v.Front().Value.(*Notice)
	if !n.read {
		*message = fmt.Sprintf("%s total: %v last: %s", log.Prefix(), count, n.message)
		return nil
	}
	if *verbose {
		*message = "shock: 0 messages"
	}
	return nil
}

// TODO: impliment cleint pushing
func (v *Notify) Push(m string, reply *[]string) os.Error {
	notice := new(Notice)
	notice.message = m
	notice.label = "client"
	v.PushFront(notice)
	return nil
}

// returns all unread notices
func (v *Notify) Notices(m string, reply *[]string) os.Error {
	vector := new(vector.StringVector)
	v.Each(func(n *Notice) {
		if !n.read {
			vector.Push(n.String())
		}
	})
	*reply = ([]string)(vector.Copy())
	return nil
}

// rpc call to return tasks
func (v *Notify) Tasks(m string, reply *[]string) os.Error {
	*reply = ts.Tasks()
	return nil
}

// mark all notices read
func (v *Notify) MarkRead(m string, reply *[]string) os.Error {
	v.Each(func(n *Notice) {
		n.read = true
	})
	return nil
}

func (v *Notify) Dump(m string, reply *[]string) os.Error {
	vector := new(vector.StringVector)
	v.Each(func(n *Notice) {
		vector.Push(fmt.Sprintf("%#v", n))
	})
	*reply = ([]string)(vector.Copy())
	return nil
}

// stop the server
func (n *Notify) StopServer(m string, reply *[]string) os.Error {
	stop <- true
	return nil
}

// Task
type Task interface {
	IsRunning() bool
	Run() os.Error
	String() string
	Tick() <-chan int64
}
