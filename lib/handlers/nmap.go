package handlers

import (
	"context"
	"crypto/rand"
	"fmt"
	"math/big"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/farhansabbir/telnet/lib"
)

func NmapHandler(ctx context.Context, host string, fromport, endport, iterations, timeout int, throttle bool) {
	istart := time.Now()                                   // capture initial time
	ipaddresses, err := lib.ResolveName(ctx, host) // resolve DNS
	var stats = make([]time.Duration, 0)
	if err != nil {
		fmt.Printf("%s ", lib.LogWithTimestamp(err.Error(), true))
		fmt.Println(lib.LogStats("telnet", stats, iterations))
	} else { // this is where no error occured in DNS lookup and we can proceed with regular nmap now
		fmt.Println(lib.LogWithTimestamp("DNS lookup successful for "+host+"' to "+strconv.Itoa(len(ipaddresses))+" addresses '["+strings.Join(ipaddresses[:], ", ")+"]' in "+time.Since(istart).String(), false))
		var WG sync.WaitGroup
		for i := 0; i < iterations; i++ { // loop over the ip addresses for the iterations required
			for _, ip := range ipaddresses { //  we need to loop over all ip addresses returned, even for once
				for port := fromport; port <= endport; port++ { // we need to loop over all ports individually
					if throttle { // check if throttle is enable, then slow things down a bit of random milisecond wait between 0 10000 ms
						i, err := rand.Int(rand.Reader, big.NewInt(10000))
						if err != nil {
							fmt.Println(err)
							return // added return to exit if error occurs
						}
						time.Sleep(time.Millisecond * time.Duration(i.Int64()))
					}
					WG.Add(1)
					go func(ip string, port int) {
						defer WG.Done()
						_, err := lib.IsPortUp(ip, port, timeout) // check if given port from this iteration is up or not
						if err != nil {

						} else {
							fmt.Println(lib.LogWithTimestamp(ip+" has port "+strconv.Itoa(port)+" open", false))
						}
					}(ip, port)
				}
			}
		}
		WG.Wait()
	}
	fmt.Println("Total time taken: " + time.Since(istart).String())
}
