package dbuf

import "context"

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

func (s *dbufService) ReleasePackets(ctx context.Context, req *ReleasePacketsRequest) (*ReleasePacketsResponse, error) {
	if err := s.bq.ReleasePackets(int(req.BufferId)); err != nil {
		return nil, err
	}
	resp := &ReleasePacketsResponse{}

	return resp, nil
}
