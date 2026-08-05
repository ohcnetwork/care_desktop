package care

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"
)

// The server keeps its own DNS record current, using the same Cloudflare token it
// needs for certificate renewal. A stale record fails unhelpfully: valid certificate,
// healthy server, and devices simply can't find it. See docs/tls.md.

// dnsTTL is short on purpose: when the address changes, devices should pick up the
// new one in about a minute rather than serving a stale answer for an hour.
const dnsTTL = 60

// cloudflareAPI is a var, not a const, so tests can point the client at a stub.
var cloudflareAPI = "https://api.cloudflare.com/client/v4"

// OutboundIP is this computer's address on the network it can reach.
//
// Enumerating interfaces and picking a likely one is wrong here: this machine always
// has Docker bridges (172.x), often a VPN. Opening a UDP socket toward an off-LAN
// address sends no packets but makes the kernel choose a source address from the real
// routing table - by definition the interface facing the network.
func OutboundIP() (string, error) {
	conn, err := net.Dial("udp", "1.1.1.1:80")
	if err != nil {
		return "", fmt.Errorf("no usable network interface: %w", err)
	}
	defer conn.Close()
	addr, ok := conn.LocalAddr().(*net.UDPAddr)
	if !ok || addr.IP == nil {
		return "", fmt.Errorf("could not determine this computer's network address")
	}
	return addr.IP.String(), nil
}

// heldLocally reports whether this computer actually has the given address.
func heldLocally(ip string) bool {
	for _, own := range localIPv4s() {
		if own == ip {
			return true
		}
	}
	return false
}

// lanIP is the address clients use to reach this computer. Auto-detection answers
// "which address reaches the internet?", which matches only when clients are on that
// same network - not when this machine is itself the hotspot. CARE_LAN_IP settles it.
// See docs/tls.md#when-the-server-is-on-two-networks.
func (e *Engine) lanIP() (string, error) {
	pinned := strings.TrimSpace(e.get("CARE_LAN_IP", ""))
	if pinned == "" {
		return OutboundIP()
	}
	ip := net.ParseIP(pinned)
	if ip == nil || ip.To4() == nil {
		return "", fmt.Errorf("CARE_LAN_IP is %q, which isn't an IPv4 address "+
			"(expected something like 192.168.4.1)", pinned)
	}
	return ip.String(), nil
}

// SyncDNS points the clinic's A record at the address clients use to reach this
// computer, creating the record if it has gone missing. A no-op when the record is
// already correct, so it's cheap to call on every start.
func (e *Engine) SyncDNS() error {
	host := e.publicHost()
	if host == "" {
		return nil // not configured yet; nothing to point anywhere
	}
	ip, err := e.lanIP()
	if err != nil {
		return err
	}
	// A pinned address this machine doesn't actually hold is nearly always a typo,
	// but the interface may simply not be up yet (a Pi's hotspot can start after
	// Docker). Warn rather than refuse, so a boot-order race can't wedge the clinic.
	if e.get("CARE_LAN_IP", "") != "" && !heldLocally(ip) {
		e.logln("note: CARE_LAN_IP is " + ip + ", but this computer currently has " +
			strings.Join(localIPv4s(), ", ") + " - check it if devices can't connect")
	}
	cf := newCloudflare(e.dnsToken())
	zoneID, err := cf.zoneFor(host)
	if err != nil {
		return err
	}
	recordID, current, err := cf.findARecord(zoneID, host)
	if err != nil {
		return err
	}
	if current == ip {
		return nil // already correct - the common case
	}
	if err := cf.putARecord(zoneID, recordID, host, ip); err != nil {
		return err
	}
	switch current {
	case "":
		e.logln("Created DNS record " + host + " -> " + ip)
	default:
		e.logln("Updated DNS record " + host + ": " + current + " -> " + ip +
			" (devices pick this up within a minute)")
	}
	return nil
}

// --- Cloudflare API ---------------------------------------------------------

type cloudflare struct {
	token string
	base  string
	hc    *http.Client
}

func newCloudflare(token string) *cloudflare {
	return &cloudflare{
		token: strings.TrimSpace(token),
		base:  cloudflareAPI,
		hc:    &http.Client{Timeout: 15 * time.Second},
	}
}

// cfEnvelope is Cloudflare's uniform response wrapper. HTTP status alone isn't
// enough - the API can return 200 with success:false.
type cfEnvelope struct {
	Success bool            `json:"success"`
	Result  json.RawMessage `json:"result"`
	Errors  []struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"errors"`
}

func (c *cloudflare) do(method, path string, body any, out any) error {
	var rdr io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return err
		}
		rdr = bytes.NewReader(b)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, method, c.base+path, rdr)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.hc.Do(req)
	if err != nil {
		return fmt.Errorf("couldn't reach Cloudflare (%w) - is this computer online?", err)
	}
	defer resp.Body.Close()

	var env cfEnvelope
	if err := json.NewDecoder(resp.Body).Decode(&env); err != nil {
		return fmt.Errorf("unexpected reply from Cloudflare (HTTP %d)", resp.StatusCode)
	}
	if !env.Success {
		if len(env.Errors) > 0 {
			return fmt.Errorf("Cloudflare says: %s", env.Errors[0].Message)
		}
		return fmt.Errorf("Cloudflare rejected the request (HTTP %d)", resp.StatusCode)
	}
	if out != nil {
		return json.Unmarshal(env.Result, out)
	}
	return nil
}

// verify reports whether the token is live - a typo, a revoked token, or one from
// the wrong account, caught before any long build starts.
func (c *cloudflare) verify() error {
	return c.do("GET", "/user/tokens/verify", nil, nil)
}

// zoneFor finds which zone owns host, by longest suffix match against the zones the
// token can see. Chopping labels off the host instead would get .co.uk wrong, and
// would fail when the clinic's zone is itself a subdomain.
func (c *cloudflare) zoneFor(host string) (string, error) {
	var zones []struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	}
	if err := c.do("GET", "/zones?per_page=50", nil, &zones); err != nil {
		return "", err
	}
	best, bestID := "", ""
	for _, z := range zones {
		if host == z.Name || strings.HasSuffix(host, "."+z.Name) {
			if len(z.Name) > len(best) {
				best, bestID = z.Name, z.ID
			}
		}
	}
	if bestID == "" {
		return "", fmt.Errorf("this Cloudflare token doesn't cover %q - check it's scoped "+
			"to that domain's zone (see docs/tls.md)", host)
	}
	return bestID, nil
}

// findARecord returns the record's id and current address, or empty strings when no
// A record exists for host yet.
func (c *cloudflare) findARecord(zoneID, host string) (id, content string, err error) {
	var recs []struct {
		ID      string `json:"id"`
		Content string `json:"content"`
	}
	path := "/zones/" + zoneID + "/dns_records?type=A&name=" + host
	if err := c.do("GET", path, nil, &recs); err != nil {
		return "", "", err
	}
	if len(recs) == 0 {
		return "", "", nil
	}
	return recs[0].ID, recs[0].Content, nil
}

// putARecord updates the record when id is set, or creates it when it isn't.
// Proxied is always false: Cloudflare cannot proxy traffic to a private address, and
// an orange-cloud record would break the clinic entirely.
func (c *cloudflare) putARecord(zoneID, id, host, ip string) error {
	body := map[string]any{
		"type":    "A",
		"name":    host,
		"content": ip,
		"ttl":     dnsTTL,
		"proxied": false,
	}
	if id == "" {
		return c.do("POST", "/zones/"+zoneID+"/dns_records", body, nil)
	}
	return c.do("PUT", "/zones/"+zoneID+"/dns_records/"+id, body, nil)
}
