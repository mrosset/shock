package main

import (
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
	SECOND = 1e09
	MINUTE = SECOND * 60
	HOUR   = MINUTE * 60
	DAY    = HOUR * 24
)

var (
	isServer   = flag.Bool("s", false, "start in server mode")
	isClient   = flag.Bool("c", true, "start in client mode")
	isReciever = flag.Bool("r", false, "recieve message count")
	isDump     = flag.Bool("d", false, "dump messages")
	ts         = new(TaskServer)
	con        = &connection{"tcp", "localhost:8080"}
	stop       = make(chan bool)
	alerts     = new(Notify)
	//con        = &connection{"unix", "/tmp/shock"}
	//socket     = "localhost:8080"
)

func init() {
	log.SetFlags(log.Lshortfile | log.Ltime)
	log.SetPrefix("shock: ")
}

func main() {
	flag.Parse()
	f, err := os.Create("/home/strings/shock.log")
	output := io.MultiWriter(f, os.Stderr)
	if err != nil {
		fmt.Println(err)
	}
	defer f.Close()

	log.SetOutput(output)

	if *isServer {
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
	ts.PushFront(NewShell(MINUTE*30, "git", "fetch", "/home/strings/go"))
	ts.PushFront(NewShell(SECOND*30, "bti", "--user mikerosset --action friends", "/home/strings/go"))
	repos, err := filepath.Glob("/home/strings/github/*")
	if err != nil {
		log.Print(err)
	}
	for _, r := range repos {
		if filepath.Base(r)[0] != '.' {
			log.Println("adding", r)
			st := NewShell(MINUTE*30, "git", "fetch", r)
			ts.PushFront(st)
			go st.Run()
		}
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
	err = client.Call("Notify.Last", "", &message)
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

// Tasks 
type TaskServer struct {
	list.List
}

func (v *TaskServer) Tasks() []string {
	vector := new(vector.StringVector)
	for e := v.Front(); e != nil; e = e.Next() {
		switch t := e.Value.(type) {
		case Task:
			vector.Push(fmt.Sprintf("%-40.40v running: %v", t, t.IsRunning()))
		default:
			vector.Push(fmt.Sprintf("%T is unknown", t))
		}
	}
	return vector.Copy()
}


// Pulse all tasks once a Second
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
				log.Printf("running %v", t)
				if !t.IsRunning() {
					go func() {
						if err := t.Run(); err != nil {
							log.Printf("error running %v %v", t, err)
						}
					}()
				}
			default:
				//log.Printf("derp %v", t)
			}
		default:
			log.Printf(fmt.Sprintf("%T is unknown", t))
		}
	}
}

// Notify
type Notify struct {
	queue vector.StringVector
}

func (v *Notify) Len(n string, reply *int) os.Error {
	*reply = v.queue.Len()
	return nil
}

func (v *Notify) Last(n string, message *string) os.Error {
	if v.queue.Len() == 0 {
		*message = "shock: 0 messages"
		return nil
	}
	*message = fmt.Sprintf("%s total: %v last: %s", log.Prefix(), v.queue.Len(), v.queue.Last())
	return nil
}

func (n *Notify) Push(m string, reply *[]string) os.Error {
	n.queue.Push(m)
	return nil
}

func (n *Notify) List(m string, reply *[]string) os.Error {
	*reply = ([]string)(alerts.queue.Copy())
	return nil
}

func (n *Notify) Tasks(m string, reply *[]string) os.Error {
	*reply = ts.Tasks()
	return nil
}

func (n *Notify) MarkRead(m string, reply *bool) os.Error {
	alerts.queue.Cut(0, alerts.queue.Len())
	return nil
}

func (n *Notify) StopServer(m string, reply *bool) os.Error {
	stop <- true
	return nil
}

func (n *Notify) Test(m string, reply *[]string) os.Error {
	for i := 0; i < 100; i++ {
		alerts.queue.Push(fmt.Sprintf("%v", i))
	}
	return nil
}

// Task
type Task interface {
	IsRunning() bool
	Run() os.Error
	String() string
	Tick() <-chan int64
}

func contains(s string) bool {
	var c bool
	alerts.queue.Do(func(e string) {
		if e == s {
			c = true
		}
	})
	return c
}
