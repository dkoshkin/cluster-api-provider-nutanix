package client

import (
	"context"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/util/wait"

	nutanixtestclient "github.com/nutanix-cloud-native/cluster-api-provider-nutanix/test/helpers/prism-go-client/v3"
)

func Test_GetTaskStatus(t *testing.T) {
	client, err := nutanixtestclient.NewTestClient()
	assert.NoError(t, err)
	// use cleanup over defer as the connection gets closed before the tests run with t.Parallel()
	t.Cleanup(func() {
		client.Close()
	})

	t.Parallel()
	tests := []struct {
		name           string
		taskUUID       string
		handler        func(w http.ResponseWriter, r *http.Request)
		ctx            context.Context
		expectedStatus string
		expectedErr    error
	}{
		{
			name:     "succeeded",
			taskUUID: "succeeded",
			handler: func(w http.ResponseWriter, r *http.Request) {
				fmt.Fprint(w, `{"status": "SUCCEEDED"}`)
			},
			ctx:            context.Background(),
			expectedStatus: "SUCCEEDED",
		},
		{
			name:     "unauthorized",
			taskUUID: "unauthorized",
			handler: func(w http.ResponseWriter, r *http.Request) {
				http.Error(w, `{"error_code": "401"}`, http.StatusUnauthorized)
			},
			ctx:         context.Background(),
			expectedErr: fmt.Errorf("invalid Nutanix credentials"),
		},
		{
			name:     "invalid",
			taskUUID: "invalid",
			handler: func(w http.ResponseWriter, r *http.Request) {
				fmt.Fprint(w, `{"status": "INVALID_UUID", "error_detail": "invalid UUID", "progress_message": "invalid UUID"}`)
			},
			ctx:            context.Background(),
			expectedStatus: "INVALID_UUID",
			expectedErr:    fmt.Errorf("error_detail: invalid UUID, progress_message: invalid UUID"),
		},
		{
			name:     "failed",
			taskUUID: "failed",
			handler: func(w http.ResponseWriter, r *http.Request) {
				fmt.Fprint(w, `{"status": "FAILED", "error_detail": "task failed", "progress_message": "will never succeed"}`)
			},
			ctx:            context.Background(),
			expectedStatus: "FAILED",
			expectedErr:    fmt.Errorf("error_detail: task failed, progress_message: will never succeed"),
		},
	}
	for _, tt := range tests {
		tt := tt // Capture range variable.
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			client.AddHandler(nutanixtestclient.GetTaskURLPath(tt.taskUUID), tt.handler)

			status, err := GetTaskStatus(tt.ctx, client.Client, tt.taskUUID)
			assert.Equal(t, tt.expectedErr, err)
			assert.Equal(t, tt.expectedStatus, status)
		})
	}
}

func Test_WaitForTaskCompletion(t *testing.T) {
	client, err := nutanixtestclient.NewTestClient()
	assert.NoError(t, err)
	// use cleanup over defer as the connection gets closed before the tests run with t.Parallel()
	t.Cleanup(func() {
		client.Close()
	})

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*1)
	t.Cleanup(func() {
		cancel()
	})

	t.Parallel()
	tests := []struct {
		name        string
		taskUUID    string
		handler     func(w http.ResponseWriter, r *http.Request)
		ctx         context.Context
		expectedErr error
	}{
		{
			name:     "succeeded",
			taskUUID: "succeeded",
			handler: func(w http.ResponseWriter, r *http.Request) {
				fmt.Fprint(w, `{"status": "SUCCEEDED"}`)
			},
			ctx: ctx,
		},
		{
			name:     "invalid",
			taskUUID: "invalid",
			handler: func(w http.ResponseWriter, r *http.Request) {
				fmt.Fprint(w, `{"status": "INVALID_UUID", "error_detail": "invalid UUID", "progress_message": "invalid UUID"}`)
			},
			ctx:         ctx,
			expectedErr: fmt.Errorf("error_detail: invalid UUID, progress_message: invalid UUID"),
		},
		{
			name:     "timeout",
			taskUUID: "timeout",
			handler: func(w http.ResponseWriter, r *http.Request) {
				fmt.Fprint(w, `{"status": "PENDING"}`)
			},
			ctx:         ctx,
			expectedErr: wait.ErrWaitTimeout,
		},
	}
	for _, tt := range tests {
		tt := tt // Capture range variable.
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			client.AddHandler(nutanixtestclient.GetTaskURLPath(tt.taskUUID), tt.handler)

			err := WaitForTaskToSucceed(tt.ctx, client.Client, tt.taskUUID)
			if tt.expectedErr != nil {
				assert.ErrorContains(t, err, tt.expectedErr.Error())
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
