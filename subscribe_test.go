package gnmi

import (
	"context"
	"testing"
	"time"

	"github.com/freeconf/yang/fc"
	pb_gnmi "github.com/openconfig/gnmi/proto/gnmi"
)

func TestSub(t *testing.T) {
	data := map[string]interface{}{
		"me": map[string]interface{}{
			"name":    "joe",
			"skill":   "manager",
			"address": "123 mockingbird lane.",
		},
		"users": []map[string]interface{}{
			{"name": "mary", "skill": "welder"},
			{"name": "john", "skill": "mechanic"},
		},
	}
	dev := newTestDevice(data)
	b, _ := dev.Browser("x")
	root := b.Root()
	prefix := root

	t.Run("onetime", func(t *testing.T) {
		opts := &pb_gnmi.Subscription{
			Mode: pb_gnmi.SubscriptionMode_SAMPLE,
			Path: &pb_gnmi.Path{},
		}
		var actual []byte
		sink := func(resp *pb_gnmi.SubscribeResponse) error {
			update := resp.Response.(*pb_gnmi.SubscribeResponse_Update)
			actual = update.Update.Update[0].Val.GetJsonVal()
			return nil
		}
		sub := newSubscription(dev, prefix, opts, sink)
		err := sub.execute()
		fc.AssertEqual(t, nil, err)
		// NOTE: same gold file as TestGet as they should match
		fc.Gold(t, *updateFlag, actual, "testdata/get-gold.json")
	})

	t.Run("onchange", func(t *testing.T) {
		opts := &pb_gnmi.Subscription{
			Mode:              pb_gnmi.SubscriptionMode_ON_CHANGE,
			SampleInterval:    10 * uint64(time.Millisecond),
			HeartbeatInterval: 40 * uint64(time.Millisecond),
		}
		var at time.Time
		sink := func(resp *pb_gnmi.SubscribeResponse) error {
			at = time.Now()
			return nil
		}
		sub := newSubscription(dev, prefix, opts, sink)
		err := sub.execute()
		fc.AssertEqual(t, nil, err)
		fc.AssertEqual(t, false, at.IsZero())
		t0 := at
		err = sub.execute()
		fc.AssertEqual(t, nil, err)
		fc.AssertEqual(t, t0, at)

		<-time.After(sub.getHeartbeatInterval())
		err = sub.execute()
		fc.AssertEqual(t, nil, err)
		fc.AssertEqual(t, false, t0 == at)
	})
}

func TestSubMgr(t *testing.T) {
	mgr := &subscriptionManager{}

	t.Run("invalid", func(t *testing.T) {
		ctx := context.Background()
		fc.AssertEqual(t, errNoSampleInterval, mgr.add(ctx, &dummySub{}))
	})

	t.Run("valid", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		sub := &dummySub{sample: time.Nanosecond}
		fc.AssertEqual(t, nil, mgr.add(ctx, sub))
		<-time.After(time.Millisecond)
		t0 := sub.at
		<-time.After(time.Millisecond)
		t1 := sub.at
		fc.AssertEqual(t, false, t0 == t1)
		<-time.After(time.Millisecond)
		t2 := sub.at
		fc.AssertEqual(t, false, t1 == t2)

		cancel()

		// give some time to cancel
		<-time.After(time.Millisecond)

		t3 := sub.at
		<-time.After(time.Millisecond)
		t4 := sub.at
		fc.AssertEqual(t, true, t3 == t4)
	})
}

type dummySub struct {
	sample time.Duration
	at     time.Time
}

func (d *dummySub) getSampleInterval() time.Duration {
	return d.sample
}

func (d *dummySub) execute() error {
	d.at = time.Now()
	return nil
}

func TestIsEqualValues(t *testing.T) {
	a1 := &pb_gnmi.TypedValue{
		Value: &pb_gnmi.TypedValue_JsonVal{
			JsonVal: []byte(`{"a":"A"}`),
		},
	}
	a2 := &pb_gnmi.TypedValue{
		Value: &pb_gnmi.TypedValue_JsonVal{
			JsonVal: []byte(`{"a":"A"}`),
		},
	}
	b := &pb_gnmi.TypedValue{
		Value: &pb_gnmi.TypedValue_JsonVal{
			JsonVal: []byte(`{"b":"B"}`),
		},
	}
	fc.AssertEqual(t, true, isEqualValues(a1, a2))
	fc.AssertEqual(t, false, isEqualValues(a1, b))
	fc.AssertEqual(t, true, isEqualValues(nil, nil))
	fc.AssertEqual(t, false, isEqualValues(a1, nil))
	fc.AssertEqual(t, false, isEqualValues(nil, a1))
	fc.AssertEqual(t, true, isEqualValues(a1, a1))
}
