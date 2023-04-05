package gnmi

import (
	"bytes"
	"context"
	"errors"
	"reflect"
	"time"

	"github.com/freeconf/restconf/device"
	"github.com/freeconf/yang/fc"
	"github.com/freeconf/yang/node"
	pb_gnmi "github.com/openconfig/gnmi/proto/gnmi"
)

type subscriptionSink func(*pb_gnmi.SubscribeResponse) error

type subscriptionManager struct {
	subs []reoccurringSubscription
}

type reoccurringSubscription interface {
	execute() error
	getSampleInterval() time.Duration
}

var errNoSampleInterval = errors.New("no sample interval given")

type subService struct {
	subMgr subscriptionManager
}

func (s *subService) subscribe(d device.Device, server pb_gnmi.GNMI_SubscribeServer) error {
	for {
		req, err := server.Recv()
		if err != nil && req != nil {
			return err
		}
		if req != nil {
			if err = s.handleSubscribeList(d, server.Context(), req, server.Send); err != nil {
				return err
			}
		}
	}
}

// according to gNMI spec, this is for config or metrics only, not YANG notifications!
func (s *subService) handleSubscribeList(d device.Device, ctx context.Context, req *pb_gnmi.SubscribeRequest, sink subscriptionSink) error {
	list := req.GetSubscribe()

	prefix, err := selectPath(d, list.UseModels, list.Prefix)
	if err != nil {
		return err
	}

	for _, subReq := range list.Subscription {
		fc.Debug.Printf("new sub mode = %d", list.Mode)

		sub := newSubscription(d, prefix, subReq, sink)

		// execute once sychronously avoids kicking off threads and runs thru
		// sub to validate paths
		if err := sub.execute(); err != nil {
			return err
		}
		if list.Mode != pb_gnmi.SubscriptionList_ONCE {
			if err := s.subMgr.add(ctx, sub); err != nil {
				return err
			}
		}
	}
	return nil
}

func (mgr *subscriptionManager) add(ctx context.Context, sub reoccurringSubscription) error {
	fc.Debug.Printf("starting ticker with sample rate %s", sub.getSampleInterval())
	sample := sub.getSampleInterval()
	if sample == 0 {
		return errNoSampleInterval
	}
	mgr.subs = append(mgr.subs, sub)
	t := time.NewTicker(sample)
	go func() {
		defer t.Stop()
		for {
			select {
			case <-t.C:
				fc.Debug.Printf("ticker fired")
				if err := sub.execute(); err != nil {
					// unclear how i should handle this
					fc.Err.Printf("cannot get sub %s", err)
				}
			case <-ctx.Done():
				return
			}
		}
	}()
	return nil
}

type subscription struct {
	device        device.Device
	prefix        *node.Selection
	sink          subscriptionSink
	opts          *pb_gnmi.Subscription
	previousValue *pb_gnmi.TypedValue
	previousTime  time.Time
}

func (s *subscription) getHeartbeatInterval() time.Duration {
	return time.Duration(s.opts.HeartbeatInterval) * time.Nanosecond
}

func (s *subscription) getSampleInterval() time.Duration {
	if s.opts.SampleInterval == 0 {
		return s.getHeartbeatInterval()
	}
	return time.Duration(s.opts.SampleInterval) * time.Nanosecond
}

func newSubscription(d device.Device, prefix *node.Selection, opts *pb_gnmi.Subscription, sink subscriptionSink) *subscription {
	return &subscription{
		device: d,
		prefix: prefix,
		opts:   opts,
		sink:   sink,
	}
}

func (s *subscription) execute() error {
	sel, err := advanceSelection(s.device, s.prefix, s.opts.Path)
	fc.Debug.Printf("sub request %s", sel.Path)
	if err != nil {
		return err
	}
	val, err := getVal(sel)
	if err != nil {
		return err
	}

	now := time.Now()
	if s.previousValue != nil {
		if now.Sub(s.previousTime) < s.getHeartbeatInterval() {
			if s.opts.Mode == pb_gnmi.SubscriptionMode_ON_CHANGE {
				if isEqualValues(s.previousValue, val) {
					return nil
				}
			}
		}
	}

	update := &pb_gnmi.Update{
		Path: s.opts.Path,
		Val:  val,
	}
	resp := &pb_gnmi.SubscribeResponse{
		Response: &pb_gnmi.SubscribeResponse_Update{
			Update: &pb_gnmi.Notification{
				Timestamp: now.UnixNano(),
				Update:    []*pb_gnmi.Update{update},
			},
		},
	}
	s.sink(resp)
	s.previousValue = val
	s.previousTime = now
	return nil
}

func isEqualValues(a, b *pb_gnmi.TypedValue) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	t := reflect.TypeOf(a.Value)
	if t != reflect.TypeOf(b.Value) {
		return false
	}
	switch a.Value.(type) {
	case *pb_gnmi.TypedValue_JsonVal:
		return bytes.Equal(a.GetJsonVal(), b.GetJsonVal())
	case *pb_gnmi.TypedValue_JsonIetfVal:
		return bytes.Equal(a.GetJsonIetfVal(), b.GetJsonIetfVal())
	}
	return false
}
