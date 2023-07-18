package main

import (
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"

	"github.com/docker/docker/pkg/ioutils"
)

var parameter = struct {
	McastIfc   *string
	Iface      *net.Interface
	ListenAddr *string
	ListenPort *int
}{}

func main() {
	fmt.Println("gdpxy")
	if err := parseParameter(); err != nil {
		panic(err)
	}

	http.HandleFunc("/", handler)
	fmt.Printf("http listen: %s:%d\n", *parameter.ListenAddr, *parameter.ListenPort)
	err := http.ListenAndServe(fmt.Sprintf("%s:%d", *parameter.ListenAddr, *parameter.ListenPort), nil)
	if err != nil {
		panic(err)
	}
}

func parseParameter() (err error) {
	parameter.McastIfc = flag.String("m", "", "mcast_ifc (IP addr or name)")
	parameter.ListenAddr = flag.String("a", "0.0.0.0", "http listen addr (default 0.0.0.0)")
	parameter.ListenPort = flag.Int("p", 8080, "http listen port (default 8080)")
	flag.Parse()

	iface := *parameter.McastIfc

	if iface != "" {
		// interface
		ifis, e := net.Interfaces()
		if e != nil {
			err = e
			return
		}

		for _, v := range ifis {
			if v.Name == iface {
				parameter.Iface = &v
				break
			}

			addrs, err := v.Addrs()
			if err != nil {
				continue
			}

			for _, addr := range addrs {
				sa := strings.Split(addr.String(), "/")
				fmt.Println(sa)
				if len(sa) > 1 && iface == sa[0] {
					parameter.Iface = &v
					break
				}
			}
		}

		if parameter.Iface == nil {
			err = fmt.Errorf("interface %s not found", iface)
			return
		}

		fmt.Printf("multicast: select interface %s\n", parameter.Iface.Name)
	}

	return
}

var tag = "/rtp/"
var tagLen = len(tag)

func handler(w http.ResponseWriter, r *http.Request) {
	var err error
	defer func(w http.ResponseWriter, r *http.Request) {
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	}(w, r)

	path := r.URL.Path
	fmt.Println(path)
	if len(path) < tagLen {
		err = fmt.Errorf("path too short")
		return
	}

	addr := strings.Replace(path[tagLen:], "/", ":", -1)
	fmt.Println(addr)

	conn, err := newMulticastReader(addr)
	if err != nil {
		return
	}

	defer func() {
		_ = conn.Close()
	}()

	w.Header().Set("X-Content-Type-Options", "nosniff")
	writer := ioutils.NewWriteFlusher(w)
	fmt.Println("start send data")
	_, _ = io.Copy(writer, conn)
}

var maxDatagramSize = 1024 * 32

func newMulticastReader(address string) (conn *net.UDPConn, err error) {
	// Parse the string address
	addr, err := net.ResolveUDPAddr("udp", address)
	if err != nil {
		return
	}

	fmt.Printf("multicast: listen addr %s\n", address)
	conn, err = net.ListenMulticastUDP("udp", parameter.Iface, addr)
	if err == nil {
		_ = conn.SetReadBuffer(maxDatagramSize)
	}
	return
}
