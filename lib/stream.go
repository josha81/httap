package httap

import (
	"bufio"
	"bytes"
	"io"
	"io/ioutil"
	"math/rand"
	"net"
	"net/http"
	"os"
	"time"

	"github.com/google/gopacket"
	"github.com/google/gopacket/tcpassembly/tcpreader"
)

type Stream struct {
	tcpreader.ReaderStream
	tap  *Wiretap
	flow gopacket.Flow
}

func NewStream(tap *Wiretap, netFlow, tcpFlow gopacket.Flow) *Stream {
	return &Stream{
		ReaderStream: tcpreader.NewReaderStream(),
		tap:          tap,
		flow:         netFlow,
	}
}

func (st *Stream) Consume() {
	buf := bufio.NewReader(st)

	for {
		req, err := http.ReadRequest(buf)
		if err == io.EOF {
			return
		} else if err != nil {
			st.tap.Log("Error: %s", err)
		} else {
			st.forward(req)
		}
	}
}

func (st *Stream) forward(req *http.Request) {
	req.URL.Scheme = "http"
	req.URL.Host = req.Host

	if len(st.tap.Methods) > 0 && !st.tap.Methods[req.Method] {
		return
	}

	body := new(bytes.Buffer)
	if _, err := io.Copy(body, req.Body); err != nil {
		st.tap.Log("Error: %s", err)
	}

	st.replaceHeaders(req)

	for _, dst := range st.tap.Destinations {
		n := st.forwardCount()
		for i := 0; i < n; i++ {
			copy := st.copy(req, body, dst)
			repeat := i > 0
			time.AfterFunc(time.Duration(i)*st.tap.RepeatDelay, func() {
				st.send(copy, req.URL.String(), repeat)
			})
		}
	}
}

func (st *Stream) replaceHeaders(req *http.Request) {
	for key, value := range st.tap.Headers {
		if value == "" {
			req.Header.Del(key)
			if key == "user-agent" {
				/* "Use the defaultUserAgent unless the Header contains one,
				   which may be blank to not send the header." */
				req.Header.Set(key, "")
			}
		} else {
			req.Header.Set(key, value)
			if key == "host" {
				req.Host = value
			}
		}
	}
}

func (st *Stream) send(req *http.Request, url string, repeat bool) {
	res, err := st.tap.Transport.RoundTrip(req)
	if err != nil {
		st.tap.Log("Error: %s", err)
	} else {
		/* "The client must close the response body when finished with it." */
		defer res.Body.Close()

		var fmt string
		if repeat {
			fmt = "%s %s %s (%s REPEAT) %d"
		} else {
			fmt = "%s %s %s (%s) %d"
		}
		st.tap.Log(fmt, st.flow.Src().String(), req.Method, url, req.URL.Host, res.StatusCode)

		if st.tap.Verbose {
			req.Body = nil
			req.Write(os.Stdout)
		}
	}
}

func (st *Stream) copy(req *http.Request, body *bytes.Buffer, dst *net.TCPAddr) *http.Request {
	host := *dst

	/* If the destination IP is unset, use the original destination IP. */
	if host.IP == nil {
		host.IP = net.ParseIP(st.flow.Dst().String())
	}

	url := *req.URL
	copy := *req
	copy.URL = &url
	copy.URL.Host = host.String()
	copy.Body = ioutil.NopCloser(bytes.NewReader(body.Bytes()))

	return &copy
}

func (st *Stream) forwardCount() int {
	min := int(st.tap.Multiply)
	prb := st.tap.Multiply - float32(min)
	if rand.Float32() < prb {
		return min + 1
	} else {
		return min
	}
}
