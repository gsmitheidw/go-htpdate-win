package main

import (
	"flag"
	"fmt"
	"net"
	"net/http"
	"os"
	"sync"
	"time"
	"syscall"
	"unsafe"
)

var (
	queryFlag  = flag.Bool("q", false, "Query only, print server times and offsets (default)")
	setFlag    = flag.Bool("s", false, "Set local system time to average of server times")
	debugFlag  = flag.Bool("d", false, "Debug output")
	helpFlag   = flag.Bool("h", false, "Show help")
	daemonFlag = flag.Bool("D", false, "Run in daemon mode serving NTP on localhost:123")
	interval   = flag.Int("i", 1, "Daemon polling interval in seconds")
)

// SYSTEMTIME for Windows API
type systemtime struct {
	Year, Month, DayOfWeek, Day       uint16
	Hour, Minute, Second, Milliseconds uint16
}

func usage() {
	fmt.Println(`go-htpdate-win: Query HTTP(S) servers for time and optionally set local system time.

Usage:
  go-htpdate-win.exe [options] <URL>...

Options:
  -q          Query only, print server times and offsets (default)
  -s          Set local system time to average of server times
  -d          Debug output
  -D          Daemon mode: serve NTP on localhost:123
  -i <secs>   Poll interval for daemon mode (default 1)
  -h          Show this help
`)
}

// Set Windows system time
func setSystemTime(t time.Time) error {
	st := systemtime{
		Year:         uint16(t.Year()),
		Month:        uint16(t.Month()),
		Day:          uint16(t.Day()),
		Hour:         uint16(t.Hour()),
		Minute:       uint16(t.Minute()),
		Second:       uint16(t.Second()),
		Milliseconds: uint16(t.Nanosecond() / 1e6),
	}
	k32, err := syscall.LoadDLL("kernel32.dll")
	if err != nil {
		return err
	}
	defer k32.Release()

	setSysTime, err := k32.FindProc("SetSystemTime")
	if err != nil {
		return err
	}
	r, _, e := setSysTime.Call(uintptr(unsafe.Pointer(&st)))
	if r == 0 {
		return e
	}
	return nil
}

// Query HTTP(S) server for Date header
func getHTTPTime(url string) (time.Time, error) {
	client := &http.Client{Timeout: 5 * time.Second}
	req, err := http.NewRequest("HEAD", url, nil)
	if err != nil {
		return time.Time{}, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return time.Time{}, err
	}
	defer resp.Body.Close()

	dateHeader := resp.Header.Get("Date")
	if dateHeader == "" {
		return time.Time{}, fmt.Errorf("no Date header")
	}

	t, err := time.Parse(time.RFC1123, dateHeader)
	if err != nil {
		t, err = time.Parse(time.RFC1123Z, dateHeader)
		if err != nil {
			return time.Time{}, fmt.Errorf("failed to parse Date header: %v", err)
		}
	}

	if *debugFlag {
		fmt.Printf("[DEBUG] URL: %s -> %s\n", url, dateHeader)
	}
	return t, nil
}

// Average offsets from multiple servers and print results
func queryServers(urls []string) (float64, error) {
	now := time.Now().UTC()
	var offsets []float64

	for _, url := range urls {
		t, err := getHTTPTime(url)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error querying %s: %v\n", url, err)
			continue
		}
		offset := t.Sub(now).Seconds()
		offsets = append(offsets, offset)
		fmt.Printf("%s -> Server Time: %s, Local Time: %s, Offset: %.3f sec\n",
			url, t.Format(time.RFC3339), now.Format(time.RFC3339), offset)
	}

	if len(offsets) == 0 {
		return 0, fmt.Errorf("no valid server responses")
	}

	var sum float64
	for _, o := range offsets {
		sum += o
	}
	avgOffset := sum / float64(len(offsets))
	fmt.Printf("Average offset: %.3f sec\n", avgOffset)
	return avgOffset, nil
}

// Minimal NTP responder (UDP/123) for localhost using latest HTTP time
func runNTPDaemon(urls []string, intervalSec int) {
	var mu sync.Mutex
	var lastTime time.Time

	// Poll HTTP servers regularly
	go func() {
		ticker := time.NewTicker(time.Duration(intervalSec) * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			offset, err := queryServers(urls)
			if err != nil {
				if *debugFlag {
					fmt.Println("[DEBUG] NTP Daemon: query error:", err)
				}
				continue
			}
			mu.Lock()
			lastTime = time.Now().UTC().Add(time.Duration(offset * float64(time.Second)))
			mu.Unlock()
		}
	}()

	addr := net.UDPAddr{Port: 123, IP: net.ParseIP("127.0.0.1")}
	conn, err := net.ListenUDP("udp", &addr)
	if err != nil {
		fmt.Println("Failed to start NTP daemon:", err)
		return
	}
	defer conn.Close()
	fmt.Println("NTP daemon listening on 127.0.0.1:123")

	buf := make([]byte, 48)
	for {
		n, remote, err := conn.ReadFromUDP(buf)
		if err != nil || n < 48 {
			continue
		}
		mu.Lock()
		seconds := uint32(lastTime.Unix() + 2208988800) // NTP epoch
		fraction := uint32((float64(lastTime.Nanosecond()) / 1e9) * (1 << 32))
		mu.Unlock()
		buf[0] = 0x1C // NTP mode 4, version 4
		buf[40] = byte(seconds >> 24)
		buf[41] = byte(seconds >> 16)
		buf[42] = byte(seconds >> 8)
		buf[43] = byte(seconds)
		buf[44] = byte(fraction >> 24)
		buf[45] = byte(fraction >> 16)
		buf[46] = byte(fraction >> 8)
		buf[47] = byte(fraction)

		conn.WriteToUDP(buf[:48], remote)
	}
}

func main() {
	flag.Parse()
	if *helpFlag || flag.NArg() == 0 {
		usage()
		return
	}

	urls := flag.Args()

	if *daemonFlag {
		runNTPDaemon(urls, *interval)
		return
	}

	offset, err := queryServers(urls)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}

	if *setFlag {
		newTime := time.Now().UTC().Add(time.Duration(offset * float64(time.Second)))
		fmt.Printf("Setting system time to %s (avg offset %.3f sec)\n", newTime.Format(time.RFC3339), offset)
		err := setSystemTime(newTime)
		if err != nil {
			fmt.Fprintln(os.Stderr, "Failed to set system time:", err)
			os.Exit(1)
		}
	}
}

