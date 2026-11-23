package cast

import (
	"context"
	"fmt"
	"net"
	"sort"
	"strconv"
	"strings"
	"time"

	"golang.org/x/net/dns/dnsmessage"
	"golang.org/x/net/ipv4"
)

const (
	mdnsServiceName = "_googlecast._tcp.local."
	mdnsMulticastIP = "224.0.0.251"
	mdnsPort        = 5353

	// Google Cast capability bitmask (TXT field "ca").
	capVideoOut = 1 << 0
	capAudioOut = 1 << 2
)

// Discoverer finds Chromecast receivers on the LAN via mDNS and static config.
type Discoverer struct {
	StaticDevices []Device
	BrowseTimeout time.Duration
}

// Discover merges configured static devices with live _googlecast._tcp.local mDNS responses.
func (d *Discoverer) Discover(ctx context.Context) ([]Device, error) {
	byID := make(map[string]Device)
	for _, dev := range d.StaticDevices {
		if dev.Port == 0 {
			dev.Port = 8009
		}
		if !dev.HasAudio && !dev.HasVideo {
			dev.HasAudio = true
		}
		dev.IsStatic = true
		byID[dev.ID] = dev
	}

	timeout := d.BrowseTimeout
	if timeout <= 0 {
		timeout = 1200 * time.Millisecond
	}
	mdnsDevices, _ := browseMDNS(ctx, timeout)
	for _, dev := range mdnsDevices {
		if _, exists := byID[dev.ID]; !exists {
			byID[dev.ID] = dev
		}
	}

	result := make([]Device, 0, len(byID))
	for _, dev := range byID {
		result = append(result, dev)
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].Name != result[j].Name {
			return result[i].Name < result[j].Name
		}
		return result[i].ID < result[j].ID
	})
	return result, nil
}

// ParseMDNSResponse extracts Cast Device entries from a raw DNS-SD mDNS packet.
func ParseMDNSResponse(packet []byte) []Device {
	var p dnsmessage.Parser
	if _, err := p.Start(packet); err != nil {
		return nil
	}
	_ = p.SkipAllQuestions()

	type srvInfo struct {
		host string
		port int
	}

	srvs := make(map[string]srvInfo)
	txts := make(map[string]map[string]string)
	ips := make(map[string]string)

	consume := func(hdr dnsmessage.ResourceHeader, readBody func() error) {
		_ = readBody()
	}

	parseSections := func(next func() (dnsmessage.ResourceHeader, error)) {
		for {
			hdr, err := next()
			if err != nil {
				return
			}
			name := strings.TrimSuffix(strings.ToLower(hdr.Name.String()), ".")
			switch hdr.Type {
			case dnsmessage.TypeSRV:
				consume(hdr, func() error {
					res, err := p.SRVResource()
					if err != nil {
						return err
					}
					srvs[name] = srvInfo{
						host: strings.TrimSuffix(strings.ToLower(res.Target.String()), "."),
						port: int(res.Port),
					}
					return nil
				})
			case dnsmessage.TypeTXT:
				consume(hdr, func() error {
					res, err := p.TXTResource()
					if err != nil {
						return err
					}
					kv := make(map[string]string, len(res.TXT))
					for _, entry := range res.TXT {
						if k, v, ok := strings.Cut(entry, "="); ok {
							kv[k] = v
						}
					}
					txts[name] = kv
					return nil
				})
			case dnsmessage.TypeA:
				consume(hdr, func() error {
					res, err := p.AResource()
					if err != nil {
						return err
					}
					ips[name] = net.IP(res.A[:]).String()
					return nil
				})
			default:
				_ = p.SkipAnswer()
			}
		}
	}

	// Walk Answer, Authority, Additional sections.
	parseSections(p.AnswerHeader)
	_ = p.SkipAllAuthorities()
	parseSections(p.AdditionalHeader)

	var devices []Device
	for svcName, srv := range srvs {
		if !strings.Contains(svcName, "_googlecast._tcp") {
			continue
		}
		ip := ips[srv.host]
		if ip == "" {
			continue
		}
		kv := txts[svcName]
		id := kv["id"]
		if id == "" {
			id = fmt.Sprintf("%s:%d", ip, srv.port)
		}
		friendly := kv["fn"]
		if friendly == "" {
			friendly = svcName
		}
		model := kv["md"]
		hasVideo, hasAudio := parseCastCapabilities(kv["ca"])
		devices = append(devices, Device{
			ID:       id,
			Name:     friendly,
			Model:    model,
			Host:     ip,
			Port:     srv.port,
			HasVideo: hasVideo,
			HasAudio: hasAudio,
		})
	}
	return devices
}

func parseCastCapabilities(raw string) (hasVideo, hasAudio bool) {
	if raw == "" {
		return false, true
	}
	val, err := strconv.Atoi(raw)
	if err != nil {
		return false, true
	}
	hasVideo = (val & capVideoOut) != 0
	hasAudio = (val&capAudioOut) != 0 || hasVideo
	return hasVideo, hasAudio
}

func browseMDNS(ctx context.Context, timeout time.Duration) ([]Device, error) {
	qName, err := dnsmessage.NewName(mdnsServiceName)
	if err != nil {
		return nil, err
	}
	query := dnsmessage.Message{
		Header: dnsmessage.Header{RecursionDesired: false},
		Questions: []dnsmessage.Question{
			{
				Name:  qName,
				Type:  dnsmessage.TypePTR,
				Class: dnsmessage.ClassINET,
			},
		},
	}
	packed, err := query.Pack()
	if err != nil {
		return nil, err
	}

	conn, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4zero, Port: 0})
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	pconn := ipv4.NewPacketConn(conn)
	_ = pconn.SetMulticastTTL(2)

	dst := &net.UDPAddr{IP: net.ParseIP(mdnsMulticastIP), Port: mdnsPort}
	if _, err := conn.WriteToUDP(packed, dst); err != nil {
		return nil, err
	}

	deadline := time.Now().Add(timeout)
	if ctxDeadline, ok := ctx.Deadline(); ok && ctxDeadline.Before(deadline) {
		deadline = ctxDeadline
	}
	_ = conn.SetReadDeadline(deadline)

	byID := make(map[string]Device)
	buf := make([]byte, 8192)
	for {
		if ctx.Err() != nil {
			break
		}
		n, _, err := conn.ReadFromUDP(buf)
		if err != nil {
			break
		}
		for _, dev := range ParseMDNSResponse(buf[:n]) {
			byID[dev.ID] = dev
		}
	}

	out := make([]Device, 0, len(byID))
	for _, dev := range byID {
		out = append(out, dev)
	}
	return out, nil
}
