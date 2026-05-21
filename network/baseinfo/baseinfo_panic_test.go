package baseinfo

import (
	"sync"
	"testing"
	"time"

	"github.com/oneclickvirt/basics/model"
)

func TestSafeFetchIPInfoRecoverPanic(t *testing.T) {
	fetch := func(string) (*model.IpInfo, error) {
		panic("boom")
	}
	ipInfo, err := safeFetchIPInfo(fetch, "tcp4")
	if err == nil {
		t.Fatalf("expected panic to be converted to error")
	}
	if ipInfo != nil {
		t.Fatalf("expected nil result after panic, got %+v", ipInfo)
	}
}

func TestExecuteFunctionsRecoverPanic(t *testing.T) {
	ipInfoChan := make(chan *ipInfoWithSource, 1)
	fetch := func(string) (*model.IpInfo, error) {
		panic("boom")
	}

	var wg sync.WaitGroup
	wg.Add(1)
	go executeFunctions("ipv4", fetch, "panicfetch", ipInfoChan, &wg)

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("executeFunctions did not finish")
	}

	select {
	case res := <-ipInfoChan:
		if res == nil {
			t.Fatal("expected channel message")
		}
		if res.info != nil {
			t.Fatalf("expected nil info when panic occurs, got %+v", res.info)
		}
		if res.source != "panicfetch" {
			t.Fatalf("unexpected source: %s", res.source)
		}
	default:
		t.Fatal("expected one result in channel")
	}
}
