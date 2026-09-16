package mdns

import (
	"fmt"
	"net"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/miekg/dns"
	"golang.org/x/net/ipv4"
	"golang.org/x/net/ipv6"
)

// This opt-in check uses real multicast sockets without advertising a clinic's name.
func TestHostnameOverLAN(t *testing.T) {
	if os.Getenv("CARE_MDNS_NETWORK_TEST") != "1" {
		t.Skip("set CARE_MDNS_NETWORK_TEST=1 to exercise the LAN responder")
	}
	name := fmt.Sprintf("care-test-%x", time.Now().UnixNano())
	a, err := Advertise(name, func(message string) { t.Log(message) })
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(a.Stop)
	// Open the query sockets after startup announcements have finished.
	time.Sleep(3 * time.Second)
	if err := a.Resolves(); err != nil {
		t.Fatal(err)
	}
	for _, link := range a.links {
		families := []string{"udp4"}
		if link.ipv6 {
			families = append(families, "udp6")
		}
		for _, family := range families {
			for _, mode := range []string{"multicast", "legacy"} {
				t.Run(link.iface.Name+"/"+family+"/"+mode, func(t *testing.T) {
					group := multicastAddr
					local := &net.UDPAddr{IP: link.ips[0]}
					if family == "udp6" {
						group = &net.UDPAddr{IP: multicastAddr6.IP, Port: 5353, Zone: link.iface.Name}
						local = &net.UDPAddr{IP: net.IPv6unspecified}
					}
					var conn *net.UDPConn
					var err error
					if mode == "legacy" {
						conn, err = net.ListenUDP(family, local)
					} else {
						conn, err = net.ListenMulticastUDP(family, &link.iface, group)
					}
					if err != nil {
						t.Fatal(err)
					}
					t.Cleanup(func() { _ = conn.Close() })
					var read func([]byte) (int, net.IP, error)
					var setupErrors []error
					// Destination metadata detects unicast replies to multicast questions.
					if family == "udp4" {
						packet := ipv4.NewPacketConn(conn)
						setupErrors = []error{packet.SetMulticastInterface(&link.iface), packet.SetMulticastLoopback(true), packet.SetMulticastTTL(255)}
						_ = packet.SetControlMessage(ipv4.FlagDst, true)
						read = func(buf []byte) (int, net.IP, error) {
							n, cm, _, err := packet.ReadFrom(buf)
							if cm != nil {
								return n, cm.Dst, err
							}
							return n, nil, err
						}
					} else {
						packet := ipv6.NewPacketConn(conn)
						setupErrors = []error{packet.SetMulticastInterface(&link.iface), packet.SetMulticastLoopback(true), packet.SetMulticastHopLimit(255)}
						_ = packet.SetControlMessage(ipv6.FlagDst, true)
						read = func(buf []byte) (int, net.IP, error) {
							n, cm, _, err := packet.ReadFrom(buf)
							if cm != nil {
								return n, cm.Dst, err
							}
							return n, nil, err
						}
					}
					for _, err := range append(setupErrors, conn.SetDeadline(time.Now().Add(3*time.Second))) {
						if err != nil {
							t.Fatal(err)
						}
					}
					query := new(dns.Msg)
					query.SetQuestion(name+".local.", dns.TypeA)
					query.RecursionDesired = false
					if mode != "legacy" {
						query.Id = 0
					}
					wire, err := query.Pack()
					if err != nil {
						t.Fatal(err)
					}
					if _, err := conn.WriteToUDP(wire, group); err != nil {
						t.Fatal(err)
					}
					buf := make([]byte, 9000)
					for {
						n, destination, err := read(buf)
						if err != nil {
							t.Fatalf("no valid %s hostname reply: %v", mode, err)
						}
						var reply dns.Msg
						if err := reply.Unpack(buf[:n]); err != nil || !reply.Response {
							continue
						}
						matched := false
						for _, rr := range reply.Answer {
							record, ok := rr.(*dns.A)
							if !ok || !strings.EqualFold(record.Hdr.Name, name+".local.") {
								continue
							}
							matched = true
							if !containsIP(link.ips, record.A) {
								t.Fatalf("advertised off-interface address %s on %s", record.A, link.iface.Name)
							}
						}
						if !matched {
							continue
						}
						if !reply.Authoritative || reply.Rcode != dns.RcodeSuccess || reply.Id != query.Id {
							t.Fatalf("invalid response header: %+v", reply.MsgHdr)
						}
						if mode == "legacy" && (len(reply.Question) != 1 || reply.Question[0] != query.Question[0]) {
							t.Fatal("legacy response did not repeat the question")
						}
						if destination != nil && destination.IsMulticast() != (mode == "multicast") {
							t.Fatalf("%s reply sent to incorrect destination %s", mode, destination)
						}
						return
					}
				})
			}
		}
	}
}
