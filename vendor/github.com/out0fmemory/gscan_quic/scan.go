package gscan

import (
	"bytes"
	"crypto/sha256"
	"crypto/tls"
	"encoding/base64"
	"io"
	"io/ioutil"
	"log"
	"math/rand"
	"net"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	quic "github.com/phuslu/quic-go"
	"github.com/phuslu/quic-go/h2quic"
)

type ScanRecord struct {
	IP         string
	MatchHosts []string
	PingRTT    time.Duration
	SSLRTT     time.Duration

	httpVerifyTimeout time.Duration
}
type ScanRecordArray []*ScanRecord

type ScanOptions struct {
	Config *GScanConfig

	recordMutex sync.Mutex
	hostsMutex  sync.Mutex
	inputHosts  HostIPTable
	records     ScanRecordArray

	scanCounter int32
}

func (options *ScanOptions) AddRecord(rec *ScanRecord) {
	options.recordMutex.Lock()
	if nil == options.records {
		options.records = make(ScanRecordArray, 0)
	}
	options.records = append(options.records, rec)
	options.recordMutex.Unlock()
	log.Printf("Found a record: IP=%s, SSLRTT=%fs\n", rec.IP, rec.SSLRTT.Seconds())
}

func (options *ScanOptions) IncScanCounter() {
	atomic.AddInt32(&(options.scanCounter), 1)
	if options.scanCounter%1000 == 0 {
		log.Printf("Scanned %d IPs, Found %d records\n", options.scanCounter, options.RecordSize())
	}
}

func (options *ScanOptions) RecordSize() int {
	options.recordMutex.Lock()
	defer options.recordMutex.Unlock()
	return len(options.records)
}

func (options *ScanOptions) SSLMatchHosts(conn *tls.Conn) []string {
	hosts := make([]string, 0)
	options.hostsMutex.Lock()
	for _, host := range options.inputHosts {
		testhost := host.Host
		if strings.Contains(testhost, ".appspot.com") {
			testhost = "appengine.google.com"
		} else if strings.Contains(testhost, "ggpht.com") {
			testhost = "googleusercontent.com"
		} else if strings.Contains(testhost, ".books.google.com") {
			testhost = "books.google.com"
		} else if strings.Contains(testhost, ".googleusercontent.com") {
			testhost = "googleusercontent.com"
		}
		if conn.VerifyHostname(testhost) == nil {
			hosts = append(hosts, host.Host)
		}
	}
	options.hostsMutex.Unlock()
	dest := make([]string, len(hosts))
	perm := rand.Perm(len(hosts))
	for i, v := range perm {
		dest[v] = hosts[i]
	}
	hosts = dest
	return hosts
}

func (options *ScanOptions) HaveHostInRecords(host string) bool {
	options.hostsMutex.Lock()
	defer options.hostsMutex.Unlock()
	_, exists := options.inputHosts[host]
	return !exists
}

func (options *ScanOptions) RemoveFromInputHosts(hosts []string) {
	options.hostsMutex.Lock()
	for _, host := range hosts {
		delete(options.inputHosts, host)
	}
	options.hostsMutex.Unlock()
}

func matchHostnames(pattern, host string) bool {
	if len(pattern) == 0 || len(host) == 0 {
		return false
	}
	patternParts := strings.Split(pattern, ".")
	hostParts := strings.Split(host, ".")

	if len(patternParts) != len(hostParts) {
		return false
	}

	for i, patternPart := range patternParts {
		if patternPart == "*" {
			continue
		}
		if patternPart != hostParts[i] {
			return false
		}
	}
	return true
}

var (
	tlsCfg = &tls.Config{
		InsecureSkipVerify: true,
	}
	g2pkp, _ = base64.StdEncoding.DecodeString("7HIpactkIAq2Y49orFOOQKurWxmmSFZhBCoQYcRhJ3Y=")
	g3pkp, _ = base64.StdEncoding.DecodeString("f8NnEFZxQ4ExFOhSN7EiFWtiudZQVD2oY60uauV/n78=")
	g3ecc, _ = base64.StdEncoding.DecodeString("ekG8/PoSqjfKunOaS9iIzR3hZAJptWJKV7CgyriO+MA=")
)

func testip_once(ip string, options *ScanOptions, record *ScanRecord) bool {
	start := time.Now()

	pingRTT := (options.Config.ScanMinPingRTT + options.Config.ScanMaxPingRTT) / 2
	if options.Config.VerifyPing {
		err := Ping(ip, options.Config.ScanMaxPingRTT)
		if err != nil {
			return false
		}
		end := time.Now()
		if nil == err {
			if options.Config.ScanMinPingRTT > 0 && end.Sub(start) < options.Config.ScanMinPingRTT {
				return false
			}
			pingRTT = end.Sub(start)
		}
	}
	record.PingRTT = record.PingRTT + pingRTT

	addr := net.JoinHostPort(ip, "443")

	start = time.Now()
	success := make(chan bool, 5)

	go func() {
		<-time.After(options.Config.ScanMaxSSLRTT + 500*time.Millisecond)
		success <- false
	}()

	var quicSessn quic.Session
	defer func() {
		if quicSessn != nil {
			quicSessn.Close(nil)
		}
	}()

	tr := &h2quic.RoundTripper{DisableCompression: true}
	defer tr.Close()

	quicCfg := &quic.Config{
		HandshakeTimeout: options.Config.ScanMaxSSLRTT - 500*time.Millisecond,
		// IdleTimeout:      options.Config.ScanMaxSSLRTT,
		KeepAlive: false,
	}

	udpAddr, err := net.ResolveUDPAddr("udp", addr)
	if err != nil {
		return false
	}
	udpConn, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.IPv4zero, Port: 0})
	if err != nil {
		return false
	}

	udpConn.SetDeadline(time.Now().Add(options.Config.ScanMaxSSLRTT))
	defer udpConn.Close()

	go func(chan bool) {
		var err error
		// quicSessn, err = quic.DialAddr(addr, tlsCfg, quicCfg)
		quicSessn, err = quic.Dial(udpConn, udpAddr, addr, tlsCfg, quicCfg)
		if err != nil {
			success <- false
			return
		}
		// 证书验证
		cs := quicSessn.ConnectionState()
		if cs == nil {
			success <- false
			return
		}
		pcs := cs.PeerCertificates
		if len(pcs) < 2 {
			success <- false
			return
		}
		pkp := sha256.Sum256(pcs[1].RawSubjectPublicKeyInfo)
		if !bytes.Equal(g2pkp, pkp[:]) && !bytes.Equal(g3pkp, pkp[:]) && !bytes.Equal(g3ecc, pkp[:]) {
			success <- false
			return
		}

		tr.DialAddr = func(hostname string, tlsConfig *tls.Config, config *quic.Config) (quic.Session, error) {
			return quicSessn, err
		}
		hclient := &http.Client{
			Transport: tr,
		}

		for _, verifyHost := range options.Config.ScanGoogleIP.HTTPVerifyHosts {
			req, _ := http.NewRequest(http.MethodHead, "https://"+verifyHost, nil)
			req.Close = true
			resp, err := hclient.Do(req)
			if resp != nil && resp.Body != nil {
				io.Copy(ioutil.Discard, resp.Body)
				resp.Body.Close()
			}
			if nil != err || resp.StatusCode >= 400 {
				success <- false
				return
			}
		}
		success <- true
	}(success)

	if <-success == false {
		return false
	}

	sslRTT := time.Since(start)
	record.SSLRTT = record.SSLRTT + sslRTT
	return true
}

func testip(ip string, options *ScanOptions) *ScanRecord {
	record := new(ScanRecord)
	record.IP = ip
	for i := 0; i < options.Config.ScanCountPerIP; i++ {
		if !testip_once(ip, options, record) {
			return nil
		}
	}
	record.PingRTT = record.PingRTT / time.Duration(options.Config.ScanCountPerIP)
	record.SSLRTT = record.SSLRTT / time.Duration(options.Config.ScanCountPerIP)
	return record
}

func testip_worker(ch chan string, options *ScanOptions, wg *sync.WaitGroup) {
	for ip := range ch {
		record := testip(ip, options)
		if nil != record {
			if !options.Config.scanIP {
				options.RemoveFromInputHosts(record.MatchHosts)
			}
			options.AddRecord(record)
		}
		options.IncScanCounter()
	}
	wg.Done()
}
