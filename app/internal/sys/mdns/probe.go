package mdns

import (
	"context"
	"fmt"
	"net"
	"strings"

	"github.com/miekg/dns"
	"golang.org/x/net/ipv4"
)

func probeHostname(ctx context.Context, host string, link lanInterface) error {
	conn, err := net.ListenUDP("udp4", &net.UDPAddr{IP: link.ips[0]})
	if err != nil {
		return err
	}
	defer conn.Close()
	packet := ipv4.NewPacketConn(conn)
	deadline, ok := ctx.Deadline()
	if !ok {
		return fmt.Errorf("hostname probe requires a deadline")
	}
	for _, err := range []error{
		packet.SetMulticastInterface(&link.iface),
		packet.SetMulticastTTL(255),
		packet.SetMulticastLoopback(true),
		conn.SetDeadline(deadline),
	} {
		if err != nil {
			return err
		}
	}
	query := new(dns.Msg)
	query.SetQuestion(host, dns.TypeA)
	query.RecursionDesired = false
	wire, err := query.Pack()
	if err != nil {
		return err
	}
	if _, err := conn.WriteToUDP(wire, multicastAddr); err != nil {
		return err
	}
	buf := make([]byte, 9000)
	for {
		n, from, err := conn.ReadFromUDP(buf)
		if err != nil {
			return fmt.Errorf("resolve %s: %w", host, err)
		}
		if from.Port != 5353 {
			continue
		}
		var reply dns.Msg
		if err := reply.Unpack(buf[:n]); err != nil {
			return fmt.Errorf("invalid hostname response: %w", err)
		}
		if matchesHostname(&reply, query, link.ips) {
			return nil
		}
	}
}

func matchesHostname(reply, query *dns.Msg, ips []net.IP) bool {
	if !reply.Response || !reply.Authoritative || reply.Rcode != dns.RcodeSuccess ||
		reply.Id != query.Id || len(reply.Question) != 1 || reply.Question[0] != query.Question[0] {
		return false
	}
	found := false
	for _, rr := range reply.Answer {
		record, ok := rr.(*dns.A)
		if !ok || !strings.EqualFold(record.Hdr.Name, query.Question[0].Name) {
			continue
		}
		if record.Hdr.Ttl == 0 || !containsIP(ips, record.A) {
			return false
		}
		found = true
	}
	return found
}

func containsIP(ips []net.IP, ip net.IP) bool {
	for _, candidate := range ips {
		if candidate.Equal(ip) {
			return true
		}
	}
	return false
}
