package cast_test

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"golang.org/x/net/dns/dnsmessage"

	"github.com/shelfd/shelfd/internal/cast"
	"github.com/shelfd/shelfd/internal/cast/casttest"
)

func TestCastMessageWireRoundTripAndBounds(t *testing.T) {
	orig := &cast.CastMessage{
		ProtocolVersion: cast.ProtocolVersionCastV2,
		SourceID:        "sender-shelfd",
		DestinationID:   "receiver-0",
		Namespace:       cast.NamespaceHeartbeat,
		PayloadType:     cast.PayloadTypeString,
		PayloadUTF8:     `{"type":"PING"}`,
	}

	var buf bytes.Buffer
	if err := cast.WriteFrame(&buf, orig); err != nil {
		t.Fatalf("WriteFrame: %v", err)
	}
	got, err := cast.ReadFrame(&buf)
	if err != nil {
		t.Fatalf("ReadFrame: %v", err)
	}
	if got.SourceID != orig.SourceID || got.DestinationID != orig.DestinationID ||
		got.Namespace != orig.Namespace || got.PayloadUTF8 != orig.PayloadUTF8 {
		t.Fatalf("round-trip mismatch: got %+v, want %+v", got, orig)
	}

	oversized := &cast.CastMessage{
		ProtocolVersion: cast.ProtocolVersionCastV2,
		SourceID:        "sender-shelfd",
		DestinationID:   "receiver-0",
		Namespace:       cast.NamespaceMedia,
		PayloadType:     cast.PayloadTypeString,
		PayloadUTF8:     strings.Repeat("x", cast.MaxPacketSize+10),
	}
	if _, err := cast.MarshalCastMessage(oversized); err == nil {
		t.Fatalf("expected error for oversized CastMessage packet")
	}
}

func TestParseMDNSResponse(t *testing.T) {
	svcName, _ := dnsmessage.NewName("LivingRoom._googlecast._tcp.local.")
	hostName, _ := dnsmessage.NewName("livingroom.local.")
	msg := dnsmessage.Message{
		Header: dnsmessage.Header{Response: true, Authoritative: true},
		Answers: []dnsmessage.Resource{
			{
				Header: dnsmessage.ResourceHeader{Name: svcName, Type: dnsmessage.TypeSRV, Class: dnsmessage.ClassINET, TTL: 120},
				Body:   &dnsmessage.SRVResource{Port: 8009, Target: hostName},
			},
			{
				Header: dnsmessage.ResourceHeader{Name: svcName, Type: dnsmessage.TypeTXT, Class: dnsmessage.ClassINET, TTL: 120},
				Body:   &dnsmessage.TXTResource{TXT: []string{"id=tv-001", "fn=Living Room TV", "md=Chromecast Ultra", "ca=196613"}},
			},
		},
		Additionals: []dnsmessage.Resource{
			{
				Header: dnsmessage.ResourceHeader{Name: hostName, Type: dnsmessage.TypeA, Class: dnsmessage.ClassINET, TTL: 120},
				Body:   &dnsmessage.AResource{A: [4]byte{192, 168, 1, 42}},
			},
		},
	}
	raw, err := msg.Pack()
	if err != nil {
		t.Fatalf("pack mdns: %v", err)
	}

	devs := cast.ParseMDNSResponse(raw)
	if len(devs) != 1 {
		t.Fatalf("expected 1 device, got %d", len(devs))
	}
	d := devs[0]
	if d.ID != "tv-001" || d.Name != "Living Room TV" || d.Host != "192.168.1.42" || d.Port != 8009 || !d.HasVideo || !d.HasAudio {
		t.Fatalf("unexpected parsed device: %+v", d)
	}
}

func TestClientLifecycleAgainstLoopbackReceiver(t *testing.T) {
	rec, err := casttest.StartReceiver(false)
	if err != nil {
		t.Fatalf("StartReceiver: %v", err)
	}
	defer rec.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	client, err := cast.DialClient(ctx, rec.Host, rec.Port, cast.ClientCallbacks{})
	if err != nil {
		t.Fatalf("DialClient: %v", err)
	}
	defer client.Close()

	rs, err := client.Launch(ctx, cast.DefaultMediaReceiverAppID)
	if err != nil {
		t.Fatalf("Launch: %v", err)
	}
	if rs.TransportID == "" {
		t.Fatalf("expected non-empty transportId")
	}

	ms, err := client.Load(ctx, cast.MediaInformation{
		ContentID:   "http://127.0.0.1:8080/api/v1/audiobooks/b1/stream",
		ContentType: "audio/mpeg",
		StreamType:  "BUFFERED",
		Duration:    120,
		Metadata:    cast.MediaMetadata{MetadataType: 3, Title: "Peter Rabbit"},
	}, 12.5, []int{1})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if ms.PlayerState != "PLAYING" || ms.CurrentTime < 12.5 {
		t.Fatalf("unexpected load status: %+v", ms)
	}

	ms, err = client.SetPlaybackRate(ctx, 1.5)
	if err != nil || ms.PlaybackRate != 1.5 {
		t.Fatalf("SetPlaybackRate failed: %+v %v", ms, err)
	}

	ms, err = client.Pause(ctx)
	if err != nil || ms.PlayerState != "PAUSED" {
		t.Fatalf("Pause failed: %+v %v", ms, err)
	}

	ms, err = client.Seek(ctx, 45.0)
	if err != nil || ms.CurrentTime < 44.9 {
		t.Fatalf("Seek failed: %+v %v", ms, err)
	}

	ms, err = client.EditTracksInfo(ctx, []int{})
	if err != nil || len(ms.ActiveTrackIDs) != 0 {
		t.Fatalf("EditTracksInfo disable failed: %+v %v", ms, err)
	}

	vol := 0.35
	rs, err = client.SetVolume(ctx, &vol, nil)
	if err != nil || rs.Volume < 0.34 || rs.Volume > 0.36 {
		t.Fatalf("SetVolume failed: %+v %v", rs, err)
	}
}
