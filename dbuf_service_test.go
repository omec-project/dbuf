package dbuf

import (
	"context"
	"github.com/golang/mock/gomock"
	"github.com/golang/protobuf/proto"
	. "github.com/omec-project/dbuf/api"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"net"
	"strings"
	"testing"
	"time"
)

var defaultCtx = context.Background()

const (
	queueId1 = uint64(1)
)

func TestDbufService_GetDbufState(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mockBufferQueue := NewMockQueueManagerInterface(ctrl)
	server, client := setupDbufServiceOrDie(t, mockBufferQueue)
	defer shutdownGrpcServerOrDie(t, server)

	someDbufState := GetDbufStateResponse{
		MaximumQueues:   10,
		AllocatedQueues: 20,
		EmptyQueues:     30,
		MaximumMemory:   40,
		FreeMemory:      50,
	}
	mockBufferQueue.EXPECT().GetState().Return(someDbufState)

	resp, err := client.GetDbufState(defaultCtx, &GetDbufStateRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if !proto.Equal(resp, &someDbufState) {
		t.Fatal("got:", resp, "expected:", someDbufState)
	}
}

func TestDbufService_GetQueueState(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mockBufferQueue := NewMockQueueManagerInterface(ctrl)
	server, client := setupDbufServiceOrDie(t, mockBufferQueue)
	defer shutdownGrpcServerOrDie(t, server)

	queueId := uint64(1)
	queueState := GetQueueStateResponse{
		MaximumBuffers: 10,
		FreeBuffers:    20,
		MaximumMemory:  30,
		FreeMemory:     40,
		State:          GetQueueStateResponse_QUEUE_STATE_BUFFERING,
	}
	mockBufferQueue.EXPECT().GetQueueState(queueId).Return(queueState, nil)

	req := &GetQueueStateRequest{
		QueueId: queueId,
	}
	resp, err := client.GetQueueState(defaultCtx, req)
	if err != nil {
		t.Fatal(err)
	}
	if !proto.Equal(resp, &queueState) {
		t.Fatal("got:", resp, "expected:", queueState)
	}
}

func assertStatusErrorMessage(t *testing.T, s *status.Status, msg string) {
	if !strings.Contains(s.Message(), msg) {
		t.Fatalf("Wrong error message.\nexpected: \"%v\"\ngot: \"%v\"", msg, s.Message())
	}
}

func assertStatusErrorCode(t *testing.T, s *status.Status, code codes.Code) {
	if s.Code() != code {
		t.Fatalf("Wrong error code.\nexpected: %v\ngot: %v", code, s.Code())
	}
}

func setupDbufServiceOrDie(t *testing.T, bq QueueManagerInterface) (server *grpc.Server, client DbufServiceClient) {
	server = grpc.NewServer()
	RegisterDbufServiceServer(server, newDbufService(bq))
	lis, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		t.Fatal(err)
	}
	go server.Serve(lis)

	clientConn, err := grpc.Dial(
		lis.Addr().String(), grpc.WithInsecure(), grpc.WithBlock(), grpc.WithTimeout(time.Second),
	)
	if err != nil {
		t.Fatal(err)
	}
	client = NewDbufServiceClient(clientConn)

	return
}

func shutdownGrpcServerOrDie(t *testing.T, server *grpc.Server) {
	server.Stop()
}

func TestDbufService_ModifyQueue(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	t.Run("ReleaseSuccess", func(t *testing.T) {
		mockBufferQueue := NewMockQueueManagerInterface(ctrl)
		server, client := setupDbufServiceOrDie(t, mockBufferQueue)
		defer shutdownGrpcServerOrDie(t, server)

		mockBufferQueue.EXPECT().ReleasePackets(uint32(queueId1), gomock.Any(), false, false).Return(nil)
		req := &ModifyQueueRequest{
			QueueId:            queueId1,
			Action:             ModifyQueueRequest_QUEUE_ACTION_RELEASE,
			DestinationAddress: "localhost:2521",
		}
		_, err := client.ModifyQueue(defaultCtx, req)
		if err != nil {
			t.Fatal("ModifyQueue failed:", err)
		}
	})

	t.Run("ReleaseAndPassthroughSuccess", func(t *testing.T) {
		mockBufferQueue := NewMockQueueManagerInterface(ctrl)
		server, client := setupDbufServiceOrDie(t, mockBufferQueue)
		defer shutdownGrpcServerOrDie(t, server)

		mockBufferQueue.EXPECT().ReleasePackets(uint32(queueId1), gomock.Any(), false, true).Return(nil)
		req := &ModifyQueueRequest{
			QueueId:            queueId1,
			Action:             ModifyQueueRequest_QUEUE_ACTION_RELEASE_AND_PASSTHROUGH,
			DestinationAddress: "localhost:2521",
		}
		_, err := client.ModifyQueue(defaultCtx, req)
		if err != nil {
			t.Fatal("ModifyQueue failed:", err)
		}
	})

	t.Run("DropSuccess", func(t *testing.T) {
		mockBufferQueue := NewMockQueueManagerInterface(ctrl)
		server, client := setupDbufServiceOrDie(t, mockBufferQueue)
		defer shutdownGrpcServerOrDie(t, server)

		mockBufferQueue.EXPECT().ReleasePackets(uint32(queueId1), gomock.Any(), true, false).Return(nil)
		req := &ModifyQueueRequest{
			QueueId:            queueId1,
			Action:             ModifyQueueRequest_QUEUE_ACTION_DROP,
			DestinationAddress: "localhost:2521",
		}
		_, err := client.ModifyQueue(defaultCtx, req)
		if err != nil {
			t.Fatal("ModifyQueue failed:", err)
		}
	})

	t.Run("InvalidQueueFail", func(t *testing.T) {
		mockBufferQueue := NewMockQueueManagerInterface(ctrl)
		server, client := setupDbufServiceOrDie(t, mockBufferQueue)
		defer shutdownGrpcServerOrDie(t, server)

		mockBufferQueue.EXPECT().ReleasePackets(uint32(queueId1), gomock.Any(), gomock.Any(), gomock.Any()).Return(status.Error(codes.NotFound, "some message"))
		req := &ModifyQueueRequest{
			QueueId: queueId1,
			Action:  ModifyQueueRequest_QUEUE_ACTION_RELEASE,
		}
		resp, err := client.ModifyQueue(defaultCtx, req)
		s := status.Convert(err)
		assertStatusErrorCode(t, s, codes.NotFound)
		if resp != nil {
			t.Fatal("Response should be empty on error, got: ", resp)
		}
	})

	t.Run("InvalidActionFail", func(t *testing.T) {
		mockBufferQueue := NewMockQueueManagerInterface(ctrl)
		server, client := setupDbufServiceOrDie(t, mockBufferQueue)
		defer shutdownGrpcServerOrDie(t, server)

		req := &ModifyQueueRequest{
			QueueId: queueId1,
			Action:  ModifyQueueRequest_QUEUE_ACTION_INVALID,
		}
		resp, err := client.ModifyQueue(defaultCtx, req)
		s := status.Convert(err)
		assertStatusErrorCode(t, s, codes.InvalidArgument)
		assertStatusErrorMessage(t, s, "unknown queue operation")
		if resp != nil {
			t.Fatal("Response should be empty on error, got: ", resp)
		}
	})

	t.Run("InvalidDestinationFail", func(t *testing.T) {
		mockBufferQueue := NewMockQueueManagerInterface(ctrl)
		server, client := setupDbufServiceOrDie(t, mockBufferQueue)
		defer shutdownGrpcServerOrDie(t, server)

		req := &ModifyQueueRequest{
			QueueId:            1,
			DestinationAddress: "invalid",
		}
		resp, err := client.ModifyQueue(defaultCtx, req)
		s, ok := status.FromError(err)
		if !ok {
			t.Fatal("FromError failed")
		}
		assertStatusErrorCode(t, s, codes.InvalidArgument)
		assertStatusErrorMessage(t, s, "could not be parsed")
		if resp != nil {
			t.Fatal("Response should be empty on error, got: ", resp)
		}
	})
}

func TestDbufService_Subscribe(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	t.Run("SubscribeSuccess", func(t *testing.T) {
		mockBufferQueue := NewMockQueueManagerInterface(ctrl)
		server, client := setupDbufServiceOrDie(t, mockBufferQueue)
		defer shutdownGrpcServerOrDie(t, server)

		var ch chan Notification
		mockBufferQueue.EXPECT().RegisterSubscriber(gomock.Any()).DoAndReturn(
			func(ch_ chan Notification) error {
				ch = ch_
				mockBufferQueue.EXPECT().UnregisterSubscriber(gomock.Eq(ch)).Return(nil)
				return nil
			})
		ctx, cancel := context.WithCancel(defaultCtx)
		clientStream, err := client.Subscribe(ctx, &SubscribeRequest{})
		if err != nil {
			t.Fatal("Subscribe failed:", err)
		}
		n, err := clientStream.Recv()
		if err != nil {
			t.Fatal("Recv error: ", err)
		}
		if ready := n.GetReady(); ready == nil {
			t.Fatal("Server did not respond with ready")
		}
		// Send a FirstBuffer notification.
		firstBufNotification := Notification{
			MessageType: &Notification_FirstBuffer_{&Notification_FirstBuffer{NewBufferId: uint32(queueId1)}},
		}
		ch <- firstBufNotification
		n, err = clientStream.Recv()
		if err != nil {
			t.Fatal("Recv error: ", err)
		}
		if first := n.GetFirstBuffer(); first == nil {
			t.Fatal("Server did not respond with first buffer")
		}

		cancel()
		// Give the service a chance to exit the receive loop and unregister our subscriber.
		time.Sleep(time.Millisecond * 1)
	})

	// TODO(max): make it work reliably or remove
	t.Run("SubscribePreReadyCloseFail", func(t *testing.T) {
		mockBufferQueue := NewMockQueueManagerInterface(ctrl)
		server, client := setupDbufServiceOrDie(t, mockBufferQueue)
		defer shutdownGrpcServerOrDie(t, server)

		mockBufferQueue.EXPECT().RegisterSubscriber(gomock.Any()).Return(nil)
		mockBufferQueue.EXPECT().UnregisterSubscriber(gomock.Any()).Return(nil)
		ctx, cancel := context.WithCancel(defaultCtx)
		_, err := client.Subscribe(ctx, &SubscribeRequest{})
		if err != nil {
			t.Fatal("Subscribe failed:", err)
		}
		time.Sleep(time.Millisecond * 1)
		cancel()

		// Give the service a chance to exit the receive loop and unregister our subscriber.
		time.Sleep(time.Millisecond * 100)
	})
}
