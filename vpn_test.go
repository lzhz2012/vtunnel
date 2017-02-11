package vpn

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"github.com/mholt/caddy/caddyhttp/httpserver"
	"github.com/mholt/caddy"
	"github.com/FTwOoO/noise"
	"errors"
	"encoding/hex"
	"bytes"
	"fmt"
	"log"
	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"math/rand"
	"net"
	"github.com/FTwOoO/netstack/tcpip/buffer"
)

var validClientPublicKey = "e01ee3207ea15d346c362b7e20cef3a1088ec0a11a1141b3584ed44e2bb69531"
var validClientPrivateKey = "22e70850eb2da8fe184ed4998575f403f24c7ad54dbdc2132ae6a44c81b41180"

var invalidClientPublicKey = "5561cbf77dc96a21041f2a6127b927e439a15852dd4a915e7741fe4889afdb34"
var invalidClientPrivateKey = "d15fde7c16da6364374e8cc96f934c13de5b686aa0ad005e5ba2093fe2ff5da3"

var serverPublicKey = "e8e394b473b7b58514404fdddc0dd237ff631ceba3c0d1eddcddecb58f5a7d2a"
var serverPrivateKey = "3fbf4c6e081f845ab7998471dd4af084eea403f66a87cb5c2d775fbaa6c76eb4"

func getClientHandshake(pub string, pri string) (h *NoiseIXHandshake, err error) {
	cs := noise.NewCipherSuite(noise.DH25519, noise.CipherAESGCM, noise.HashSHA256)
	publicKey, _ := hex.DecodeString(pub)
	privateKey, _ := hex.DecodeString(pri)
	staticI := noise.DHKey{Public:publicKey, Private:privateKey}

	h, err = NewNoiseIXHandshake(
		cs,
		[]byte(DefaultPrologue),
		staticI,
		true,
	)
	return
}

type BasicAuthInfo struct {
	user     string
	password string
}

func TestHandshake(t *testing.T) {

	input := fmt.Sprintf(`vpn /vpn {
		    publickey %s
		    privatekey %s
		    clients {
		 	%s
		    }

		    subnet 192.168.4.1/24
		    mtu 1400
		    dnsport 53
		}`, serverPublicKey, serverPrivateKey, validClientPublicKey)

	h, err := Parse(caddy.NewTestController("http", input))
	if err != nil {
		t.Fatal(err)
	}

	h.Next = httpserver.HandlerFunc(func(w http.ResponseWriter, r *http.Request) (int, error) {
		return http.StatusNotFound, errors.New("404 error")
	})

	/*
	validHs, err := getClientHandshake(validClientPublicKey, validClientPrivateKey)
	if err != nil {
		t.Fatal(err)
	}

	invalidHs, err := getClientHandshake(invalidClientPublicKey, invalidClientPrivateKey)
	if err != nil {
		t.Fatal(err)
	}*/

	invalidAuth := BasicAuthInfo{user:"__", password:invalidClientPublicKey}

	SendBasicAuthHandshake(t, h, invalidAuth, http.StatusUnauthorized)
	SendDataWithBasicAuth(t, h, invalidAuth, http.StatusUnauthorized)

	validAuth := BasicAuthInfo{user:"__", password:validClientPublicKey}
	cliSetting := SendBasicAuthHandshake(t, h, validAuth, http.StatusOK)

	validAuth.user = cliSetting.Ip.String()
	SendDataWithBasicAuth(t, h, validAuth, http.StatusOK)
}

func SendBasicAuthHandshake(t *testing.T, h *handler, info BasicAuthInfo, expectedCode int) *ClientSetting {

	req, err := http.NewRequest("GET", "https://127.0.0.1/vpn/auth", nil)
	if err != nil {
		t.Fatalf("Could not create HTTP request: %v", err)
	}
	req.SetBasicAuth(info.user, info.password)

	rec := httptest.NewRecorder()
	statusCode, err := h.ServeHTTP(rec, req)
	if statusCode != expectedCode {
		t.Fatalf("Code [%d] not expected[%d]\n", statusCode, expectedCode)
	}

	if expectedCode == http.StatusOK {
		respContent := make([]byte, 1024)
		n, err := rec.Body.Read(respContent)
		if err != nil {
			t.Fatal(err)
		}

		cliSetting, err := DecodeClientSetting(string(respContent[:n]))
		if err != nil {
			t.Fatal(err)
		}
		log.Printf("Got auth response:%v", cliSetting.Encode())
		return cliSetting
	}

	return nil
}

func SendDataWithBasicAuth(t *testing.T, h *handler, info BasicAuthInfo, expectedCode int) {
	buf := bytes.NewBuffer([]byte{})
	packet := createFakeIPPacket(net.IP{192, 168, 4, 1})
	WritePackets(buf, []buffer.View{packet})

	req, err := http.NewRequest("POST", "https://127.0.0.1/vpn/", buf)
	if err != nil {
		t.Fatalf("Could not create HTTP request: %v", err)
	}

	req.SetBasicAuth(info.user, info.password)

	rec := httptest.NewRecorder()
	statusCode, err := h.ServeHTTP(rec, req)
	if statusCode != expectedCode {
		if err != nil {
			t.Fatal(err.Error())
		}
		t.Fatalf("Code [%d] not expected[%d]\n", statusCode, expectedCode)
	}

}

func createFakeIPPacket(src net.IP) []byte {
	buf := gopacket.NewSerializeBuffer()
	opts := gopacket.SerializeOptions{FixLengths:true}
	gopacket.SerializeLayers(buf, opts,
		&layers.IPv4{SrcIP:src, DstIP:net.IPv4(8, 8, 8, 8), Protocol:layers.IPProtocolICMPv4},
		&layers.ICMPv4{Id:uint16(rand.Int31())},
	)

	packetData := buf.Bytes()
	return packetData
}
