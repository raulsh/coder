package notifications

import (
	"encoding/json"

	"github.com/coder/coder/v2/coderd/notifications/types"
	"golang.org/x/exp/maps"
	"golang.org/x/xerrors"
)

type MessagePayload struct {
	Version string `json:"_version"`

	NotificationName string `json:"notification_name,omitempty"`
	CreatedBy        string `json:"created_by,omitempty"`

	UserID    string `json:"user_id,omitempty"`
	UserEmail string `json:"user_email,omitempty"`
	UserName  string `json:"user_name,omitempty"`

	Labels types.Labels `json:"labels,omitempty"`
}

func (p *MessagePayload) ToLabels() (types.Labels, error) {
	labels := make(types.Labels, len(p.Labels))
	maps.Copy(labels, p.Labels)

	// Zero out labels so they're omitted since types.Labels is not multi-level.
	p.Labels = nil

	out, err := json.Marshal(p)
	if err != nil {
		return nil, xerrors.Errorf("marshal payload to json: %w", err)
	}

	var combined types.Labels
	err = json.Unmarshal(out, &combined)
	if err != nil {
		return nil, xerrors.Errorf("unmarshal payload from json: %w", err)
	}

	if labels == nil {
		return combined, nil
	}

	// Payload keys take precedence over supplied labels as these keys are reserved.
	labels.Merge(combined)
	return labels, nil
}
