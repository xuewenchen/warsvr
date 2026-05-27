package router

import (
	"cardwar/conf"
	"testing"
)

func TestBuildRouteIndex(t *testing.T) {
	cfg := conf.GatewayConfig{
		Routes: map[string]conf.BackendRoute{
			"chatsvr": {Forward: []uint32{5}, RouteKey: "playerId"},
		},
	}
	idx := BuildRouteIndex(cfg)
	if len(idx) != 1 {
		t.Fatalf("expected 1 route, got %d", len(idx))
	}
	r := idx[5]
	if r == nil {
		t.Fatal("expected route for msgID 5")
	}
	if r.Backend != "chatsvr" {
		t.Errorf("expected backend chatsvr, got %s", r.Backend)
	}
	if r.RouteKey != "playerId" {
		t.Errorf("expected routeKey playerId, got %s", r.RouteKey)
	}
}

func TestBuildRouteIndex_MultipleBackends(t *testing.T) {
	cfg := conf.GatewayConfig{
		Routes: map[string]conf.BackendRoute{
			"chatsvr": {Forward: []uint32{5}, RouteKey: "playerId"},
			"roomsvr": {Forward: []uint32{7, 8}, RouteKey: "connId"},
		},
	}
	idx := BuildRouteIndex(cfg)
	if len(idx) != 3 {
		t.Fatalf("expected 3 routes, got %d", len(idx))
	}
	if idx[5].Backend != "chatsvr" {
		t.Error("msgID 5 should go to chatsvr")
	}
	if idx[7].Backend != "roomsvr" {
		t.Error("msgID 7 should go to roomsvr")
	}
	if idx[8].Backend != "roomsvr" {
		t.Error("msgID 8 should go to roomsvr")
	}
}

func TestBuildRouteIndex_Empty(t *testing.T) {
	idx := BuildRouteIndex(conf.GatewayConfig{})
	if len(idx) != 0 {
		t.Fatalf("expected 0 routes, got %d", len(idx))
	}
}

func TestGatewayRef_RouteFor(t *testing.T) {
	gw := &GatewayRef{}
	gw.SetRoutes(map[uint32]*BackendRouteInfo{
		5: {Backend: "chatsvr", RouteKey: "playerId"},
	})
	if r := gw.RouteFor(5); r == nil || r.Backend != "chatsvr" {
		t.Error("RouteFor(5) should return chatsvr route")
	}
	if r := gw.RouteFor(99); r != nil {
		t.Error("RouteFor(99) should return nil")
	}
}
