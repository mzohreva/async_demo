package cmd

import (
	"fmt"
	"log"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/spf13/cobra"
)

var queriesPerSecond uint
var connections uint
var warmupDuration time.Duration
var testDuration time.Duration
var useTLS bool

var loadTestCmd = &cobra.Command{
	Use:     "load-test",
	Aliases: []string{"load"},
	Short:   "A collection of load tests for various types of operations.",
	Long:    "A collection of load tests for various types of operations.",
}

func init() {
	rootCmd.AddCommand(loadTestCmd)

	loadTestCmd.PersistentFlags().UintVar(&queriesPerSecond, "qps", 10, "Queries per second (QPS)")
	loadTestCmd.PersistentFlags().UintVarP(&connections, "connections", "c", 10, "Number of concurrent connections")
	loadTestCmd.PersistentFlags().DurationVarP(&testDuration, "duration", "d", 30*time.Second, "Test duration")
	loadTestCmd.PersistentFlags().DurationVarP(&warmupDuration, "warmup", "w", 10*time.Second, "Warmup duration")
	loadTestCmd.PersistentFlags().BoolVarP(&useTLS, "tls", "t", false, "Use TLS to connect.")
}

type Stage int

const (
	WarmupStage Stage = iota + 1
	TestStage
)

type Connection interface{}

type LoadTest interface {
	Setup() (Connection, error)
	Test(Connection, Stage) (time.Duration, error)
	Cleanup(Connection)
}

// a load test performed over a TCP or TLS connection (no HTTP)
type rawLoadTest struct {
	setupFunc   func(net.Conn) error
	testFunc    func(net.Conn, Stage) (time.Duration, error)
	cleanupFunc func(net.Conn)
}

func (r *rawLoadTest) Setup() (Connection, error) {
	conn := setupRawConnection(useTLS)
	err := r.setupFunc(conn)
	return conn, err
}

func (r *rawLoadTest) Test(conn Connection, stage Stage) (time.Duration, error) {
	netConn := conn.(net.Conn)
	return r.testFunc(netConn, stage)
}

func (r *rawLoadTest) Cleanup(conn Connection) {
	netConn := conn.(net.Conn)
	r.cleanupFunc(netConn)
}

type httpLoadTest struct {
	baseURL     string
	setupFunc   func(*http.Client, string) error
	testFunc    func(*http.Client, string, Stage) (time.Duration, error)
	cleanupFunc func(*http.Client, string)
}

func (h *httpLoadTest) Setup() (Connection, error) {
	conn := setupHttpConnection()
	h.baseURL = serverURL(useTLS)
	err := h.setupFunc(conn, h.baseURL)
	return conn, err
}

func (h *httpLoadTest) Test(conn Connection, stage Stage) (time.Duration, error) {
	client := conn.(*http.Client)
	return h.testFunc(client, h.baseURL, stage)
}

func (h *httpLoadTest) Cleanup(conn Connection) {
	client := conn.(*http.Client)
	h.cleanupFunc(client, h.baseURL)
}

func loadTest(name string, test LoadTest) {
	fmt.Printf("      Load test: %v\n", name)
	fmt.Printf("         Server: %v:%v\n", serverHost, serverPort)
	fmt.Printf("     Target QPS: %v\n", queriesPerSecond)
	fmt.Printf("    Connections: %v\n", connections)
	fmt.Printf("  Test Duration: %v\n", testDuration)
	fmt.Printf("Warmup Duration: %v\n", warmupDuration)
	fmt.Println()
	type testResult struct {
		t time.Time
		d time.Duration
		s Stage
	}
	ticker := time.NewTicker(time.Duration(warmupDuration.Nanoseconds() / int64(connections)))
	start := make(chan struct{})
	end := make(chan struct{})
	result := make(chan testResult, 1000) // buffered channel just in case
	var ready, finished sync.WaitGroup
	var wg1 sync.WaitGroup

	launchWorker := func() {
		callTestFunc := func(t time.Time, conn Connection, stage Stage) {
			d, err := test.Test(conn, stage)
			if err != nil {
				fmt.Printf("Error: %v\n", err)
			}
			result <- testResult{t, d, stage}
		}
		ready.Add(1)
		finished.Add(1)
		wg1.Add(1)
		go func() {
			defer wg1.Done()

			conn, err := test.Setup()
			if err != nil {
				log.Fatal(err)
			}
			callTestFunc(time.Time{}, conn, WarmupStage)
			ready.Done()
			<-start
		testLoop:
			for {
				select {
				case t := <-ticker.C:
					callTestFunc(t, conn, TestStage)
				case <-end:
					break testLoop
				}
			}
			finished.Done()
			test.Cleanup(conn)
		}()
	}

	var wg2 sync.WaitGroup
	wg2.Add(2)
	var warmups, tests []time.Duration
	var lastTick time.Time
	go func() {
		defer wg2.Done()

		for r := range result {
			if r.s == WarmupStage {
				warmups = append(warmups, r.d)
			} else {
				tests = append(tests, r.d)
			}
			lastTick = r.t
		}
	}()

	for i := uint(0); i < connections; i++ {
		<-ticker.C
		launchWorker()
	}
	ticker.Reset(time.Duration(time.Second.Nanoseconds() / int64(queriesPerSecond)))

	ready.Wait()
	t0 := time.Now()
	close(start)
	var t1 time.Time

	go func() {
		defer wg2.Done()

		time.Sleep(testDuration)
		close(end)
		finished.Wait()
		t1 = time.Now()
	}()
	wg1.Wait()
	close(result)
	wg2.Wait()
	ticker.Stop()

	sendDuration := lastTick.Sub(t0)
	testDuration := t1.Sub(t0)

	fmt.Printf("       Warmup: %v queries, %v\n", len(warmups), summarizeTimings(warmups))
	fmt.Printf("         Test: %v queries, %v\n", len(tests), summarizeTimings(tests))
	fmt.Printf("Test duration: %v (%0.2f QPS)\n", testDuration, float64(len(tests))/testDuration.Seconds())
	fmt.Printf("Send duration: %v (%0.2f QPS)\n", sendDuration, float64(len(tests))/sendDuration.Seconds())
}
