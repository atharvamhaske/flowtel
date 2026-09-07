package model_test

import (
	"errors"
	"testing"
	"time"

	"github.com/atharvamhaske/flowtel/pkg/model"
)

func TestEventValidate(t *testing.T) {
	tests := []struct {
		name      string
		event     model.Event
		wantError bool
	}{
		{
			name: "valid session",
			event: model.Event{
				ID: "session-1", Harness: "pi", Profile: model.ProfileBoth,
				Kind: model.KindSession, Name: "flowtel.session",
			},
		},
		{
			name: "missing identity",
			event: model.Event{
				Harness: "pi", Profile: model.ProfileBoth,
				Kind: model.KindSession, Name: "flowtel.session",
			},
			wantError: true,
		},
		{
			name: "end before start",
			event: model.Event{
				ID: "llm-1", Harness: "pi", Profile: model.ProfileBoth,
				Kind: model.KindLLM, Name: "llm", Start: time.Unix(2, 0), End: time.Unix(1, 0),
			},
			wantError: true,
		},
		{
			name: "permission requires decision",
			event: model.Event{
				ID: "permission-1", Harness: "pi", Profile: model.ProfileBoth,
				Kind: model.KindPermission, Name: "flowtel.permission",
			},
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := tt.event.Validate()
			if tt.wantError {
				if !errors.Is(err, model.ErrInvalidEvent) {
					t.Fatalf("Validate() error = %v, want %v", err, model.ErrInvalidEvent)
				}
				return
			}
			if err != nil {
				t.Fatalf("Validate() unexpected error = %v", err)
			}
		})
	}
}
