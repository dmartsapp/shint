package main

import (
	"crypto/tls"
	"flag"
	"fmt"
	"io"
	"math/rand"
	"net"
	"net/http"
	URL "net/url"
	"os"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"golang.org/x/net/icmp"
	"golang.org/x/net/ipv4"
)

var (
	iterations int
	delay      int
	udp        *bool
	throttle   *bool
	timeout    int
	web        *bool
	nmap       *bool
	fromport   int
	endport    int
)

const (
	SuccessNoError   uint8  = 0
	NoSuchHostError  uint8  = 2
	TimeoutError     uint8  = 3
	UnreachableError uint8  = 5
	HttpGetError     uint8  = 4
	UnknownError     uint8  = 1
	HTTP_CLIENT      string = "dmarts.app-http-v0.1"
)

func init() {
	flag.IntVar(&iterations, "count", 1, "Number of times to check connectivity")
	flag.IntVar(&timeout, "timeout", 5, "Timeout in seconds to connect")
	flag.IntVar(&delay, "delay", 0, "Seconds delay between each iteration given in count")
	udp = flag.Bool("udp", false, "Flag option (Doesn't expect any value after option). Use UDP instead of tcp to connect to endpoint")
	web = flag.Bool("web", false, "Use web request as a web client.")
	throttle = flag.Bool("throttle", false, "Flag option to throttle between every iteration of count to simulate non-uniform request.")
	nmap = flag.Bool("nmap", false, "Flag option to run tcp port scan. This flag ignores all other parameters except -fromport and -endport, if mentioned.")
	flag.IntVar(&fromport, "fromport", 1, "Start port to begin TCP scan from")
	flag.IntVar(&endport, "endport", 80, "End port to run TCP scan to")

	flag.Usage = func() {
		fmt.Println("Usage: " + os.Args[0] + " [options] <fqdn|IP> port")
		fmt.Println("options:")
		flag.PrintDefaults()
		fmt.Println()
		fmt.Println("Example (fqdn): " + os.Args[0] + " google.com 443")
		fmt.Println("Example (IP): " + os.Args[0] + " 10.10.10.10 443")
		fmt.Println("Example (fqdn with -web flag to send 'https' request to path '/pages/index.html' as client with user-agent set as '" + HTTP_CLIENT + "'): " + os.Args[0] + " -web https://google.com/pages/index.html")
		os.Exit(int(SuccessNoError))
	}
}

func resolveName(ipaddress string) *net.IPAddr {
	ip, err := net.ResolveIPAddr("", ipaddress)
	if err != nil {
		fmt.Println(err.Error())
		if strings.Contains(err.Error(), "no such host") {
			os.Exit(int(NoSuchHostError))
		}
		os.Exit(int(UnknownError))

	}
	return ip
}

func Ping(target_ip string) {
	ping, err := icmp.ListenPacket("udp4", "0.0.0.0")
	if err != nil {

	}
	defer ping.Close()
	msg := icmp.Message{
		Type: ipv4.ICMPTypeEcho, Code: 0,
		Body: &icmp.Echo{
			ID: os.Getpid() & 0xffff, Seq: 1,
			Data: []byte(""),
		},
	}
	msg_bytes, err := msg.Marshal(nil)
	if err != nil {
		fmt.Printf("Error on Marshal %v", msg_bytes)
		panic(err)
	}

	// Write the message to the listening connection
	if _, err := ping.WriteTo(msg_bytes, &net.UDPAddr{IP: net.ParseIP(target_ip)}); err != nil {
		fmt.Printf("Error on WriteTo %v", err)
		panic(err)
	}
}

func main() {

	flag.Parse()
	if !*web {
		if len(flag.Args()) != 2 {
			flag.Usage()
		}
	}

	if !*udp {
		THROTTLE_MAX := 10 // this is the max in seconds to wait if throttle is true and delay isn't 0

		if !*web {
			// this is regular TCP telnet
			port := flag.Args()[1]
			start := time.Now()
			ip := resolveName(flag.Args()[0]).String()
			end := time.Now()
			fmt.Println(time.Now().Local().String() + ". Successfully resolved '" + flag.Args()[0] + "' to '" + ip + "' in: " + strconv.Itoa(int(end.Sub(start).Milliseconds())) + "ms")
			statistics := make([]int, 0, iterations)
			for i := 0; i < iterations; i++ {
				delay1 := delay
				if *throttle {
					delay1 = delay * rand.Intn(THROTTLE_MAX)
				}
				time.Sleep(time.Second * time.Duration(delay1))
				timetaken := dialNow("tcp", ip+":"+port, timeout)

				if timetaken >= 0 {
					fmt.Println(time.Now().Local().String() + ". Successfully reached '" + flag.Args()[0] + ":" + port + "' in: " + strconv.Itoa(timetaken) + "ms.")
					statistics = append(statistics, timetaken)
				} else {
					fmt.Println(time.Now().Local().String() + ". Unable to reach '" + flag.Args()[0] + ":" + port + "'")
				}
			}
			slices.Sort(statistics)
			{
				avg := 0
				for _, val := range statistics {
					avg = avg + val
				}
				fmt.Println("------------- Statistics of telnet to '" + flag.Args()[0] + "' on port '" + port + "' -------------")
				fmt.Printf("%v requests transmitted, %v failed, %v%% success, ", iterations, (iterations - len(statistics)), (len(statistics) * 100 / iterations))
				if len(statistics) == 0 {
					fmt.Printf("min/max/avg = 0/0/0\n")
				} else {
					fmt.Printf("min/avg/max = %v/%v/%vms latency\n", slices.Min(statistics), avg/len(statistics), slices.Max(statistics))
				}

			}
			//fmt.Println(slices.Max(statistics))
		}

		if *web {
			// this is web request; Check for other flags
			url := flag.Args()[0]
			if matches, _ := regexp.MatchString(`(?:https?://)`, url); matches {
				urlarg, err := URL.Parse(url)
				if err != nil {
					fmt.Println(err.Error())
					os.Exit(1)
				}

				start := time.Now()
				ip := resolveName(urlarg.Hostname())
				end := time.Now()
				fmt.Println(time.Now().Local().String() + ". Successfully resolved '" + urlarg.Hostname() + "' to '" + ip.String() + "' in: " + strconv.Itoa(int(end.Sub(start).Milliseconds())) + "ms")

				httpClient := &http.Client{Timeout: time.Second * time.Duration(timeout)}

				if strings.Contains(url, "https") {
					httpsTransport := &http.Transport{
						TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
					}
					httpClient = &http.Client{Transport: httpsTransport, Timeout: time.Second * time.Duration(timeout)}
				}

				ret := int(SuccessNoError)
				statistics := make([]int, 0, iterations)
				req, _ := http.NewRequest(http.MethodGet, url, nil)
				req.Header.Add("User-Agent", HTTP_CLIENT)
				for i := 0; i < iterations; i++ {
					start := time.Now()
					resp, err := httpClient.Do(req)
					end := time.Now()
					delay1 := delay
					if *throttle {
						delay1 = delay * rand.Intn(THROTTLE_MAX)
					}
					time.Sleep(time.Second * time.Duration(delay1))
					if err != nil {
						if strings.Contains(err.Error(), "HTTP response to HTTPS client") {
							fmt.Println(time.Now().Local().String() + ". " + url + " is probably http instead of https. Elapsed time: " + strconv.Itoa(int(end.Sub(start).Seconds())) + "s")
						}
						if strings.Contains(err.Error(), "refused") {
							fmt.Println(time.Now().Local().String() + ". " + url + " is down. Elapsed time: " + strconv.Itoa(int(end.Sub(start).Microseconds())) + "µs")
							ret = (int(UnreachableError))

						}
						if strings.Contains(err.Error(), "Client.Timeout") {
							fmt.Println(time.Now().Local().String() + ". " + url + " is down within timeout (" + strconv.Itoa(timeout) + "s). Elapsed time: " + strconv.Itoa(int(end.Sub(start).Seconds())) + "s")
							ret = (int(TimeoutError))

						}
						if strings.Contains(err.Error(), "reset by peer") {
							fmt.Println(time.Now().Local().String() + ". " + url + ": unable to connect within elasped timeout (Possible protocol mismatch, e.g. http vs https). Elapsed time: " + strconv.Itoa(int(end.Sub(start).Seconds())) + "s")
							ret = (int(TimeoutError))

						}
						continue
						//fmt.Println(err.Error())
						//os.Exit(ret)
					}

					payload, _ := io.ReadAll(resp.Body)

					fmt.Printf(time.Now().Local().String()+". Read: %v bytes.", len(string(payload)))

					fmt.Print(" HTTP Response code: " + resp.Status + ". ")
					fmt.Println(" Response received in: " + strconv.Itoa(int(end.Sub(start).Milliseconds())) + "ms")
					resp.Body.Close()
					statistics = append(statistics, int(end.Sub(start).Milliseconds()))
					ret = int(resp.StatusCode)
				}
				slices.Sort(statistics)
				{
					avg := 0
					for _, val := range statistics {
						avg = avg + val
					}
					fmt.Println("------------- Statistics of web request to '" + url + "' -------------")
					fmt.Printf("%v requests transmitted, %v failed, %v%% success, ", iterations, (iterations - len(statistics)), (len(statistics) * 100 / iterations))
					if len(statistics) == 0 {
						fmt.Printf("min/max/avg = 0/0/0\n")
					} else {
						fmt.Printf("min/avg/max = %v/%v/%vms latency\n", slices.Min(statistics), avg/len(statistics), slices.Max(statistics))
					}

				}

				os.Exit(int(ret))
			} else {
				fmt.Println(url + " is a not valid http/https url")
			}

		}
	} else {
		// this is for UDP request
		fmt.Println("Placeholder for regular UDP")
	}
}

// func dialNow(protocol string, addressport string, timeout int, wg *sync.WaitGroup) int {
func dialNow(protocol string, addressport string, timeout int) int {
	start := time.Now()
	connect, err := net.DialTimeout(protocol, addressport, time.Duration(timeout)*time.Second)
	end := time.Now()
	if err != nil {

		if strings.Contains(err.Error(), "timeout") {
			return -1

		}
		if strings.Contains(err.Error(), "refused") {
			//fmt.Println(addressport + " combination is down. Elapsed time: " + strconv.Itoa(int(end.Sub(start).Microseconds())) + "µs")
			return -1
		}
		// wg.Done()
		fmt.Println(err.Error())
		return -1
	}
	connect.Close()

	// wg.Done()
	return int((end.Sub(start)).Milliseconds())
}
