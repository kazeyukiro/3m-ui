package realityscan

import (
	"context"
	"errors"
	"net"
	"net/netip"
	"slices"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"
)

type trackedDialConn struct {
	net.Conn
	closed atomic.Bool
}

func (c *trackedDialConn) Close() error {
	c.closed.Store(true)
	return nil
}

func addressLookup(addresses ...string) func(context.Context, string, string) ([]netip.Addr, error) {
	return func(context.Context, string, string) ([]netip.Addr, error) {
		ips := make([]netip.Addr, len(addresses))
		for i, address := range addresses {
			ips[i] = netip.MustParseAddr(address)
		}
		return ips, nil
	}
}

func TestPublicDialFallsBackBeforeProbeDeadline(t *testing.T) {
	for _, tc := range []struct {
		name      string
		addresses []string
	}{
		{"same family", []string{"1.1.1.1", "8.8.8.8"}},
		{"IPv6 blackhole", []string{"2001:4860:4860::8888", "2606:4700:4700::1111", "8.8.8.8"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				ctx, cancel := context.WithTimeout(context.Background(), probeTimeout)
				defer cancel()
				winner := &trackedDialConn{}
				var attempts, stopped atomic.Int32
				start := time.Now()
				conn, err := publicDialWith(ctx, "tcp", "example.com:443", addressLookup(tc.addresses...), func(ctx context.Context, network, address string) (net.Conn, error) {
					attempts.Add(1)
					if network != "tcp" {
						t.Errorf("network = %q", network)
					}
					if address == "8.8.8.8:443" {
						return winner, nil
					}
					<-ctx.Done()
					stopped.Add(1)
					return nil, ctx.Err()
				})
				if err != nil || conn != winner {
					t.Fatalf("got (%v, %v), want successful fallback", conn, err)
				}
				defer conn.Close()
				if elapsed := time.Since(start); elapsed != addressFallbackDelay || elapsed >= 300*time.Millisecond {
					t.Fatalf("fallback took %v, beyond the minimum eligibility window", elapsed)
				}
				synctest.Wait()
				if attempts.Load() != 2 || stopped.Load() != 1 || winner.closed.Load() {
					t.Fatalf("attempts=%d stopped=%d winner closed=%v", attempts.Load(), stopped.Load(), winner.closed.Load())
				}
			})
		})
	}
}

func TestPublicDialAdvancesPastTwoBlackholes(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), probeTimeout)
		defer cancel()
		winner := &trackedDialConn{}
		var active, peak, stopped atomic.Int32
		start := time.Now()
		conn, err := publicDialWith(ctx, "tcp", "example.com:443", addressLookup("1.1.1.1", "8.8.8.8", "9.9.9.9"), func(ctx context.Context, _, address string) (net.Conn, error) {
			n := active.Add(1)
			defer active.Add(-1)
			for old := peak.Load(); n > old && !peak.CompareAndSwap(old, n); old = peak.Load() {
			}
			if address == "9.9.9.9:443" {
				return winner, nil
			}
			<-ctx.Done()
			stopped.Add(1)
			return nil, ctx.Err()
		})
		if err != nil || conn != winner || time.Since(start) != addressDialTimeout {
			t.Fatalf("got (%v, %v) after %v", conn, err, time.Since(start))
		}
		defer conn.Close()
		synctest.Wait()
		if peak.Load() != maxAddressDials || active.Load() != 0 || stopped.Load() != 2 {
			t.Fatalf("peak=%d active=%d stopped=%d", peak.Load(), active.Load(), stopped.Load())
		}
	})
}

func TestPublicDialCancellationAndDeadline(t *testing.T) {
	for _, deadline := range []bool{false, true} {
		t.Run(map[bool]string{false: "cancel", true: "deadline"}[deadline], func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				const stopAfter = 2 * addressFallbackDelay
				ctx, cancel := context.WithCancel(context.Background())
				wantErr := context.Canceled
				if deadline {
					cancel()
					ctx, cancel = context.WithTimeout(context.Background(), stopAfter)
					wantErr = context.DeadlineExceeded
				} else {
					go func() {
						time.Sleep(stopAfter)
						cancel()
					}()
				}
				defer cancel()
				var started, stopped atomic.Int32
				start := time.Now()
				conn, err := publicDialWith(ctx, "tcp", "example.com:443", addressLookup("1.1.1.1", "8.8.8.8", "9.9.9.9"), func(ctx context.Context, _, _ string) (net.Conn, error) {
					started.Add(1)
					<-ctx.Done()
					stopped.Add(1)
					return nil, ctx.Err()
				})
				if conn != nil || !errors.Is(err, wantErr) || time.Since(start) != stopAfter {
					t.Fatalf("got (%v, %v) after %v", conn, err, time.Since(start))
				}
				synctest.Wait()
				if started.Load() != 2 || stopped.Load() != 2 {
					t.Fatalf("started=%d stopped=%d", started.Load(), stopped.Load())
				}
			})
		})
	}
}

func TestPublicDialClosesLateSuccessfulLoser(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		winner, loser := &trackedDialConn{}, &trackedDialConn{}
		conn, err := publicDialWith(context.Background(), "tcp", "example.com:443", addressLookup("1.1.1.1", "8.8.8.8"), func(ctx context.Context, _, address string) (net.Conn, error) {
			if address == "8.8.8.8:443" {
				return winner, nil
			}
			// Model a connection completing just as its dial was canceled.
			<-ctx.Done()
			return loser, nil
		})
		if err != nil || conn != winner {
			t.Fatalf("got (%v, %v)", conn, err)
		}
		defer conn.Close()
		synctest.Wait()
		if winner.closed.Load() || !loser.closed.Load() {
			t.Fatalf("winner closed=%v loser closed=%v", winner.closed.Load(), loser.closed.Load())
		}
	})
}

func TestPublicDialAllAddressesFail(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		wantErr := errors.New("connection refused")
		var attempts atomic.Int32
		conn, err := publicDialWith(context.Background(), "tcp", "example.com:443", addressLookup("1.1.1.1", "8.8.8.8", "9.9.9.9"), func(context.Context, string, string) (net.Conn, error) {
			attempts.Add(1)
			return nil, wantErr
		})
		if conn != nil || !errors.Is(err, wantErr) || attempts.Load() != 3 {
			t.Fatalf("got (%v, %v), attempts=%d", conn, err, attempts.Load())
		}
	})
}

func TestPublicDialRejectsPrivateAddressesBeforeDialing(t *testing.T) {
	for _, address := range []string{"127.0.0.1", "10.0.0.1", "::1", "fd00::1", "::ffff:192.168.1.1"} {
		t.Run(address, func(t *testing.T) {
			conn, err := publicDialWith(context.Background(), "tcp", "example.com:443", addressLookup("1.1.1.1", address), func(context.Context, string, string) (net.Conn, error) {
				t.Fatal("dialed before validating all DNS addresses")
				return nil, nil
			})
			if conn != nil || err == nil {
				t.Fatalf("accepted mixed public/private DNS results: (%v, %v)", conn, err)
			}
		})
	}
}

func TestInterleaveAddresses(t *testing.T) {
	lookup := addressLookup("2001:4860:4860::8888", "2606:4700:4700::1111", "1.1.1.1", "8.8.8.8", "::ffff:1.1.1.1", "9.9.9.9")
	ips, _ := lookup(context.Background(), "ip", "example.com")
	got := interleaveAddresses(ips)
	want, _ := addressLookup("2001:4860:4860::8888", "1.1.1.1", "2606:4700:4700::1111", "8.8.8.8", "9.9.9.9")(context.Background(), "ip", "example.com")
	if !slices.Equal(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}
