package cmd

import (
	"crypto/tls"
	"fmt"
	"math"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func setupRawConnection(useTLS bool) net.Conn {
	addr := fmt.Sprintf("%v:%v", serverHost, serverPort)
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		fmt.Printf("Failed to connect to %v: %v\n", addr, err)
		os.Exit(1)
	}
	if useTLS {
		conn = tls.Client(conn, &tls.Config{InsecureSkipVerify: true})
	}
	return conn
}

func serverURL(useTLS bool) string {
	if useTLS {
		return fmt.Sprintf("https://%v:%v", serverHost, serverPort)
	}
	return fmt.Sprintf("http://%v:%v", serverHost, serverPort)
}

func setupHttpConnection() *http.Client {
	// same values as http.DefaultTransport unless noted explicitly
	transport := &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
			DualStack: true,
		}).DialContext,
		MaxIdleConns:          1,                     // different from default (100)
		IdleConnTimeout:       idleConnectionTimeout, // different from default (90 sec)
		TLSHandshakeTimeout:   30 * time.Second,      // different from default (10 sec)
		ExpectContinueTimeout: 1 * time.Second,
		TLSClientConfig:       &tls.Config{InsecureSkipVerify: true},
	}
	return &http.Client{
		Transport: transport,
		Timeout:   requestTimeout,
	}
}

func setupCloseHandler(onClose func()) {
	c := make(chan os.Signal, 2)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-c
		fmt.Println("\r- Ctrl+C detected")
		onClose()
		os.Exit(0)
	}()
}

func summarizeTimings(times []time.Duration) string {
	if len(times) == 0 {
		return "--"
	}
	avg, min, max := time.Duration(0), time.Duration(math.MaxInt64), time.Duration(0)
	for i := range times {
		min = minDuration(min, times[i])
		max = maxDuration(max, times[i])
		avg += times[i]
	}
	avg = time.Duration(avg.Nanoseconds() / int64(len(times)))
	return fmt.Sprintf("min = %0.3f, avg = %0.3f, max = %0.3f", min.Seconds()*1000, avg.Seconds()*1000, max.Seconds()*1000)
}

func minDuration(a, b time.Duration) time.Duration {
	if a < b {
		return a
	}
	return b
}

func maxDuration(a, b time.Duration) time.Duration {
	if a > b {
		return a
	}
	return b
}
