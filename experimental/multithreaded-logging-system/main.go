package main

import (
	"flag"
	"fmt"
	"log"
	"math/rand"
	"runtime"
	"sync"
	"time"

	"github.com/pebbe/zmq4"
)

const endpoint = "inproc://log-pipeline"

var phrases = []string{
	"processing job 42",
	"finished task",
	"error retry",
	"waiting on downstream service",
	"flushed buffered events",
}

func main() {
	workerCount := flag.Int("workers", 3, "number of worker goroutines")
	messageCount := flag.Int("messages", 5, "messages sent by each worker")
	maxDelayMs := flag.Int("delay-ms", 500, "maximum random delay between messages in milliseconds")
	flag.Parse()

	ctx, err := zmq4.NewContext()
	if err != nil {
		log.Fatalf("create zmq context: %v", err)
	}
	defer ctx.Term()

	log.Printf("starting experiment with %d workers, %d messages each", *workerCount, *messageCount)
	log.Printf("safety rules: one shared context, one socket per goroutine, no shared sockets")

	var loggerReady sync.WaitGroup
	loggerReady.Add(1)

	var loggerWG sync.WaitGroup
	loggerWG.Add(1)
	go loggerLoop(ctx, &loggerWG, &loggerReady, *workerCount**messageCount)

	loggerReady.Wait()

	var workerWG sync.WaitGroup
	for workerID := 1; workerID <= *workerCount; workerID++ {
		workerWG.Add(1)
		go workerLoop(ctx, &workerWG, workerID, *messageCount, *maxDelayMs)
	}

	workerWG.Wait()
	loggerWG.Wait()

	log.Print("experiment complete")
}

func loggerLoop(ctx *zmq4.Context, wg *sync.WaitGroup, ready *sync.WaitGroup, expectedMessages int) {
	defer wg.Done()
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	socket, err := ctx.NewSocket(zmq4.PULL)
	if err != nil {
		log.Fatalf("logger create pull socket: %v", err)
	}
	defer socket.Close()

	if err := socket.Bind(endpoint); err != nil {
		log.Fatalf("logger bind %s: %v", endpoint, err)
	}

	ready.Done()

	for i := 0; i < expectedMessages; i++ {
		msg, err := socket.Recv(0)
		if err != nil {
			log.Fatalf("logger recv: %v", err)
		}
		fmt.Println(msg)
	}
}

func workerLoop(ctx *zmq4.Context, wg *sync.WaitGroup, workerID, messageCount, maxDelayMs int) {
	defer wg.Done()
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	socket, err := ctx.NewSocket(zmq4.PUSH)
	if err != nil {
		log.Fatalf("worker-%d create push socket: %v", workerID, err)
	}
	defer socket.Close()

	if err := socket.Connect(endpoint); err != nil {
		log.Fatalf("worker-%d connect %s: %v", workerID, endpoint, err)
	}

	rng := rand.New(rand.NewSource(time.Now().UnixNano() + int64(workerID)))
	for i := 0; i < messageCount; i++ {
		delay := time.Duration(rng.Intn(maxDelayMs+1)) * time.Millisecond
		time.Sleep(delay)

		line := fmt.Sprintf(
			"%s | worker-%d | %s",
			time.Now().Format("15:04:05.000"),
			workerID,
			phrases[rng.Intn(len(phrases))],
		)

		if _, err := socket.Send(line, 0); err != nil {
			log.Fatalf("worker-%d send: %v", workerID, err)
		}
	}
}
