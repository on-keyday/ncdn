package main

import (
	"context"
	"net/netip"

	gobgpapi "github.com/osrg/gobgp/api"
	"github.com/osrg/gobgp/pkg/server"
	"google.golang.org/protobuf/protoadapt"
	"google.golang.org/protobuf/types/known/anypb"
)

type BGPManager struct {
	Server *server.BgpServer
	VIP    netip.Prefix
	SelfIP netip.Addr
}

func SetupIBGP(asn uint32, selfIP, peerIP netip.Addr) (*BGPManager, error) {
	bgpServe := server.NewBgpServer(server.GrpcListenAddress("localhost:50051"))
	err := bgpServe.AddPeer(context.Background(), &gobgpapi.AddPeerRequest{
		Peer: &gobgpapi.Peer{
			Conf: &gobgpapi.PeerConf{
				PeerAs:          asn,
				NeighborAddress: peerIP.String(),
			},
		},
	})
	if err != nil {
		return nil, err
	}
	err = bgpServe.StartBgp(context.Background(), &gobgpapi.StartBgpRequest{
		Global: &gobgpapi.Global{
			As:         asn,
			RouterId:   selfIP.String(),
			ListenPort: -1, // won't listen on a port
		},
	})
	if err != nil {
		return nil, err
	}
	return &BGPManager{
		Server: bgpServe,
	}, nil
}

func (b *BGPManager) UpdateVIP(vip netip.Prefix) error {
	nextHopAttr := &gobgpapi.NextHopAttribute{
		NextHop: b.SelfIP.String(),
	}
	nextHopAny, err := anypb.New(protoadapt.MessageV2Of(nextHopAttr))
	if err != nil {
		return err
	}
	if b.VIP.IsValid() {
		if b.VIP == vip {
			return nil // No change needed
		}
		// Remove the old VIP path
		nlri := &gobgpapi.IPAddressPrefix{
			PrefixLen: uint32(b.VIP.Bits()),
			Prefix:    b.VIP.Addr().String(),
		}
		anyP, err := anypb.New(protoadapt.MessageV2Of(nlri))
		if err != nil {
			return err
		}
		err = b.Server.DeletePath(context.Background(), &gobgpapi.DeletePathRequest{
			Path: &gobgpapi.Path{
				Nlri: anyP,
				Pattrs: []*anypb.Any{
					nextHopAny,
				},
			},
		})
		if err != nil {
			return err
		}
	}
	nlri := &gobgpapi.IPAddressPrefix{
		PrefixLen: uint32(vip.Bits()),
		Prefix:    vip.Addr().String(),
	}
	anyP, err := anypb.New(protoadapt.MessageV2Of(nlri))
	if err != nil {
		return err
	}
	_, err = b.Server.AddPath(context.Background(), &gobgpapi.AddPathRequest{
		Path: &gobgpapi.Path{
			Nlri: anyP,
			Pattrs: []*anypb.Any{
				nextHopAny,
			},
		},
	})
	if err != nil {
		return err
	}
	b.VIP = vip
	return err
}
