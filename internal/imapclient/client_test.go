package imapclient

import (
	"bufio"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func fakeServer(t *testing.T, handler func(net.Conn)) (string, int) {
	t.Helper()
	ln, e := net.Listen("tcp", "127.0.0.1:0")
	if e != nil {
		t.Fatal(e)
	}
	go func() {
		c, e := ln.Accept()
		if e == nil {
			handler(c)
			c.Close()
		}
		ln.Close()
	}()
	a := ln.Addr().(*net.TCPAddr)
	return "127.0.0.1", a.Port
}
func TestUIDBodyPeekAndMarkSeen(t *testing.T) {
	var commands []string
	host, port := fakeServer(t, func(c net.Conn) {
		fmt.Fprint(c, "* OK ready\r\n")
		r := bufio.NewReader(c)
		for {
			line, e := r.ReadString('\n')
			if e != nil {
				return
			}
			line = strings.TrimSpace(line)
			commands = append(commands, line)
			f := strings.Fields(line)
			tag := f[0]
			switch {
			case strings.Contains(line, " LOGIN "):
				fmt.Fprintf(c, "%s OK login\r\n", tag)
			case strings.Contains(line, " SELECT "):
				fmt.Fprint(c, "* OK [UIDVALIDITY 77] valid\r\n")
				fmt.Fprintf(c, "%s OK selected\r\n", tag)
			case strings.Contains(line, "UID SEARCH"):
				fmt.Fprint(c, "* SEARCH 10 11\r\n")
				fmt.Fprintf(c, "%s OK search\r\n", tag)
			case strings.Contains(line, "BODY.PEEK[]"):
				body := "Subject: x\r\n\r\nhello"
				fmt.Fprintf(c, "* 1 FETCH (UID 11 BODY[] {%d}\r\n%s\r\n)\r\n%s OK fetch\r\n", len(body), body, tag)
			case strings.Contains(line, "+FLAGS.SILENT"):
				fmt.Fprintf(c, "%s OK store\r\n", tag)
			case strings.Contains(line, "LOGOUT"):
				fmt.Fprint(c, "* BYE\r\n")
				fmt.Fprintf(c, "%s OK bye\r\n", tag)
				return
			}
		}
	})
	c, e := Dial(host, port, false, "", "", time.Second, time.Second)
	if e != nil {
		t.Fatal(e)
	}
	defer c.Close()
	if e = c.Login("u", "p"); e != nil {
		t.Fatal(e)
	}
	if e = c.Select("INBOX"); e != nil {
		t.Fatal(e)
	}
	if c.UIDValidity != 77 {
		t.Fatal(c.UIDValidity)
	}
	u, e := c.Search(time.Now().Add(-24*time.Hour), true)
	if e != nil || len(u) != 2 {
		t.Fatal(e, u)
	}
	b, e := c.FetchPeek(11, 1024)
	if e != nil || !strings.Contains(string(b), "hello") {
		t.Fatal(e, string(b))
	}
	if e = c.MarkSeen(11); e != nil {
		t.Fatal(e)
	}
	joined := strings.Join(commands, "\n")
	if !strings.Contains(joined, "UID FETCH 11 (BODY.PEEK[])") {
		t.Fatalf("BODY.PEEK not used:\n%s", joined)
	}
	if strings.Contains(joined, " RFC822") {
		t.Fatalf("RFC822 used:\n%s", joined)
	}
	if !strings.Contains(joined, "UID STORE 11 +FLAGS.SILENT (\\Seen)") {
		t.Fatalf("UID STORE missing:\n%s", joined)
	}
}

func TestOversizedFetchIsDiscardedAndConnectionStaysUsable(t *testing.T) {
	host, port := fakeServer(t, func(c net.Conn) {
		fmt.Fprint(c, "* OK ready\r\n")
		r := bufio.NewReader(c)
		for {
			line, e := r.ReadString('\n')
			if e != nil {
				return
			}
			line = strings.TrimSpace(line)
			tag := strings.Fields(line)[0]
			switch {
			case strings.Contains(line, "BODY.PEEK[]"):
				body := strings.Repeat("x", 20)
				fmt.Fprintf(c, "* 1 FETCH (BODY[] {%d}\r\n%s)\r\n%s OK fetch\r\n", len(body), body, tag)
			case strings.Contains(line, "+FLAGS.SILENT"):
				fmt.Fprintf(c, "%s OK store\r\n", tag)
				return
			}
		}
	})
	c, e := Dial(host, port, false, "", "", time.Second, time.Second)
	if e != nil {
		t.Fatal(e)
	}
	defer c.conn.Close()
	if _, e = c.FetchPeek(1, 10); e == nil {
		t.Fatal("expected size error")
	}
	if e = c.MarkSeen(1); e != nil {
		t.Fatalf("connection desynchronized after oversized literal: %v", e)
	}
}

func TestTLSWithBundledCA(t *testing.T) {
	// Generated at test runtime so no private key or external network is required.
	priv, e := rsa.GenerateKey(rand.Reader, 2048)
	if e != nil {
		t.Fatal(e)
	}
	now := time.Now()
	tmpl := &x509.Certificate{SerialNumber: big.NewInt(1), NotBefore: now.Add(-time.Minute), NotAfter: now.Add(time.Hour), KeyUsage: x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}, IPAddresses: []net.IP{net.ParseIP("127.0.0.1")}, BasicConstraintsValid: true}
	der, e := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &priv.PublicKey, priv)
	if e != nil {
		t.Fatal(e)
	}
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(priv)})
	cert, e := tls.X509KeyPair(certPEM, keyPEM)
	if e != nil {
		t.Fatal(e)
	}
	ln, e := tls.Listen("tcp", "127.0.0.1:0", &tls.Config{Certificates: []tls.Certificate{cert}})
	if e != nil {
		t.Fatal(e)
	}
	defer ln.Close()
	go func() {
		c, e := ln.Accept()
		if e != nil {
			return
		}
		defer c.Close()
		fmt.Fprint(c, "* OK tls ready\r\n")
		time.Sleep(100 * time.Millisecond)
	}()
	ca := filepath.Join(t.TempDir(), "ca.pem")
	if e = os.WriteFile(ca, certPEM, 0600); e != nil {
		t.Fatal(e)
	}
	addr := ln.Addr().(*net.TCPAddr)
	c, e := Dial("127.0.0.1", addr.Port, true, "", ca, time.Second, time.Second)
	if e != nil {
		t.Fatalf("bundled CA was not trusted: %v", e)
	}
	c.conn.Close()
}
