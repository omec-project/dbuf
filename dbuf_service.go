package dbuf

import (
	"context"
	"log"
)

type dbufService struct {
	UnimplementedDbufServiceServer
	bq *BufferQueue
}

func newDbufService(bq *BufferQueue) *dbufService {
	s := &dbufService{}
	s.bq = bq
	return s
}

func (s *dbufService) GetCurrentState(ctx context.Context, req *GetCurrentStateRequest) (*GetCurrentStateResponse, error) {
	state := s.bq.GetState()
	resp := &GetCurrentStateResponse{}
	resp.FreeBuffers = uint64(state.freeBufferIds)
	resp.MaximumBuffers = uint64(state.maximumBufferIds)
	resp.FreeMemory = uint64(state.freeMemory)
	resp.MaximumMemory = uint64(state.maximumMemory)

	return resp, nil
}

func (s *dbufService) GetQueueState(ctx context.Context, req *GetQueueStateRequest) (*GetQueueStateResponse, error) {
	state, err := s.bq.GetQueueState(req.QueueId)
	if err != nil {
		return nil, err
	}

	return &state, nil
}

func (s *dbufService) ReleasePackets(ctx context.Context, req *ReleasePacketsRequest) (*ReleasePacketsResponse, error) {
	if err := s.bq.ReleasePackets(uint32(req.BufferId)); err != nil {
		return nil, err
	}
	resp := &ReleasePacketsResponse{}

	return resp, nil
}

func (s *dbufService) Subscribe(req *SubscribeRequest, stream DbufService_SubscribeServer) error {
	ch := make(chan Notification, 16)
	if err := s.bq.RegisterSubscriber(ch); err != nil {
		return err
	}
	defer s.bq.UnregisterSubscriber(ch)
	ready := &Notification{MessageType: &Notification_Ready_{&Notification_Ready{}}}
	if err := stream.Send(ready); err != nil {
		return err
	}

readLoop:
	for {
		select {
		case <-stream.Context().Done():
			log.Print("Client cancelled subscription")
			break readLoop
		case n := <-ch:
			log.Printf("Sending notification %v", n)
			if err := stream.Send(&n); err != nil {
				return err
			}
		}
	}

	return nil
}
