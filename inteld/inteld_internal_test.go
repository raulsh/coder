package inteld

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/coder/coder/v2/inteld/proto"
)

func Test_invocationQueue(t *testing.T) {
	t.Parallel()
	t.Run("EnqueueWhileSending", func(t *testing.T) {
		// This test ensures that no invocations are missed while
		// a group of invocations are being sent.
		t.Parallel()
		ctx, cancelFunc := context.WithCancel(context.Background())
		queue := newInvocationQueue(time.Millisecond)
		sendStarted := make(chan []*proto.Invocation)
		sendCompleted := make(chan struct{})
		go queue.startSendLoop(ctx, func(i []*proto.Invocation) error {
			sendStarted <- i
			<-sendCompleted
			return nil
		})
		queue.enqueue(&proto.ReportInvocationRequest{
			ExitCode: 1,
		})
		inv := <-sendStarted
		require.Len(t, inv, 1)
		require.Equal(t, int32(1), inv[0].ExitCode)
		queue.enqueue(&proto.ReportInvocationRequest{
			ExitCode: 2,
		})
		sendCompleted <- struct{}{}
		inv = <-sendStarted
		require.Len(t, inv, 1)
		require.Equal(t, int32(2), inv[0].ExitCode)
		sendCompleted <- struct{}{}

		cancelFunc()
	})
}
